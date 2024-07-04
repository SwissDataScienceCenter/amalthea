package controller

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Ingress     ChildResource[networkingv1.Ingress]
	Service     ChildResource[v1.Service]
	StatefulSet ChildResource[appsv1.StatefulSet]
	PVC         ChildResource[v1.PersistentVolumeClaim]
}

type ChildResourceUpdates struct {
	Ingress     ChildResourceUpdate[networkingv1.Ingress]
	Service     ChildResourceUpdate[v1.Service]
	StatefulSet ChildResourceUpdate[appsv1.StatefulSet]
	PVC         ChildResourceUpdate[v1.PersistentVolumeClaim]
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
	return output
}

func (c ChildResources) Reconcile(ctx context.Context, clnt client.Client, cr *amaltheadevv1alpha1.AmaltheaSession) (ChildResourceUpdates, error) {
	output := ChildResourceUpdates{
		StatefulSet: c.StatefulSet.Reconcile(ctx, clnt, cr),
		PVC:         c.PVC.Reconcile(ctx, clnt, cr),
		Service:     c.Service.Reconcile(ctx, clnt, cr),
		Ingress:     c.Ingress.Reconcile(ctx, clnt, cr),
	}
	return output, output.combineErrors()
}

func (c ChildResourceUpdates) AllEqual(op controllerutil.OperationResult) bool {
	ingressOK := c.Ingress.Manifest == nil || (c.Ingress.Manifest != nil && c.Ingress.UpdateResult == op)
	return ingressOK && c.Service.UpdateResult == op && c.PVC.UpdateResult == op && c.StatefulSet.UpdateResult == op
}

func (c ChildResourceUpdates) IsRunning(cr *amaltheadevv1alpha1.AmaltheaSession) bool {
	onlyStatusUpdates := c.AllEqual(controllerutil.OperationResultUpdatedStatusOnly)
	noUpdates := c.AllEqual(controllerutil.OperationResultNone)
	ssReady := c.StatefulSet.Manifest.Status.ReadyReplicas == 1 && c.StatefulSet.Manifest.Status.Replicas == 1
	return ssReady && (onlyStatusUpdates || noUpdates)
}

func (c ChildResourceUpdates) State(cr *amaltheadevv1alpha1.AmaltheaSession) amaltheadevv1alpha1.State {
	switch {
	case cr.GetDeletionTimestamp() != nil:
		return amaltheadevv1alpha1.NotReady
	case cr.Spec.Hibernated && c.StatefulSet.Manifest.Spec.Replicas != nil && *c.StatefulSet.Manifest.Spec.Replicas == 0:
		return amaltheadevv1alpha1.Hibernated
	case c.IsRunning(cr):
		return amaltheadevv1alpha1.Running
	default:
		return amaltheadevv1alpha1.NotReady
	}
}

func (c ChildResourceUpdates) Status(ctx context.Context, clnt client.Client, cr *amaltheadevv1alpha1.AmaltheaSession) amaltheadevv1alpha1.AmaltheaSessionStatus {
	log := log.FromContext(ctx)

	idle := isIdle()
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

	failing := isFailing()
	failingSince := cr.Status.FailingSince
	if failing && failingSince.IsZero() {
		failingSince = metav1.NewTime(time.Now())
	}
	if !hibernated && !failingSince.IsZero() {
		failingSince = metav1.Time{}
	}

	sessionURLStr := "None"
	if cr.Spec.Ingress != nil {
		urlScheme := "http"
		if cr.Spec.Ingress.TLSSecretName != nil {
			urlScheme = "https"
		}
		pathPrefix := "/"
		if cr.Spec.Ingress.PathPrefix != nil {
			pathPrefix = *cr.Spec.Ingress.PathPrefix
		}
		sessionURL := url.URL{
			Scheme: urlScheme,
			Path:   pathPrefix,
			Host:   cr.Spec.Ingress.Host,
		}
		sessionURL = *sessionURL.JoinPath(cr.Spec.Session.URLPath)
		sessionURLStr = strings.TrimSuffix(sessionURL.String(), "/")
	}

	state := c.State(cr)

	// Used for debugging to ensure the reconcile loop does not needlessly reschdule or update child resources
	log.Info("Update summary", "Ingress", c.Ingress.UpdateResult, "StatefulSet", c.StatefulSet.UpdateResult, "PVC", c.StatefulSet.UpdateResult, "Service", c.Service.UpdateResult)

	return amaltheadevv1alpha1.AmaltheaSessionStatus{
		State:           state,
		URL:             sessionURLStr,
		Idle:            idle,
		IdleSince:       idleSince,
		FailingSince:    failingSince,
		HibernatedSince: hibernatedSince,
	}
}

func (c ChildResourceUpdates) combineErrors() error {
	errorMsgs := []string{}
	errors := map[string]error{
		"Ingress":     c.Ingress.Error,
		"Service":     c.Service.Error,
		"PVC":         c.PVC.Error,
		"StatefulSet": c.StatefulSet.Error,
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
