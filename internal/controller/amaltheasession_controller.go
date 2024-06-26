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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

	if amaltheasession.Status.Conditions == nil || len(amaltheasession.Status.Conditions) == 0 {
		meta.SetStatusCondition(&amaltheasession.Status.Conditions, metav1.Condition{Type: typeAvailableAmaltheaSession, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, amaltheasession); err != nil {
			log.Error(err, "Failed to update amaltheasession status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the amaltheasession Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, amaltheasession); err != nil {
			log.Error(err, "Failed to re-fetch amaltheasession")
			return ctrl.Result{}, err
		}
	}

	// Check if the AmaltheaSession instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isAmaltheaSessionMarkedToBeDeleted := amaltheasession.GetDeletionTimestamp() != nil
	if isAmaltheaSessionMarkedToBeDeleted {
		// Add finalizer handling if needed
		return ctrl.Result{}, nil
	}

	children := amaltheasession.Children()

	// Handle StatefulSet

	found_sts := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: amaltheasession.Name, Namespace: amaltheasession.Namespace}, found_sts)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new StatefulSet
		sts := &children.StatefulSet
		log.Info("Creating a new StatefulSet",
			"StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
		if err = r.Create(ctx, sts); err != nil {
			log.Error(err, "Failed to create new StatefulSet",
				"StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
			return ctrl.Result{}, err
		}
		// StatefulSet created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		// return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get StatefulSet")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// Handle Service

	svc_found := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: amaltheasession.Name, Namespace: amaltheasession.Namespace}, svc_found)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new service
		svc := &children.Service
		log.Info("Creating a new Service",
			"Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		if err = r.Create(ctx, svc); err != nil {
			log.Error(err, "Failed to create new Service",
				"Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// Service created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// Handle Ingress

	if children.Ingress == nil {
		log.Info("No ingress specified")
		return ctrl.Result{}, nil
	}

	ing_found := &networkingv1.Ingress{}
	err = r.Get(ctx, types.NamespacedName{Name: amaltheasession.Name, Namespace: amaltheasession.Namespace}, ing_found)
	if err != nil && apierrors.IsNotFound(err) {
		ing := children.Ingress
		log.Info("Creating a new Ingress",
			"Ingress.Namespace", ing.Namespace, "Ingress.Name", ing.Name)
		if err = r.Create(ctx, ing); err != nil {
			log.Error(err, "Failed to create new Ingress",
				"Ingress.Namespace", ing.Namespace, "Ingress.Name", ing.Name)
			return ctrl.Result{}, err
		}
		// Ingress created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Ingress")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AmaltheaSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&amaltheadevv1alpha1.AmaltheaSession{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}
