package controller

import (
	"context"
	"fmt"
	"slices"
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
	networkingv1.Ingress | v1.Service | v1.PersistentVolumeClaim | appsv1.StatefulSet | v1.Secret
}

type ChildResource[T ChildResourceType] struct {
	Current *T
	Desired *T
}

type ChildResourceUpdate[T ChildResourceType] struct {
	Manifest       *T
	UpdateResult   controllerutil.OperationResult
	Error          error
	statusCallback func(*amaltheadevv1alpha1.AmaltheaSessionStatus)
}

type ChildResources struct {
	Ingress         ChildResource[networkingv1.Ingress]
	Service         ChildResource[v1.Service]
	StatefulSet     ChildResource[appsv1.StatefulSet]
	PVC             ChildResource[v1.PersistentVolumeClaim]
	DataSourcesPVCs []ChildResource[v1.PersistentVolumeClaim]
	Secret          ChildResource[v1.Secret]
}

type ChildResourceUpdates struct {
	Ingress         ChildResourceUpdate[networkingv1.Ingress]
	Service         ChildResourceUpdate[v1.Service]
	StatefulSet     ChildResourceUpdate[appsv1.StatefulSet]
	PVC             ChildResourceUpdate[v1.PersistentVolumeClaim]
	DataSourcesPVCs []ChildResourceUpdate[v1.PersistentVolumeClaim]
	Secret          ChildResourceUpdate[v1.Secret]
}

// The metrics server requires at least 10 seconds before container metrics can
// be considered accurate, let's wait a little longer before requesting metrics
// https://github.com/kubernetes-sigs/metrics-server/blob/9ebbad973db2a54193712c4d9292bbe3eaa849dc/pkg/storage/pod.go#L31
const freshContainerMinimalAge = 15 * time.Second

