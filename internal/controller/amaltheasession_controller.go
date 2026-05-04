/*
Copyright 2026.

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
	"errors"
	"os"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	childutils "github.com/SwissDataScienceCenter/amalthea/internal/controller/children"
)

// AmaltheaSessionReconciler reconciles a AmaltheaSession object
type AmaltheaSessionReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	MetricsClient metricsv1beta1.PodMetricsesGetter
	ClusterType   amaltheadevv1alpha1.ClusterType
}

// finalizers
const secretCleanupFinalizerName = "amalthea.dev/secrets-finalizer"
const rclonePVFinalizer = "amalthea.dev/rclone-pv-finalizer"

// +kubebuilder:rbac:groups=amalthea.dev,resources=amaltheasessions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=amalthea.dev,resources=amaltheasessions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=amalthea.dev,resources=amaltheasessions/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=list;watch;delete;create;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=metrics.k8s.io,resources=pods,verbs=get;list;watch

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

	if amaltheasession.GetDeletionTimestamp() == nil {
		if reflect.DeepEqual(amaltheasession.Status, amaltheadevv1alpha1.AmaltheaSessionStatus{State: amaltheadevv1alpha1.NotReady, Idle: false}) {
			// First status update/render
			amaltheasession.Status.URL = amaltheasession.GetURLString()
			err := r.Status().Update(ctx, amaltheasession)
			if err != nil {
				err = r.Get(ctx, req.NamespacedName, amaltheasession)
				if err != nil {
					return ctrl.Result{}, err
				}
				err = r.Status().Update(ctx, amaltheasession)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
			// Record initial metrics
			RecordAmaltheaSessionMetrics(amaltheasession)
		}
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if len(amaltheasession.AdoptedSecrets().Items) > 0 && !controllerutil.ContainsFinalizer(amaltheasession, secretCleanupFinalizerName) {
			controllerutil.AddFinalizer(amaltheasession, secretCleanupFinalizerName)
			if err := r.Update(ctx, amaltheasession); err != nil {
				return ctrl.Result{}, err
			}
		}

		if _, finalizerNeeded := amaltheasession.RcloneV2DataSource(); finalizerNeeded && !controllerutil.ContainsFinalizer(amaltheasession, rclonePVFinalizer) {
			controllerutil.AddFinalizer(amaltheasession, rclonePVFinalizer)
			if err := r.Update(ctx, amaltheasession); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
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

		// The object is being deleted
		if controllerutil.ContainsFinalizer(amaltheasession, secretCleanupFinalizerName) {
			if err := r.deleteSecrets(ctx, amaltheasession); err != nil {
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(amaltheasession, secretCleanupFinalizerName)
			if err := r.Update(ctx, amaltheasession); err != nil {
				return ctrl.Result{}, err
			}
		}
		if controllerutil.ContainsFinalizer(amaltheasession, rclonePVFinalizer) {
			// We don't need the whole PV to delete, just the name
			pvStub := corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: childutils.RcloneV2ResourceName(amaltheasession)}}
			err := r.Client.Delete(ctx, &pvStub)
			if err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(amaltheasession, rclonePVFinalizer)
			if err := r.Update(ctx, amaltheasession); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{Requeue: true}, nil
	}

	children, err := NewChildResources(amaltheasession, r.ClusterType)
	if err != nil {
		log.Error(
			err,
			"There was an error in generating the Kubernetes resources based on AmaltheaSession specification. Please report this with the Renku developers.",
			"name",
			amaltheasession.GetName(),
			"namespace",
			amaltheasession.GetNamespace(),
		)
		return ctrl.Result{}, err
	}

	rcloneV2Res := childutils.NewRcloneV2Resources(amaltheasession)
	err = rcloneV2Res.Reconcile(ctx, r.Client)
	if err != nil {
		log.Error(err, "Failed when reconciling rclone data sources")
		return ctrl.Result{}, err
	}
	updates, err := children.Reconcile(ctx, r.Client, amaltheasession)
	if err != nil {
		log.Error(err, "Failed when reconciling children")
		return ctrl.Result{}, err
	}

	newStatus := updates.Status(ctx, r, amaltheasession)
	statusChanged := reflect.DeepEqual(amaltheasession.Status, newStatus)
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

	// Record metrics for the session status
	RecordAmaltheaSessionMetrics(amaltheasession)

	if amaltheasession.NeedsDeletion() {
		// Clean up metrics for this session before deleting it
		RemoveAmaltheaSessionMetrics(amaltheasession)
		err = r.Delete(ctx, amaltheasession)
		log.Info("custom resource deleted")
		return ctrl.Result{}, err
	}

	err = updateHibernationState(ctx, r, amaltheasession)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Now requeue to make sure we can watch for idleness and other status changes
	requeueAfter := time.Second * 10
	if statusChanged {
		// If the status is evolving we should requeue faster
		requeueAfter = 0
	}
	return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
}

func (r *AmaltheaSessionReconciler) deleteSecrets(ctx context.Context, cr *amaltheadevv1alpha1.AmaltheaSession) error {
	adoptedSecrets := cr.AdoptedSecrets()
	if len(adoptedSecrets.Items) == 0 {
		log := log.FromContext(ctx)
		log.Info("Secret deletion finalizer called without any secret adopted, doing nothing")
		return nil
	}

	// create an initial empty error list
	error_list := errors.Join(nil, nil)
	for _, item := range adoptedSecrets.Items {
		err := r.Delete(ctx, &item)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				error_list = errors.Join(error_list, err)
			}
		}
	}
	return error_list
}

func operatorNamespace() string {
	return os.Getenv("OPERATOR_NAMESPACE")
}

func allowedToActOnPV(labels map[string]string) bool {
	clusterScoped := len(operatorNamespace()) == 0
	ns, nsFound := labels[childutils.RclonePVLabelSessionNamespaceKey]
	_, nameFound := labels[childutils.RclonePVLabelSessionNameKey]
	if clusterScoped {
		return nsFound && nameFound
	} else {
		return nsFound && nameFound && operatorNamespace() == ns
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AmaltheaSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&amaltheadevv1alpha1.AmaltheaSession{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Secret{}).
		Watches(
			// Since PVs are cluster scoped we filter to only watch PVs with the specific labels.
			// The labels indicate which name and namespace the session belongs to.
			// We only act on PVs where the namespace label matches the namespace the operator is deployed for.
			// If the OPERATOR_NAMESPACE env var is not set we assume the operator is cluster scoped and we act on all PVs
			// that have the name and namespace labels regardless of what namespace the operator is in.
			&corev1.PersistentVolume{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				labels := obj.GetLabels()
				if !allowedToActOnPV(labels) {
					return nil
				}
				ns := labels[childutils.RclonePVLabelSessionNamespaceKey]
				name := labels[childutils.RclonePVLabelSessionNameKey]
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}},
				}
			}),
			builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				labels := obj.GetLabels()
				return allowedToActOnPV(labels)
			})),
		).
		Complete(r)
}
