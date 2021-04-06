package controllers

import (
	"context"
	"log"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1 "github.com/cownetwork/instance-controller/api/v1"
)

// InstanceReconciler reconciles a Instance object
type InstanceReconciler struct {
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
	Decider Decider
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

	action, err := r.Decider.Decide(ctx, instance, req)
	if err != nil {
		return ctrl.Result{}, err // Maybe we need to requeue if no decision could be made due tue an error
	}

	switch action {
	case ActionInit:
		log.Info("initializing instance")
		if err := r.initInstance(ctx, &instance); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("created Instance successfully", "instance_id", instance.Annotations["instance.cow.network/id"])
		break
	case ActionCleanup:
		logger := log.WithValues("instance_id", instance.Annotations["instance.cow.network/id"], "instance_name", instance.Name, "namespace", instance.Namespace)
		if err := r.cleanupInstance(ctx, instance); err != nil {
			logger.Error(err, "could not cleanup Instance")
			return ctrl.Result{}, err
		}
		logger.Info("cleaned up Instance successfully")
		break
	case ActionUpdate:
		logger := log.WithValues("instance_id", instance.Annotations["instance.cow.network/id"], "instance_name", instance.Name, "namespace", instance.Namespace)
		if err := r.updateInstance(ctx, &instance); err != nil {
			logger.Error(err, "could not update Instance")
			return ctrl.Result{}, err
		}
		logger.Info("updated Instance successfully")
		break
	case ActionIgnore:
		break
	}

	return ctrl.Result{}, nil
}

func (r *InstanceReconciler) initInstance(ctx context.Context, instance *instancev1.Instance) error {
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	instance.Annotations["instance.cow.network/id"] = id.String()
	instance.Status.State = instancev1.StateInitializing

	if err := r.Update(ctx, instance); err != nil {
		return err
	}

	pod, err := r.createPod(instance)
	if err != nil {
		return err
	}

	if err := r.Create(ctx, pod); err != nil {
		return err
	}

	return nil
}

func (r *InstanceReconciler) cleanupInstance(ctx context.Context, instance instancev1.Instance) error {
	if err := r.Delete(ctx, &instance); err != nil {
		return err
	}
	return nil
}

func (r *InstanceReconciler) updateInstance(ctx context.Context, instance *instancev1.Instance) error {
	var pod corev1.Pod
	err := r.Get(ctx, client.ObjectKey{Name: instance.Annotations["instance.cow.network/id"], Namespace: instance.Namespace}, &pod)
	if err != nil {
		return err
	}
	instance.Status.IP = pod.Status.PodIP
	if err := r.Update(ctx, instance); err != nil {
		return err
	}
	return nil
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
