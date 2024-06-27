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
	"fmt"

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

	// Handle StatefulSet

	found_sts := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: amaltheasession.Name, Namespace: amaltheasession.Namespace}, found_sts)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new StatefulSet
		sts, err := r.statefulSetForAmaltheaSession(amaltheasession)
		if err != nil {
			log.Error(err, "Failed to define new StatefulSet resource for AmaltheaSession")

			// The following implementation will update the status
			meta.SetStatusCondition(&amaltheasession.Status.Conditions, metav1.Condition{Type: typeAvailableAmaltheaSession,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create StatefulSet for the custom resource (%s): (%s)", amaltheasession.Name, err)})

			if err := r.Status().Update(ctx, amaltheasession); err != nil {
				log.Error(err, "Failed to update AmaltheaSession status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

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

	return ctrl.Result{}, nil
}

// statefulSetForAmaltheaSession returns a AmaltheaSession StatefulSet object
func (r *AmaltheaSessionReconciler) statefulSetForAmaltheaSession(
	amaltheasession *amaltheadevv1alpha1.AmaltheaSession) (*appsv1.StatefulSet, error) {
	labels := labelsForAmaltheaSession(amaltheasession.Name)
	replicas := int32(1)

	session := amaltheasession.Spec.Session

	sessionContainer := corev1.Container{
		Image:           session.Image,
		Name:            "session",
		ImagePullPolicy: corev1.PullIfNotPresent,

		Ports: []corev1.ContainerPort{{
			ContainerPort: session.Port,
			Name:          "session-port",
		}},

		Args:      session.Args,
		Command:   session.Command,
		Env:       session.Env,
		Resources: session.Resources,
	}

	securityContext := &corev1.SecurityContext{
		RunAsNonRoot: &[]bool{true}[0],
		RunAsUser:    &[]int64{session.RunAsUser}[0],
		RunAsGroup:   &[]int64{session.RunAsGroup}[0],
	}

	if session.RunAsUser == 0 {
		securityContext.RunAsNonRoot = &[]bool{false}[0]
	}

	sessionContainer.SecurityContext = securityContext

	containers := []corev1.Container{sessionContainer}
	containers = append(containers, amaltheasession.Spec.ExtraContainers...)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      amaltheasession.Name,
			Namespace: amaltheasession.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers:     containers,
					InitContainers: amaltheasession.Spec.ExtraInitContainers,
				},
			},
		},
	}

	// Set the ownerRef for the StatefulSet
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(amaltheasession, sts, r.Scheme); err != nil {
		return nil, err
	}
	return sts, nil
}

// labelsForAmaltheaSessino returns the labels for selecting the resources
// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
func labelsForAmaltheaSession(name string) map[string]string {
	return map[string]string{"app.kubernetes.io/name": "AmaltheaSession",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/part-of":    "amaltheasession-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AmaltheaSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&amaltheadevv1alpha1.AmaltheaSession{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}
