package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ChildResourceType interface {
	networkingv1.Ingress | v1.Service | v1.PersistentVolumeClaim | appsv1.StatefulSet
}

type ChildResource[T ChildResourceType] struct {
	Current *T
	Desired *T
}

func (c ChildResource[T]) Reconcile(ctx context.Context, clnt client.Client, cr *amaltheadevv1alpha1.AmaltheaSession) ChildResourceUpdate[T] {
	log := log.FromContext(ctx)
	if c.Current == nil {
		return ChildResourceUpdate[T]{}
	}
	switch current := any(c.Current).(type) {
	case *networkingv1.Ingress:
		res, err := controllerutil.CreateOrPatch(ctx, clnt, current, func() error {
			desired, ok := any(c.Desired).(*networkingv1.Ingress)
			if !ok {
				return fmt.Errorf("Could not cast when reconciling")
			}
			if current.CreationTimestamp.IsZero() {
				log.Info("Creating an ingress")
				current.Spec = desired.Spec
				current.ObjectMeta = desired.ObjectMeta
				err := ctrl.SetControllerReference(cr, current, clnt.Scheme())
				return err
			}
			current.Spec = desired.Spec
			return nil
		})
		return ChildResourceUpdate[T]{c.Current, res, err}
	case *appsv1.StatefulSet:
		res, err := controllerutil.CreateOrPatch(ctx, clnt, current, func() error {
			desired, ok := any(c.Desired).(*appsv1.StatefulSet)
			if !ok {
				return fmt.Errorf("Could not cast when reconciling")
			}
			if current.CreationTimestamp.IsZero() {
				log.Info("Creating a statefulset")
				current.Spec = desired.Spec
				current.ObjectMeta = desired.ObjectMeta
				err := ctrl.SetControllerReference(cr, current, clnt.Scheme())
				return err
			}
			current.Spec.Replicas = desired.Spec.Replicas
			current.Spec.Template.Spec.Containers = desired.Spec.Template.Spec.Containers
			current.Spec.Template.Spec.InitContainers = desired.Spec.Template.Spec.InitContainers
			current.Spec.Template.Spec.Volumes = desired.Spec.Template.Spec.Volumes
			return nil
		})
		return ChildResourceUpdate[T]{c.Current, res, err}
	case *v1.PersistentVolumeClaim:
		res, err := controllerutil.CreateOrPatch(ctx, clnt, current, func() error {
			desired, ok := any(c.Desired).(*v1.PersistentVolumeClaim)
			if !ok {
				return fmt.Errorf("Could not cast when reconciling")
			}
			if current.CreationTimestamp.IsZero() {
				log.Info("Creating a PVC")
				current.Spec = desired.Spec
				current.ObjectMeta = desired.ObjectMeta
				err := ctrl.SetControllerReference(cr, current, clnt.Scheme())
				return err
			}
			current.Spec.Resources.Requests = desired.Spec.Resources.Requests
			if desired.Spec.StorageClassName != nil {
				// NOTE: If the desired storage class is nil then the current spec contains the name for the default storage class
				current.Spec.StorageClassName = desired.Spec.StorageClassName
			}
			return nil
		})
		return ChildResourceUpdate[T]{c.Current, res, err}
	case *v1.Service:
		res, err := controllerutil.CreateOrPatch(ctx, clnt, current, func() error {
			desired, ok := any(c.Desired).(*v1.Service)
			if !ok {
				return fmt.Errorf("Could not cast when reconciling")
			}
			if current.CreationTimestamp.IsZero() {
				log.Info("Creating a service")
				current.Spec = desired.Spec
				current.ObjectMeta = desired.ObjectMeta
				err := ctrl.SetControllerReference(cr, current, clnt.Scheme())
				return err
			}
			current.Spec.Ports = desired.Spec.Ports
			current.Spec.Selector = desired.Spec.Selector
			return nil
		})
		return ChildResourceUpdate[T]{c.Current, res, err}
	default:
		return ChildResourceUpdate[T]{Error: fmt.Errorf("Encountered an uknown child resource type")}
	}
}

type ChildResourceUpdate[T ChildResourceType] struct {
	Manifest     *T
	UpdateResult controllerutil.OperationResult
	Error        error
}

type ChildResources struct {
	Ingress         ChildResource[networkingv1.Ingress]
	Service         ChildResource[v1.Service]
	StatefulSet     ChildResource[appsv1.StatefulSet]
	PVC             ChildResource[v1.PersistentVolumeClaim]
	DataSourcesPVCs []ChildResource[v1.PersistentVolumeClaim]
}

type ChildResourceUpdates struct {
	Ingress         ChildResourceUpdate[networkingv1.Ingress]
	Service         ChildResourceUpdate[v1.Service]
	StatefulSet     ChildResourceUpdate[appsv1.StatefulSet]
	PVC             ChildResourceUpdate[v1.PersistentVolumeClaim]
	DataSourcesPVCs []ChildResourceUpdate[v1.PersistentVolumeClaim]
}

