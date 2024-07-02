package controller

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"time"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

func (c ChildResource[T]) Reconcile(ctx context.Context, clnt client.Client) ChildResourceUpdate[T] {
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
				return nil
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
				return nil
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
				return nil
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
				return nil
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

// func before_after_test(obj client.Object, objAfter client.Object) {
// 	obj := input.DeepCopyObject()
// 	objPatch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
// 	before, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj.DeepCopyObject())
// 	after, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
// 	if err != nil {
// 		return OperationResultNone, err
// 	}
// }

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
	desiredService := serviceForAmaltheaSession(cr)
	desiredPVC := pvcForAmalthea(cr)
	desiredStatefulSet := statefulSetForAmaltheaSession(cr)
	output := ChildResources{
		Service:     ChildResource[v1.Service]{&v1.Service{ObjectMeta: metadata}, &desiredService},
		PVC:         ChildResource[v1.PersistentVolumeClaim]{&v1.PersistentVolumeClaim{ObjectMeta: metadata}, &desiredPVC},
		StatefulSet: ChildResource[appsv1.StatefulSet]{&appsv1.StatefulSet{ObjectMeta: metadata}, &desiredStatefulSet},
	}
	if !reflect.DeepEqual(cr.Spec.Ingress, amaltheadevv1alpha1.Ingress{}) {
		desiredIngress := ingressForAmaltheaSession(cr)
		output.Ingress = ChildResource[networkingv1.Ingress]{&networkingv1.Ingress{ObjectMeta: metadata}, desiredIngress}
	}
	return output
}

func (c ChildResources) Reconcile(ctx context.Context, clnt client.Client) ChildResourceUpdates {
	output := ChildResourceUpdates{
		StatefulSet: c.StatefulSet.Reconcile(ctx, clnt),
		PVC:         c.PVC.Reconcile(ctx, clnt),
		Service:     c.Service.Reconcile(ctx, clnt),
		Ingress:     c.Ingress.Reconcile(ctx, clnt),
	}
	return output
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

	urlScheme := "http"
	if cr.Spec.Ingress.TLSSecretName != "" {
		urlScheme = "https"
	}
	sessionURL := url.URL{
		Scheme: urlScheme,
		Path:   cr.Spec.Ingress.PathPrefix,
	}
	sessionURL = *sessionURL.JoinPath(cr.Spec.Session.URLPath)

	state := c.State(cr)

	return amaltheadevv1alpha1.AmaltheaSessionStatus{
		State:           state,
		URL:             sessionURL.String(),
		Idle:            idle,
		IdleSince:       idleSince,
		FailingSince:    failingSince,
		HibernatedSince: hibernatedSince,
	}
}

// statefulSetForAmaltheaSession returns a AmaltheaSession StatefulSet object
func statefulSetForAmaltheaSession(amaltheasession *amaltheadevv1alpha1.AmaltheaSession) appsv1.StatefulSet {
	labels := labelsForAmaltheaSession(amaltheasession.Name)
	replicas := int32(1)
	if amaltheasession.Spec.Hibernated {
		replicas = 0
	}

	session := amaltheasession.Spec.Session
	pvc := pvcForAmalthea(amaltheasession)

	sessionContainer := v1.Container{
		Image:           session.Image,
		Name:            "session",
		ImagePullPolicy: v1.PullIfNotPresent,

		Ports: []v1.ContainerPort{{
			ContainerPort: session.Port,
			Name:          "session-port",
			Protocol:      v1.ProtocolTCP,
		}},

		Args:                     session.Args,
		Command:                  session.Command,
		Env:                      session.Env,
		Resources:                session.Resources,
		VolumeMounts:             []v1.VolumeMount{{Name: "session", MountPath: session.Storage.MountPath}},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: v1.TerminationMessageReadFile,
	}

	securityContext := &v1.SecurityContext{
		RunAsNonRoot: &[]bool{true}[0],
		RunAsUser:    &[]int64{session.RunAsUser}[0],
		RunAsGroup:   &[]int64{session.RunAsGroup}[0],
	}

	if session.RunAsUser == 0 {
		securityContext.RunAsNonRoot = &[]bool{false}[0]
	}

	sessionContainer.SecurityContext = securityContext

	containers := []v1.Container{sessionContainer}
	containers = append(containers, amaltheasession.Spec.ExtraContainers...)

	sts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            amaltheasession.Name,
			Namespace:       amaltheasession.Namespace,
			OwnerReferences: []metav1.OwnerReference{amaltheasession.OwnerReference()},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					Containers:     containers,
					InitContainers: amaltheasession.Spec.ExtraInitContainers,
					Volumes:        []v1.Volume{{Name: "session", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}},
				},
			},
		},
	}
	return sts
}

// pvcForAmalthea returns a AmaltheaSession PVC
func pvcForAmalthea(amaltheasession *amaltheadevv1alpha1.AmaltheaSession) v1.PersistentVolumeClaim {
	labels := labelsForAmaltheaSession(amaltheasession.Name)
	pvc := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            amaltheasession.Name,
			Namespace:       amaltheasession.Namespace,
			OwnerReferences: []metav1.OwnerReference{amaltheasession.OwnerReference()},
			Labels:          labels,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					"storage": amaltheasession.Spec.Session.Storage.Size,
				},
			},
			StorageClassName: amaltheasession.Spec.Session.Storage.ClassName,
		},
	}
	return pvc
}

// serviceForAmaltheaSession returns a AmaltheaSession Service object
func serviceForAmaltheaSession(amaltheasession *amaltheadevv1alpha1.AmaltheaSession) v1.Service {
	labels := labelsForAmaltheaSession(amaltheasession.Name)

	svc := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            amaltheasession.Name,
			Namespace:       amaltheasession.Namespace,
			OwnerReferences: []metav1.OwnerReference{amaltheasession.OwnerReference()},
		},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Ports: []v1.ServicePort{{
				Protocol:   v1.ProtocolTCP,
				Name:       "session-port",
				Port:       80,
				TargetPort: intstr.FromInt32(amaltheasession.Spec.Session.Port),
			}},
		},
	}

	return svc
}

// ingressForAmaltheaSession returns a AmaltheaSession Ingress object
func ingressForAmaltheaSession(amaltheasession *amaltheadevv1alpha1.AmaltheaSession) *networkingv1.Ingress {
	labels := labelsForAmaltheaSession(amaltheasession.Name)

	ingress := amaltheasession.Spec.Ingress
	if reflect.DeepEqual(ingress, amaltheadevv1alpha1.Ingress{}) {
		return nil
	}

	path := "/"
	if ingress.PathPrefix != "" {
		path = ingress.PathPrefix
	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            amaltheasession.Name,
			Namespace:       amaltheasession.Namespace,
			Labels:          labels,
			Annotations:     ingress.Annotations,
			OwnerReferences: []metav1.OwnerReference{amaltheasession.OwnerReference()},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingress.IngressClassName,
			Rules: []networkingv1.IngressRule{{
				Host: ingress.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path: path,
							PathType: func() *networkingv1.PathType {
								pt := networkingv1.PathTypePrefix
								return &pt
							}(),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: amaltheasession.Name,
									Port: networkingv1.ServiceBackendPort{
										Name: "session-port",
									},
								},
							},
						}},
					},
				},
			}},
			TLS: []networkingv1.IngressTLS{{
				Hosts:      []string{ingress.Host},
				SecretName: ingress.TLSSecretName,
			}},
		},
	}

	return ing
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
