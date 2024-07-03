package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NOTE: changing these constant values will result in breaking changes or restarts in existing sessions when a new operator is released
const sessionContainerName string = "session"
const sessionPortName string = "session-port"
const servicePortName string = "session-http"
const sessionVolumeName string = "session-volume"
const servicePort int32 = 80
const gitCloneImage string = "bitnami/git:2.45.2"
const gitCloneContainerName string = "git-clone"

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
		SecurityContext:          cr.securityContext(),
	}

	containers := append([]v1.Container{sessionContainer}, cr.Spec.ExtraContainers...)
	initContainers := append([]v1.Container{}, cr.Spec.ExtraInitContainers...)
	volumes := []v1.Volume{
		{
			Name:         sessionVolumeName,
			VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}},
		},
	}
	if len(cr.Spec.CodeRepositories) > 0 {
		gitCloneContainer, gitCloneVols := cr.initClone()
		initContainers = append(initContainers, gitCloneContainer)
		volumes = append(volumes, gitCloneVols...)
	}

	sts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			// NOTE: Parallel pod management policy is important
			// If set to default (i.e. not parallel) K8s waits for the pod to become ready before restarting on updates
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Replicas:            &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					SecurityContext: cr.podSecurityContext(),
					Containers:      containers,
					InitContainers:  initContainers,
					Volumes:         volumes,
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

func (cr *AmaltheaSession) Pod(ctx context.Context, clnt client.Client) (*v1.Pod, error) {
	pod := v1.Pod{}
	podName := fmt.Sprintf("%s-0", cr.Name)
	key := types.NamespacedName{Name: podName, Namespace: cr.GetNamespace()}
	err := clnt.Get(ctx, key, &pod)
	return &pod, err
}

// Generates the init container that clones the specified Git repositories
func (cr *AmaltheaSession) initClone() (v1.Container, []v1.Volume) {
	envVars := []v1.EnvVar{}
	volMounts := []v1.VolumeMount{{Name: sessionVolumeName, MountPath: cr.Spec.Session.Storage.MountPath}}
	vols := []v1.Volume{}
	commandArgs := []string{}
	gitConfigInd := 0

	for irepo, repo := range cr.Spec.CodeRepositories {
		if repo.CloningConfigSecretRef != nil {
			secretVolName := fmt.Sprintf("git-clone-cred-volume-%d-%d", irepo, gitConfigInd)
			secretMountPath := fmt.Sprintf("/git-clone-secrets/%d-%d", irepo, gitConfigInd)
			secretFilePath := fmt.Sprintf("%s/%s", secretMountPath, repo.CloningConfigSecretRef.Key)
			vols = append(
				vols,
				v1.Volume{
					Name:         secretVolName,
					VolumeSource: v1.VolumeSource{Secret: &v1.SecretVolumeSource{SecretName: repo.CloningConfigSecretRef.Name}},
				},
			)
			volMounts = append(volMounts, v1.VolumeMount{Name: secretVolName, MountPath: secretMountPath})
			envVars = append(
				envVars,
				v1.EnvVar{Name: fmt.Sprintf("GIT_CONFIG_KEY_%d", gitConfigInd), Value: "include.path"},
				v1.EnvVar{Name: fmt.Sprintf("GIT_CONFIG_VALUE_%d", gitConfigInd), Value: secretFilePath},
			)
			gitConfigInd += 1
		}
		exec := fmt.Sprintf("git clone %s --recurse-submodules", repo.Remote)
		if repo.Revision != nil {
			exec += fmt.Sprintf(" --branch %s", *repo.Revision)
		}
		if repo.ClonePath != nil {
			exec += fmt.Sprintf(" %s", *repo.ClonePath)
		}
		commandArgs = append(commandArgs, exec)
	}
	if gitConfigInd > 0 {
		envVars = append(envVars, v1.EnvVar{Name: "GIT_CONFIG_COUNT", Value: fmt.Sprintf("%d", gitConfigInd-1)})
	}

	container := v1.Container{
		Name:            gitCloneContainerName,
		Image:           gitCloneImage,
		VolumeMounts:    volMounts,
		WorkingDir:      cr.Spec.Session.Storage.MountPath,
		Env:             envVars,
		SecurityContext: &v1.SecurityContext{RunAsUser: &cr.Spec.Session.RunAsUser, RunAsGroup: &cr.Spec.Session.RunAsGroup},
		Command:         []string{"sh", "-c"},
		Args:            []string{strings.Join(commandArgs, " || ") + " || echo 'Some repositories could not be cloned'"},
	}
	return container, vols
}

func (cr *AmaltheaSession) securityContext() *v1.SecurityContext {
	return &v1.SecurityContext{
		RunAsUser:                &cr.Spec.Session.RunAsUser,
		RunAsGroup:               &cr.Spec.Session.RunAsGroup,
		RunAsNonRoot:             ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
	}
}

func (cr *AmaltheaSession) podSecurityContext() *v1.PodSecurityContext {
	return &v1.PodSecurityContext{
		RunAsUser:    &cr.Spec.Session.RunAsUser,
		RunAsGroup:   &cr.Spec.Session.RunAsGroup,
		RunAsNonRoot: ptr.To(true),
		FSGroup:      &cr.Spec.Session.RunAsGroup,
	}
}
