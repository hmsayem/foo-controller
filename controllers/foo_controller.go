/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"github.com/go-logr/logr"
	batchv1 "github.com/hmsayem/foo-kubebuilder/api/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// FooReconciler reconciles a Foo object
type FooReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=batch.hmsayem.kubebuilder.io,resources=foos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.hmsayem.kubebuilder.io,resources=foos/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch.hmsayem.kubebuilder.io,resources=foos/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

func (r *FooReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log := r.Log.WithValues("foo", req.NamespacedName)
	log.Info("fetching foo resource")
	foo := batchv1.Foo{}
	if err := r.Client.Get(ctx, req.NamespacedName, &foo); err != nil {
		log.Error(err, "failed to get foo resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// finalizer
	finalizerName := "batch.hmsayem.kubebuilder.io/finalizer"
	if foo.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("IsZero")
		if !containsString(foo.GetFinalizers(), finalizerName) {
			log.Info("Registering finalizer")
			controllerutil.AddFinalizer(&foo, finalizerName)
			if err := r.Update(ctx, &foo); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		log.Info("IsNotZero")
		if containsString(foo.GetFinalizers(), finalizerName) {
			log.Info("Removing finalizer")
			if err := r.deleteExternalResources(&foo); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&foo, finalizerName)
			if err := r.Update(ctx, &foo); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	log = log.WithValues("deploymentName", foo.Spec.DeploymentName)
	log.Info("checking if a deployment exists for foo resource")
	deployment := apps.Deployment{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: foo.Namespace, Name: foo.Spec.DeploymentName}, &deployment)
	if apierrors.IsNotFound(err) {
		log.Info("could not find existing deployment for foo, creating one...")
		deployment = *buildDeployment(foo)
		if err := r.Client.Create(ctx, &deployment); err != nil {
			log.Error(err, "failed to create Deployment resource")
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(&foo, core.EventTypeNormal, "Created", "Created deployment &q", deployment.Name)
		log.Info("Created Deployment resource for foo")
		log.Info("updating foo resource status")
		if err := r.updateFooStatus(&foo); err != nil {
			log.Error(err, "failed to update foo status")
			return ctrl.Result{}, err
		}
		log.Info("resource status synced")
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, "failed to get deployment for foo resource")
		return ctrl.Result{}, err
	}
	log.Info("deployment resource already exists for foo, checking replica count")
	expectedReplicas := int32(1)
	if *deployment.Spec.Replicas != expectedReplicas {
		log.Info("updating replica count", "old_count", *deployment.Spec.Replicas, "new_count", expectedReplicas)
		deployment.Spec.Replicas = &expectedReplicas
		if err := r.Client.Update(ctx, &deployment); err != nil {
			log.Error(err, "failed to update deployment replica count")
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(&foo, core.EventTypeNormal, "Scaled", "Scaled deployment %q to %d replicas", deployment.Name, expectedReplicas)
		return ctrl.Result{}, nil
	}
	log.Info("replica count is up to date", "replica_count", *deployment.Spec.Replicas)
	return ctrl.Result{}, nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (r *FooReconciler) updateFooStatus(foo *batchv1.Foo) error {
	foo.Status.AvailableReplicas = 1
	if err := r.Status().Update(context.Background(), foo); err != nil {
		return err
	}
	return nil
}

func (r *FooReconciler) deleteExternalResources(foo *batchv1.Foo) error {

	// delete any external resources associated with the cronJob
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.
	return nil
}

func buildDeployment(foo batchv1.Foo) *apps.Deployment {
	deployment := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            foo.Spec.DeploymentName,
			Namespace:       foo.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&foo, batchv1.GroupVersion.WithKind("Foo"))},
		},
		Spec: apps.DeploymentSpec{
			Replicas: foo.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"hmsayem/deployment-name": foo.Spec.DeploymentName,
				},
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"hmsayem/deployment-name": foo.Spec.DeploymentName,
					},
				},
				Spec: core.PodSpec{
					Containers: []core.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			},
		},
	}
	return &deployment
}

var (
	deploymentOwnerKey = ".metadata.controller"
)

// SetupWithManager sets up the controller with the Manager.
func (r *FooReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apps.Deployment{}, deploymentOwnerKey, func(rawobj client.Object) []string {
		depl := rawobj.(*apps.Deployment)
		owner := metav1.GetControllerOf(depl)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != batchv1.GroupVersion.String() || owner.Kind != "Foo" {
			return nil
		}
		return []string{owner.Name}

	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Foo{}).
		Complete(r)
}
