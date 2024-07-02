package v1alpha1

import (
	"context"
	"fmt"

	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/util/intstr"
)


type AmaltheaChildren struct {
	Ingress     *networkingv1.Ingress
	Service     v1.Service
	StatefulSet appsv1.StatefulSet
	PVC         v1.PersistentVolumeClaim
}


func (cr *AmaltheaSession) Children() AmaltheaChildren {
	return AmaltheaChildren{
		StatefulSet: cr.StatefulSet(),
		Service:     cr.Service(),
		Ingress:     cr.Ingress(),
	}
}

// StatefulSet returns a AmaltheaSession StatefulSet object
func (cr *AmaltheaSession) StatefulSet() appsv1.StatefulSet {
	labels := labelsForAmaltheaSession(cr.Name)
	replicas := int32(1)

	session := cr.Spec.Session

	sessionContainer := v1.Container{
		Image:           session.Image,
		Name:            "session",
		ImagePullPolicy: v1.PullIfNotPresent,

		Ports: []v1.ContainerPort{{
			ContainerPort: session.Port,
			Name:          "session-port",
		}},

		Args:      session.Args,
		Command:   session.Command,
		Env:       session.Env,
		Resources: session.Resources,
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
	containers = append(containers, cr.Spec.ExtraContainers...)

	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            cr.Name,
			Namespace:       cr.Namespace,
			OwnerReferences: []metav1.OwnerReference{cr.OwnerReference()},
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
					InitContainers: cr.Spec.ExtraInitContainers,
				},
			},
		},
	}
}

// Service returns a AmaltheaSession Service object
func (cr *AmaltheaSession) Service() v1.Service {
	labels := labelsForAmaltheaSession(cr.Name)

	return v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            cr.Name,
			Namespace:       cr.Namespace,
			OwnerReferences: []metav1.OwnerReference{cr.OwnerReference()},
		},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Ports: []v1.ServicePort{{
				Name:       "session-port",
				Port:       80,
				TargetPort: intstr.FromInt32(cr.Spec.Session.Port),
			}},
		},
	}
}

// Ingress returns a AmaltheaSession Ingress object
func (cr *AmaltheaSession) Ingress() *networkingv1.Ingress {
	if reflect.DeepEqual(cr.Spec.Ingress, Ingress{}) {
		return nil
	}

	labels := labelsForAmaltheaSession(cr.Name)

	ingress := cr.Spec.Ingress

	path := "/"
	if ingress.PathPrefix != "" {
		path = ingress.PathPrefix
	}

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            cr.Name,
			Namespace:       cr.Namespace,
			Labels:          labels,
			Annotations:     ingress.Annotations,
			OwnerReferences: []metav1.OwnerReference{cr.OwnerReference()},
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
									Name: cr.Name,
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

func (cr *AmaltheaSession) OwnerReference() metav1.OwnerReference {
	gvk := cr.GroupVersionKind()
	return metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               cr.ObjectMeta.Name,
		BlockOwnerDeletion: pointer.Bool(true),
		Controller:         pointer.Bool(true),
		UID:                cr.GetObjectMeta().GetUID(),
	}
}

func (cr *AmaltheaSession) Pod(ctx context.Context, clnt client.Client) (v1.Pod, error) {
	pod := v1.Pod{}
	podName := fmt.Sprintf("%s-0", cr.Name)
	key := types.NamespacedName{Name: podName, Namespace: cr.GetNamespace()}
	err := clnt.Get(ctx, key, &pod)
	return pod, err
}