func (c ChildResource[T]) Reconcile(ctx context.Context, clnt client.Client, cr *amaltheadevv1alpha1.AmaltheaSession) ChildResourceUpdate[T] { //nolint:gocyclo
	log := log.FromContext(ctx)
	if c.Current == nil {
		return ChildResourceUpdate[T]{}
	}
	switch current := any(c.Current).(type) {
	case *networkingv1.Ingress:
		res, err := controllerutil.CreateOrPatch(ctx, clnt, current, func() error {
			// NOTE: the callback function  in CreateOrPatch will load the
			// state of the object referenced from k8s, then run the callback to update
			// the object with the applied changes
			desired, ok := any(c.Desired).(*networkingv1.Ingress)
			if !ok {
				return fmt.Errorf("could not cast when reconciling")
			}
			if current.CreationTimestamp.IsZero() {
				log.Info("Creating an ingress")
				current.Spec = desired.Spec
				current.ObjectMeta = desired.ObjectMeta
				err := ctrl.SetControllerReference(cr, current, clnt.Scheme())
				return err
			}
			switch strategy := cr.Spec.ReconcileStrategy; strategy {
			case amaltheadevv1alpha1.Never:
				return nil
			case amaltheadevv1alpha1.WhenFailedOrHibernated:
				if !(cr.Status.State == amaltheadevv1alpha1.Failed || cr.Status.State == amaltheadevv1alpha1.Hibernated) {
					return nil
				}
				fallthrough
			case amaltheadevv1alpha1.Always:
				current.Spec = desired.Spec
			default:
				return fmt.Errorf("attempting to reconcile ingress with unknown stategy %s", strategy)
			}
			return nil
		})
		return ChildResourceUpdate[T]{c.Current, res, err, nil}
	case *appsv1.StatefulSet:
		var statusCallback func(*amaltheadevv1alpha1.AmaltheaSessionStatus)
		res, err := controllerutil.CreateOrPatch(ctx, clnt, current, func() error {
			desired, ok := any(c.Desired).(*appsv1.StatefulSet)
			if !ok {
				return fmt.Errorf("could not cast when reconciling")
			}
			if current.CreationTimestamp.IsZero() {
				log.Info("Creating a statefulset")
				current.Spec = desired.Spec
				current.ObjectMeta = desired.ObjectMeta
				err := ctrl.SetControllerReference(cr, current, clnt.Scheme())
				return err
			}
			if current.Spec.Replicas != nil && desired.Spec.Replicas != nil && *current.Spec.Replicas == 0 && *desired.Spec.Replicas == 1 {
				// The session is being resumed
				statusCallback = func(status *amaltheadevv1alpha1.AmaltheaSessionStatus) {
					status.IdleSince = metav1.Time{}
					status.FailingSince = metav1.Time{}
					status.HibernatedSince = metav1.Time{}
				}
			}
			if current.Spec.Replicas != nil && desired.Spec.Replicas != nil && *current.Spec.Replicas == 1 && *desired.Spec.Replicas == 0 {
				// The session is being hibernated
				statusCallback = func(status *amaltheadevv1alpha1.AmaltheaSessionStatus) {
					status.IdleSince = metav1.Time{}
					status.FailingSince = metav1.Time{}
					status.HibernatedSince = metav1.Now()
				}
			}
			current.Spec.Template.Spec.Tolerations = desired.Spec.Template.Spec.Tolerations
			current.Spec.Template.Spec.Affinity = desired.Spec.Template.Spec.Affinity
			current.Spec.Template.Spec.NodeSelector = desired.Spec.Template.Spec.NodeSelector
			current.Spec.Template.Spec.PriorityClassName = desired.Spec.Template.Spec.PriorityClassName
			current.Spec.Replicas = desired.Spec.Replicas
			switch strategy := cr.Spec.ReconcileStrategy; strategy {
			case amaltheadevv1alpha1.Never:
				return nil
			case amaltheadevv1alpha1.WhenFailedOrHibernated:
				if !(cr.Status.State == amaltheadevv1alpha1.Failed || cr.Status.State == amaltheadevv1alpha1.Hibernated) {
					return nil
				}
				fallthrough
			case amaltheadevv1alpha1.Always:
				current.Spec.Template.Spec.Containers = desired.Spec.Template.Spec.Containers
				current.Spec.Template.Spec.InitContainers = desired.Spec.Template.Spec.InitContainers
				current.Spec.Template.Spec.Volumes = desired.Spec.Template.Spec.Volumes
			default:
				return fmt.Errorf("attempting to reconcile ingress with unknown stategy %s", strategy)
			}
			return nil
		})
		return ChildResourceUpdate[T]{c.Current, res, err, statusCallback}
	case *v1.PersistentVolumeClaim:
		res, err := controllerutil.CreateOrPatch(ctx, clnt, current, func() error {
			desired, ok := any(c.Desired).(*v1.PersistentVolumeClaim)
			if !ok {
				return fmt.Errorf("could not cast when reconciling")
			}
			if current.CreationTimestamp.IsZero() {
				log.Info("Creating a PVC")
				current.Spec = desired.Spec
				current.ObjectMeta = desired.ObjectMeta
				err := ctrl.SetControllerReference(cr, current, clnt.Scheme())
				return err
			}
			switch strategy := cr.Spec.ReconcileStrategy; strategy {
			case amaltheadevv1alpha1.Never:
				return nil
			case amaltheadevv1alpha1.WhenFailedOrHibernated:
				if !(cr.Status.State == amaltheadevv1alpha1.Failed || cr.Status.State == amaltheadevv1alpha1.Hibernated) {
					return nil
				}
				fallthrough
			case amaltheadevv1alpha1.Always:
				current.Spec.Resources.Requests = desired.Spec.Resources.Requests
				if desired.Spec.StorageClassName != nil {
					// NOTE: If the desired storage class is nil then the current spec contains the name for the default storage class
					current.Spec.StorageClassName = desired.Spec.StorageClassName
				}
			default:
				return fmt.Errorf("attempting to reconcile PVC with unknown stategy %s", strategy)
			}
			return nil
		})
		return ChildResourceUpdate[T]{c.Current, res, err, nil}
	case *v1.Service:
		res, err := controllerutil.CreateOrPatch(ctx, clnt, current, func() error {
			desired, ok := any(c.Desired).(*v1.Service)
			if !ok {
				return fmt.Errorf("could not cast when reconciling")
			}
			if current.CreationTimestamp.IsZero() {
				log.Info("Creating a service")
				current.Spec = desired.Spec
				current.ObjectMeta = desired.ObjectMeta
				err := ctrl.SetControllerReference(cr, current, clnt.Scheme())
				return err
			}
			switch strategy := cr.Spec.ReconcileStrategy; strategy {
			case amaltheadevv1alpha1.Never:
				return nil
			case amaltheadevv1alpha1.WhenFailedOrHibernated:
				if !(cr.Status.State == amaltheadevv1alpha1.Failed || cr.Status.State == amaltheadevv1alpha1.Hibernated) {
					return nil
				}
				fallthrough
			case amaltheadevv1alpha1.Always:
				current.Spec.Ports = desired.Spec.Ports
				current.Spec.Selector = desired.Spec.Selector
			default:
				return fmt.Errorf("attempting to reconcile service with unknown stategy %s", strategy)
			}
			return nil
		})
		return ChildResourceUpdate[T]{c.Current, res, err, nil}
	case *v1.Secret:
		// NOTE: When the secret is updated we have nothing that will restart the Statefulset pods
		// So secret updates on a running session will not take effect until hibernating/resuming
		// This can be added in the future - there is also nothing that will restart the session when
		// an adopted secret has changed.
		res, err := controllerutil.CreateOrPatch(ctx, clnt, current, func() error {
			desired, ok := any(c.Desired).(*v1.Secret)
			if !ok {
				return fmt.Errorf("could not cast when reconciling")
			}
			if current.CreationTimestamp.IsZero() {
				log.Info("Creating a secret")
				current.Data = desired.Data
				current.StringData = desired.StringData
				current.ObjectMeta = desired.ObjectMeta
				err := ctrl.SetControllerReference(cr, current, clnt.Scheme())
				return err
			}
			switch strategy := cr.Spec.ReconcileStrategy; strategy {
			case amaltheadevv1alpha1.Never:
				return nil
			case amaltheadevv1alpha1.WhenFailedOrHibernated:
				if !(cr.Status.State == amaltheadevv1alpha1.Failed || cr.Status.State == amaltheadevv1alpha1.Hibernated) {
					return nil
				}
				fallthrough
			case amaltheadevv1alpha1.Always:
				current.Data = desired.Data
				current.StringData = desired.StringData
			default:
				return fmt.Errorf("attempting to reconcile secret with unknown stategy %s", strategy)
			}
			return nil
		})
		return ChildResourceUpdate[T]{c.Current, res, err, nil}
	default:
		return ChildResourceUpdate[T]{Error: fmt.Errorf("encountered an uknown child resource type")}
	}
}

