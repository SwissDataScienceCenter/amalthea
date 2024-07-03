package v1alpha1

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NOTE: changing these constant values will result in breaking changes or restarts in existing sessions when a new operator is released
const sessionContainerName string = "session"
const sessionPortName string = "session-port"
const servicePortName string = "session-http"
const sessionVolumeName string = "session-volume"
const servicePort int32 = 80

// StatefulSet returns a AmaltheaSession StatefulSet object
func (cr *AmaltheaSession) StatefulSet() appsv1.StatefulSet {
	labels := labelsForAmaltheaSession(cr.Name)
	replicas := int32(1)
	if cr.Spec.Hibernated {
		replicas = 0
	}

	session := cr.Spec.Session
	pvc := cr.PVC()

	sessionContainer := v1.Container{
		Image:           session.Image,
		Name:            sessionContainerName,
		ImagePullPolicy: v1.PullIfNotPresent,

		Ports: []v1.ContainerPort{{
			ContainerPort: session.Port,
			Name:          sessionPortName,
			Protocol:      v1.ProtocolTCP,
		}},

		Args:                     session.Args,
		Command:                  session.Command,
		Env:                      session.Env,
		Resources:                session.Resources,
		VolumeMounts:             []v1.VolumeMount{{Name: sessionVolumeName, MountPath: session.Storage.MountPath}},
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
	containers = append(containers, cr.Spec.ExtraContainers...)

	sts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
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
					Volumes: []v1.Volume{
						{
							Name:         sessionVolumeName,
							VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}},
						},
					},
				},
			},
		},
	}
	return sts
}

// Service returns a AmaltheaSession Service object
func (cr *AmaltheaSession) Service() v1.Service {
	labels := labelsForAmaltheaSession(cr.Name)

	svc := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Ports: []v1.ServicePort{{
				Protocol:   v1.ProtocolTCP,
				Name:       servicePortName,
				Port:       servicePort,
				TargetPort: intstr.FromInt32(cr.Spec.Session.Port),
			}},
		},
	}
	return svc
}

// Ingress returns a AmaltheaSession Ingress object
func (cr *AmaltheaSession) Ingress() *networkingv1.Ingress {
	labels := labelsForAmaltheaSession(cr.Name)

	ingress := cr.Spec.Ingress

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Name,
			Namespace:   cr.Namespace,
			Labels:      labels,
			Annotations: ingress.Annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ingress.IngressClassName,
			Rules: []networkingv1.IngressRule{{
				Host: ingress.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path: ingress.PathPrefix,
							PathType: func() *networkingv1.PathType {
								pt := networkingv1.PathTypePrefix
								return &pt
							}(),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: cr.Name,
									Port: networkingv1.ServiceBackendPort{
										Name: servicePortName,
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

// PVC returned the desired specification for a persistent volume claim
func (cr *AmaltheaSession) PVC() v1.PersistentVolumeClaim {
	labels := labelsForAmaltheaSession(cr.Name)
	requests := v1.ResourceList{"storage": resource.MustParse("1Gi")}
	if cr.Spec.Session.Storage.Size != nil {
		requests = v1.ResourceList{"storage": *cr.Spec.Session.Storage.Size}
	}

	pvc := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources:        v1.ResourceRequirements{Requests: requests},
			StorageClassName: cr.Spec.Session.Storage.ClassName,
		},
	}
	return pvc
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

func (cr *AmaltheaSession) Pod(ctx context.Context, clnt client.Client) (v1.Pod, error) {
	pod := v1.Pod{}
	podName := fmt.Sprintf("%s-0", cr.Name)
	key := types.NamespacedName{Name: podName, Namespace: cr.GetNamespace()}
	err := clnt.Get(ctx, key, &pod)
	return pod, err
}
