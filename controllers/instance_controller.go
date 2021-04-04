package controllers

import (
	"context"
	"log"

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
		// ignore not found because we only handle creation of the Instances and the deletion
		// of the underlying pod
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	idstr := instance.Annotations["instance.cow.network/id"]
	pod := &corev1.Pod{}

	err := r.Get(ctx, client.ObjectKey{Name: idstr, Namespace: instance.Namespace}, pod)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// if no state is set and no pod could be found we have a completely new instance
	if len(string(instance.Status.State)) == 0 {
		log.Info("creating new Pod for Instance")

		id, err := uuid.NewRandom()
		if err != nil {
			log.Error(err, "could not generate instance id")
			return ctrl.Result{}, err
		}

		instance.Annotations["instance.cow.network/id"] = id.String()
		instance.Status.State = instancev1.InitializingState

		if err := r.Update(ctx, &instance); err != nil {
			log.Error(err, "could not update Instance")
			return ctrl.Result{}, err
		}

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

	// if we did not find a pod, but the id is set this means
	// we can delete the corresponding Instance.
	if err != nil && apierrors.IsNotFound(err) {
		log.Info("deleting Instance because underlying Pod could not be found", "instance_id", idstr)
		if err := r.Delete(ctx, &instance); err != nil {
			log.Error(err, "could not delete Instance", "instance_id", idstr)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *InstanceReconciler) createPod(instance *instancev1.Instance) (*corev1.Pod, error) {
	id := instance.Annotations["instance.cow.network/id"]
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        id,
			Namespace:   instance.Namespace,
		},
		Spec: *instance.Spec.Template.DeepCopy(),
	}

	for i := range p.Spec.Containers {
		p.Spec.Containers[i].Env = append(p.Spec.Containers[i].Env, corev1.EnvVar{Name: "INSTANCE_ID", Value: id})
	}

	log.Println(p.Spec.Containers[0].Env)

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
	if err := mgr.GetFieldIndexer().IndexField(&corev1.Pod{}, ".metadata.controller", func(o runtime.Object) []string {
		pod := o.(*corev1.Pod)
		owner := metav1.GetControllerOf(pod)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != instancev1.GroupVersion.String() || owner.Kind != "Instance" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1.Instance{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
