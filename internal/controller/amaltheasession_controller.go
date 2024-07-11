/*
Copyright 2024.

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

package controller

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
)

// AmaltheaSessionReconciler reconciles a AmaltheaSession object
type AmaltheaSessionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Definitions to manage status conditions
const (
	// typeAvailableAmaltheaSession represents the status of the StatefulSet reconciliation
	typeAvailableAmaltheaSession = "Available"
	// typeDegradedAmaltheaSession represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedAmaltheaSession = "Degraded"
)

//+kubebuilder:rbac:groups=amalthea.dev,resources=amaltheasessions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=amalthea.dev,resources=amaltheasessions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=amalthea.dev,resources=amaltheasessions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AmaltheaSession object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *AmaltheaSessionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	amaltheasession := &amaltheadevv1alpha1.AmaltheaSession{}
	err := r.Get(ctx, req.NamespacedName, amaltheasession)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("amaltheasession resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get amaltheasession")
		return ctrl.Result{}, err
	}

	if amaltheasession.GetDeletionTimestamp() != nil {
		amaltheasession.Status.State = amaltheadevv1alpha1.NotReady
		err := r.Status().Update(ctx, amaltheasession)
		if err != nil {
			// The status update can fail if the CR is out of date, re-read the CR here and retry
			err = r.Get(ctx, req.NamespacedName, amaltheasession)
			if err != nil {
				return ctrl.Result{}, err
			}
			amaltheasession.Status.State = amaltheadevv1alpha1.NotReady
			err = r.Status().Update(ctx, amaltheasession)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("spec", "cr", amaltheasession)

	children := NewChildResources(amaltheasession)
	updates, err := children.Reconcile(ctx, r.Client, amaltheasession)
	if err != nil {
		log.Error(err, "Failed when reconciling children")
		return ctrl.Result{}, err
	}

	newStatus := updates.Status(ctx, r.Client, amaltheasession)
	amaltheasession.Status = newStatus
	err = r.Status().Update(ctx, amaltheasession)
	if err != nil {
		// The status update can fail if the CR is out of date, re-read the CR here and retry
		err = r.Get(ctx, req.NamespacedName, amaltheasession)
		if err != nil {
			return ctrl.Result{}, err
		}
		amaltheasession.Status = newStatus
		err = r.Status().Update(ctx, amaltheasession)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	err = updateStatefulSetReplicas(ctx, r, amaltheasession)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Now requeue to make sure we can watch for idleness and other status changes
	return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AmaltheaSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&amaltheadevv1alpha1.AmaltheaSession{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