func NewChildResources(cr *amaltheadevv1alpha1.AmaltheaSession) ChildResources {
	metadata := metav1.ObjectMeta{Name: cr.Name, Namespace: cr.Namespace}
	desiredService := cr.Service()
	desiredPVC := cr.PVC()
	desiredStatefulSet := cr.StatefulSet()
	desiredIngress := cr.Ingress()
	output := ChildResources{
		Service:     ChildResource[v1.Service]{&v1.Service{ObjectMeta: metadata}, &desiredService},
		PVC:         ChildResource[v1.PersistentVolumeClaim]{&v1.PersistentVolumeClaim{ObjectMeta: metadata}, &desiredPVC},
		StatefulSet: ChildResource[appsv1.StatefulSet]{&appsv1.StatefulSet{ObjectMeta: metadata}, &desiredStatefulSet},
	}

	if desiredIngress != nil {
		output.Ingress = ChildResource[networkingv1.Ingress]{&networkingv1.Ingress{ObjectMeta: metadata}, desiredIngress}
	}

	desiredDataSourcesPVCs := []ChildResource[v1.PersistentVolumeClaim]{}
	specPVCs, _, _ := cr.DataSources()
	for i := range specPVCs {
		desiredPVC := &specPVCs[i]
		childRes := ChildResource[v1.PersistentVolumeClaim]{
			Current: &v1.PersistentVolumeClaim{ObjectMeta: desiredPVC.ObjectMeta},
			Desired: desiredPVC,
		}
		desiredDataSourcesPVCs = append(desiredDataSourcesPVCs, childRes)
	}
	output.DataSourcesPVCs = desiredDataSourcesPVCs

	return output
}

func (c ChildResources) Reconcile(ctx context.Context, clnt client.Client, cr *amaltheadevv1alpha1.AmaltheaSession) (ChildResourceUpdates, error) {
	output := ChildResourceUpdates{
		StatefulSet: c.StatefulSet.Reconcile(ctx, clnt, cr),
		PVC:         c.PVC.Reconcile(ctx, clnt, cr),
		Service:     c.Service.Reconcile(ctx, clnt, cr),
		Ingress:     c.Ingress.Reconcile(ctx, clnt, cr),
	}

	dataSourceUpdates := []ChildResourceUpdate[v1.PersistentVolumeClaim]{}
	for _, pvc := range c.DataSourcesPVCs {
		dataSourceUpdates = append(dataSourceUpdates, pvc.Reconcile(ctx, clnt, cr))
	}
	output.DataSourcesPVCs = dataSourceUpdates
	return output, output.combineErrors()
}

func (c ChildResourceUpdates) AllEqual(op controllerutil.OperationResult) bool {
	ingressOK := c.Ingress.Manifest == nil || (c.Ingress.Manifest != nil && c.Ingress.UpdateResult == op)
	dataSourcesOK := true
	for _, ds := range c.DataSourcesPVCs {
		dataSourcesOK = dataSourcesOK && (ds.UpdateResult == op)
	}
	return ingressOK && c.Service.UpdateResult == op && c.PVC.UpdateResult == op && c.StatefulSet.UpdateResult == op && dataSourcesOK
}

func (c ChildResourceUpdates) IsRunning(pod *v1.Pod) bool {
	// TODO: Try to re-enable the two checks below and potentially use them to determine readiness.
	// Currently the resources created by the operator have slight changes that k8s itself applies in a few places outside
	// of the status field. So these are picked up by the functions below. For example a PVC or a statefulset gets automatic
	// updates from k8s (I think from a mutating or defaulting webhook) to fields other than the status.
	// onlyStatusUpdates := c.AllEqual(controllerutil.OperationResultUpdatedStatusOnly)
	// noUpdates := c.AllEqual(controllerutil.OperationResultNone)
	stsReady := c.StatefulSet.Manifest.Status.ReadyReplicas == 1 && c.StatefulSet.Manifest.Status.Replicas == 1
	podExists := pod != nil
	podReady := podExists && podIsReady(pod)
	return stsReady && podReady
}

func (c ChildResourceUpdates) State(cr *amaltheadevv1alpha1.AmaltheaSession, pod *v1.Pod) amaltheadevv1alpha1.State {
	switch {
	case cr.GetDeletionTimestamp() != nil:
		return amaltheadevv1alpha1.NotReady
	case cr.Spec.Hibernated && c.StatefulSet.Manifest.Spec.Replicas != nil && *c.StatefulSet.Manifest.Spec.Replicas == 0:
		return amaltheadevv1alpha1.Hibernated
	case podIsFailed(pod):
		return amaltheadevv1alpha1.Failed
	case c.IsRunning(pod):
		return amaltheadevv1alpha1.Running
	default:
		return amaltheadevv1alpha1.NotReady
	}
}