func NewChildResources(cr *amaltheadevv1alpha1.AmaltheaSession) (ChildResources, error) {
	metadata := metav1.ObjectMeta{Name: cr.Name, Namespace: cr.Namespace}
	secretMetadata := metav1.ObjectMeta{Name: cr.InternalSecretName(), Namespace: cr.Namespace}
	desiredService := cr.Service()
	desiredPVC := cr.PVC()
	desiredStatefulSet, err := cr.StatefulSet()
	if err != nil {
		return ChildResources{}, err
	}
	desiredIngress := cr.Ingress()
	desiredSecret := cr.Secret()
	output := ChildResources{
		Service:     ChildResource[v1.Service]{&v1.Service{ObjectMeta: metadata}, &desiredService},
		PVC:         ChildResource[v1.PersistentVolumeClaim]{&v1.PersistentVolumeClaim{ObjectMeta: metadata}, &desiredPVC},
		StatefulSet: ChildResource[appsv1.StatefulSet]{&appsv1.StatefulSet{ObjectMeta: metadata}, &desiredStatefulSet},
		Secret:      ChildResource[v1.Secret]{&v1.Secret{ObjectMeta: secretMetadata}, &desiredSecret},
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

	return output, nil
}

func (c ChildResources) Reconcile(ctx context.Context, clnt client.Client, cr *amaltheadevv1alpha1.AmaltheaSession) (ChildResourceUpdates, error) {
	output := ChildResourceUpdates{
		StatefulSet: c.StatefulSet.Reconcile(ctx, clnt, cr),
		PVC:         c.PVC.Reconcile(ctx, clnt, cr),
		Service:     c.Service.Reconcile(ctx, clnt, cr),
		Ingress:     c.Ingress.Reconcile(ctx, clnt, cr),
		Secret:      c.Secret.Reconcile(ctx, clnt, cr),
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
	return ingressOK && c.Service.UpdateResult == op && c.PVC.UpdateResult == op && c.StatefulSet.UpdateResult == op && dataSourcesOK && c.Secret.UpdateResult == op
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

func (c ChildResourceUpdates) State(cr *amaltheadevv1alpha1.AmaltheaSession, pod *v1.Pod, client client.Client, ctx context.Context) (amaltheadevv1alpha1.State, string) {
	msg := c.failureMessage(pod, cr, client, ctx)
	switch {
	case cr.GetDeletionTimestamp() != nil:
		return amaltheadevv1alpha1.NotReady, ""
	case cr.Spec.Hibernated && c.StatefulSet.Manifest.Spec.Replicas != nil && *c.StatefulSet.Manifest.Spec.Replicas == 0:
		return amaltheadevv1alpha1.Hibernated, ""
	case msg != "":
		return amaltheadevv1alpha1.Failed, msg
	case c.IsRunning(pod):
		return amaltheadevv1alpha1.Running, ""
	default:
		return amaltheadevv1alpha1.NotReady, ""
	}
}

func (c ChildResourceUpdates) failureMessage(pod *v1.Pod, cr *amaltheadevv1alpha1.AmaltheaSession, client client.Client, ctx context.Context) string {
	msg := podFailureReason(pod)
	if msg != "" {
		return msg
	}
	msg = pvcFailureReason(c.PVC.Manifest)
	if msg != "" {
		return msg
	}
	for ipvc := range c.DataSourcesPVCs {
		msg = pvcFailureReason(c.DataSourcesPVCs[ipvc].Manifest)
		if msg != "" {
			return msg
		}
	}
	msg = serviceFailureReason(c.Service.Manifest)
	if msg != "" {
		return msg
	}
	msg = ingressFailureReason(c.Ingress.Manifest)
	if msg != "" {
		return msg
	}

	msg = eventsInferedFailure(cr, client, ctx)
	if msg != "" {
		return msg
	}

	return ""
}

func eventsInferedFailure(cr *amaltheadevv1alpha1.AmaltheaSession, client client.Client, ctx context.Context) string {
	const failedScheduling = "FailedScheduling"
	const scheduled = "Scheduled"
	const triggeredScaleUp = "TriggeredScaleUp"
	log := log.FromContext(ctx)
	events, err := cr.GetPodEvents(ctx, client)
	if err != nil {
		return "Cannot get pod events"
	}
	if events == nil {
		return ""
	}
	for _, v := range slices.Backward(events.Items) {
		et := v.EventTime.Time
		if et.IsZero() {
			et = v.FirstTimestamp.Time
		}
		log.Info(fmt.Sprintf("Event[time=%s, reason=%s]", et, v.Reason))
		if v.Reason == failedScheduling {
			log.Info("Found a FailedScheduling event", "event", v)
			return v.Message
		}
		if v.Reason == scheduled || v.Reason == triggeredScaleUp {
			log.Info("Found a Scheduled or TriggeredScaleUp event", "event", v)
			return ""
		}
	}
	return ""
}

func (c ChildResourceUpdates) warningMessage(pod *v1.Pod) string {
	if pod == nil {
		return ""
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Reason == "Unschedulable" {
			return fmt.Sprintf("the session cannot be scheduled due to: %s. Please contact an administrator.", condition.Message)
		}
	}
	return ""
}

func Conditions(
	state amaltheadevv1alpha1.State,
	ctx context.Context,
	r *AmaltheaSessionReconciler,
	cr *amaltheadevv1alpha1.AmaltheaSession,
) []amaltheadevv1alpha1.AmaltheaSessionCondition {
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
				err := r.Get(ctx, namespacedName, &networkingv1.Ingress{})
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
				condition.Message = "The ingress is setup and operational"
			}
		}
		conditions[i] = condition
	}

	return conditions
}

