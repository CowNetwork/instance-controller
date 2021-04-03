package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1 "github.com/cownetwork/instance-controller/api/v1"
)

// InstanceReconciler reconciles a Instance object
type InstanceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=instance.cow.network,resources=instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=instance.cow.network,resources=instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=instances/status,verbs=get
func (r *InstanceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("instance_name", req.Name, "namespace", req.Namespace)

	var instance instancev1.Instance
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		log.Error(err, "could not find instance")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	idstr := instance.Annotations["instance.cow.network/id"]
	var pod *corev1.Pod

	// if pod is not found this and no id is set this indicates we have a completely new instance
	err := r.Get(ctx, client.ObjectKey{Name: idstr, Namespace: instance.Namespace}, pod)
	if err != nil && apierrors.IsNotFound(err) && idstr != "" {
		log.Info("creating new Pod for Instance")

		id, err := uuid.NewUUID()
		if err != nil {
			log.Error(err, "could not generate instance id")
			return ctrl.Result{}, err
		}

		instance.Annotations["instance.cow.network/id"] = id.String()

		pod, err := r.createPod(&instance)
		if err != nil {
			log.Error(err, "could not create Pod object")
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, pod); err != nil {
			log.Error(err, "could not create Pod for Instance", "instance_id", id.String())
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	log.Info("deleting Instance because underlying Pod could not be found", "instance_id", idstr)
	if err := r.Delete(ctx, &instance); err != nil {
		log.Error(err, "could not delete Instance", "instance_id", idstr)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *InstanceReconciler) createPod(instance *instancev1.Instance) (*corev1.Pod, error) {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        instance.Annotations["instance.cow.network/id"],
			Namespace:   instance.Namespace,
		},
		Spec: *instance.Spec.Template.DeepCopy(),
	}

	for k, v := range instance.Annotations {
		p.Annotations[k] = v
	}

	for k, v := range instance.Labels {
		p.Labels[k] = v
	}

	if err := ctrl.SetControllerReference(instance, p, r.Scheme); err != nil {
		return nil, err
	}

	return p, nil
}

func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1.Instance{}).
		Owns(&corev1.Pod).
		Complete(r)
}
