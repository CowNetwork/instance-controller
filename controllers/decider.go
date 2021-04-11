package controllers

import (
	"context"

	instancev1 "github.com/cownetwork/instance-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Action int

const (
	// actionInit tells the controller to initialize the created Instance
	ActionInit Action = iota

	// actionUpdate tells the controller to update the Instance status
	ActionUpdate

	// actionCleanup tells the controller to remove the Instance
	// and all other resources owned by it
	ActionCleanup

	// actionIgnore tells the controller to do nothing and ignore the request
	ActionIgnore
)

type Decider struct {
	Client client.Client
}

// decide decides based on the instance annotations, status and owned resource(s) what the controller needs to to do.
func (d *Decider) Decide(ctx context.Context, instance instancev1.Instance, req ctrl.Request) (Action, error) {
	idstr := instance.Status.ID
	pod := &corev1.Pod{}

	err := d.Client.Get(ctx, client.ObjectKey{Name: idstr, Namespace: instance.Namespace}, pod)
	if err != nil && !apierrors.IsNotFound(err) {
		return -1, err
	}

	// If no state is set an no pod could be found
	// the instance is completely new and a pod needs
	// to be created
	if len(string(instance.Status.State)) == 0 {
		return ActionInit, nil
	}

	// If we id not find a pod, but the State field is set
	// this means the pod corresponding to the instance has been deleted.
	// This means we can cleanup this Instance.
	if err != nil && apierrors.IsNotFound(err) {
		return ActionCleanup, nil
	}

	// If the instance is still Initializing but we have a pod
	// update the instance definition because we need to know
	// the pods IP for example
	if instance.Status.State == instancev1.StateInitializing {
		return ActionUpdate, nil
	}

	return ActionIgnore, nil
}