// statusCallback modifies an amalthea status based on the reconciliation of different child resources.
// Certain status updates can only be done based on the current and previous values of child resources,
// these should take precedence over other statuses derived only on current values and are applied here.
func (c ChildResourceUpdates) statusCallback(status *amaltheadevv1alpha1.AmaltheaSessionStatus) {
	if c.Ingress.statusCallback != nil {
		c.Ingress.statusCallback(status)
	}
	if c.Service.statusCallback != nil {
		c.Service.statusCallback(status)
	}
	if c.Secret.statusCallback != nil {
		c.Secret.statusCallback(status)
	}
	if c.PVC.statusCallback != nil {
		c.PVC.statusCallback(status)
	}
	for _, dc := range c.DataSourcesPVCs {
		if dc.statusCallback != nil {
			dc.statusCallback(status)
		}
	}
	if c.StatefulSet.statusCallback != nil {
		c.StatefulSet.statusCallback(status)
	}
}

func (c ChildResourceUpdates) Status(
	ctx context.Context,
	r *AmaltheaSessionReconciler,
	cr *amaltheadevv1alpha1.AmaltheaSession,
) amaltheadevv1alpha1.AmaltheaSessionStatus {
	log := log.FromContext(ctx)

	pod, err := cr.GetPod(ctx, r.Client)
	if err != nil {
		pod = nil
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Could not read the session pod when updating the status")
		}
	}

	idle := false
	idleSince := cr.Status.IdleSince
	state, failMsg := c.State(cr, pod, r.Client, ctx)

	if pod != nil {
		oldEnough := false
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.Name == amaltheadevv1alpha1.SessionContainerName {
				if containerStatus.State.Running != nil {
					oldEnough = time.Since(containerStatus.State.Running.StartedAt.Time) >= freshContainerMinimalAge
				}
				break
			}
		}

		if state == amaltheadevv1alpha1.Running && oldEnough {
			idleSince, idle = getIdleState(ctx, r, cr)
		}
	} else {
		events := v1.EventList{}
		if err := r.List(ctx,
			&events,
			client.MatchingFields{
				"involvedObject.name":      cr.Name,
				"involvedObject.namespace": cr.Namespace,
				"involvedObject.kind":      "StatefulSet",
			},
		); err == nil {
			for _, event := range events.Items {
				if event.Reason == "FailedCreate" && strings.Contains(event.Message, "exceeded quota") {
					state = amaltheadevv1alpha1.Failed
					failMsg = "Quota exceeded: Your resource pool does not contain enough free resources (CPU / Memory / GPU / Storage) to schedule the session"
				}
			}
		} else {
			log.Error(err, "couldn't list events")

		}
	}

	hibernated := cr.Spec.Hibernated
	hibernatedSince := cr.Status.HibernatedSince
	if hibernated && hibernatedSince.IsZero() {
		hibernatedSince = metav1.NewTime(time.Now())
	}
	if !hibernated && !hibernatedSince.IsZero() {
		hibernatedSince = metav1.Time{}
	}

	failing := state == amaltheadevv1alpha1.Failed
	failingSince := cr.Status.FailingSince
	if failing && failingSince.IsZero() {
		failingSince = metav1.NewTime(time.Now())
	}
	if !hibernated && !failingSince.IsZero() {
		failingSince = metav1.Time{}
	}

	status := amaltheadevv1alpha1.AmaltheaSessionStatus{
		Conditions:      Conditions(state, ctx, r, cr),
		State:           state,
		URL:             cr.GetURLString(),
		Idle:            idle,
		IdleSince:       idleSince,
		FailingSince:    failingSince,
		HibernatedSince: hibernatedSince,
		Error:           failMsg,
	}
	warning := c.warningMessage(pod)
	if status.Error == "" && warning != "" {
		status.Error = warning
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

	c.statusCallback(&status)

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
		"Secret":      c.Secret.Error,
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