func (c ChildResourceUpdates) Status(ctx context.Context, r *AmaltheaSessionReconciler, cr *amaltheadevv1alpha1.AmaltheaSession) amaltheadevv1alpha1.AmaltheaSessionStatus {
	log := log.FromContext(ctx)

	idle := isIdle(ctx, r.MetricsClient, cr)
	idleSince := cr.Status.IdleSince
	if idle && idleSince.IsZero() {
		idleSince = metav1.NewTime(time.Now())
	}
	if !idle && !idleSince.IsZero() {
		idleSince = metav1.Time{}
	}

	hibernated := cr.Spec.Hibernated
	hibernatedSince := cr.Status.HibernatedSince
	if hibernated && hibernatedSince.IsZero() {
		hibernatedSince = metav1.NewTime(time.Now())
	}
	if !hibernated && !hibernatedSince.IsZero() {
		hibernatedSince = metav1.Time{}
	}

	pod, err := cr.Pod(ctx, r.Client)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "Could not read the session pod when updating the status")
	}

	failing := pod != nil && podIsFailed(pod)
	failingSince := cr.Status.FailingSince
	if failing && failingSince.IsZero() {
		failingSince = metav1.NewTime(time.Now())
	}
	if !hibernated && !failingSince.IsZero() {
		failingSince = metav1.Time{}
	}

	state := c.State(cr, pod)
	conditions := cr.Status.Conditions
	if len(conditions) == 0 {
		conditions = amaltheadevv1alpha1.NewConditions()
	}

	for i, condition := range conditions {
		now := metav1.Now()
		switch condition.Type {
		case amaltheadevv1alpha1.AmaltheaSessionReady:
			stateIsRunning := state == amaltheadevv1alpha1.Running
			if stateIsRunning && condition.Status == metav1.ConditionFalse {
				condition.Status = metav1.ConditionTrue
				condition.LastTransitionTime = now
				condition.Reason = string(state)
				condition.Message = fmt.Sprint("The session is ", strings.ToLower(string(state)))
			} else if !stateIsRunning && condition.Status == metav1.ConditionTrue {
				condition.Status = metav1.ConditionFalse
				condition.LastTransitionTime = now
				condition.Reason = string(state)
				condition.Message = fmt.Sprint("The session is ", strings.ToLower(string(state)))
			}
		case amaltheadevv1alpha1.AmaltheaSessionRoutingReady:
			ingressExists := func() bool {
				namespacedName := types.NamespacedName{Name: cr.Name, Namespace: cr.GetNamespace()}
				err := r.Client.Get(ctx, namespacedName, &networkingv1.Ingress{})
				return err == nil
			}
			if cr.Spec.Ingress == nil && condition.Status == metav1.ConditionTrue {
				condition.Status = metav1.ConditionFalse
				condition.LastTransitionTime = now
				condition.Reason = "IngressDeleted"
				condition.Message = fmt.Sprint("The ingress information was not specified or was deleted from custom resource ",
					cr.Name)
			} else if cr.Spec.Ingress != nil && condition.Status == metav1.ConditionFalse && ingressExists() {
				condition.Status = metav1.ConditionTrue
				condition.LastTransitionTime = now
				condition.Reason = "IngressOperational"
				condition.Message = fmt.Sprint("The ingress is setup and operational")
			}
		}
		conditions[i] = condition
	}

	status := amaltheadevv1alpha1.AmaltheaSessionStatus{
		Conditions:      conditions,
		State:           state,
		URL:             cr.GetURLString(),
		Idle:            idle,
		IdleSince:       idleSince,
		FailingSince:    failingSince,
		HibernatedSince: hibernatedSince,
	}

	if pod != nil {
		initCounts, counts := containerCounts(pod)
		status.InitContainerCounts = initCounts
		status.ContainerCounts = counts
	}
	if state == amaltheadevv1alpha1.Hibernated || cr.DeletionTimestamp != nil {
		status.ContainerCounts.Ready = 0
		status.InitContainerCounts.Ready = 0
	}

	// Used for debugging to ensure the reconcile loop does not needlessly reschdule or update child resources
	// log.Info("Update summary", "Ingress", c.Ingress.UpdateResult, "StatefulSet", c.StatefulSet.UpdateResult, "PVC", c.StatefulSet.UpdateResult, "Service", c.Service.UpdateResult)

	return status
}

func (c ChildResourceUpdates) combineErrors() error {
	errorMsgs := []string{}
	errors := map[string]error{
		"Ingress":     c.Ingress.Error,
		"Service":     c.Service.Error,
		"PVC":         c.PVC.Error,
		"StatefulSet": c.StatefulSet.Error,
	}
	for _, pvc := range c.DataSourcesPVCs {
		if pvc.Error == nil {
			continue
		}
		errors["DataSourcesPVCs/"+pvc.Manifest.Name] = pvc.Error
	}
	for name, err := range errors {
		if err == nil {
			continue
		}
		errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", name, err.Error()))
	}
	if len(errorMsgs) == 0 {
		return nil
	}
	return fmt.Errorf("error in reconciling children %s", strings.Join(errorMsgs, ", "))
}
