package v1alpha1

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NOTE: changing these constant values will result in breaking changes or restarts in existing sessions when a new operator is released
// We should prefix reserved names like below with `amalthea-` and then add checks in our spec to prevent people from naming things where they
// start with the same `amalthea-` prefix.
const prefix string = "amalthea-"
const SessionContainerName string = prefix + "session"
const servicePortName string = prefix + "http"
const servicePort int32 = 80
const sessionVolumeName string = prefix + "volume"
const shmVolumeName string = prefix + "dev-shm"
const authProxyPort int32 = 65535

var sidecarsImage string = getSidecarsImage()
var rcloneStorageClass string = getStorageClass()
var rcloneDefaultStorage resource.Quantity = resource.MustParse("1Gi")

const rcloneStorageSecretNameAnnotation = "csi-rclone.dev/secretName"

func (cr *AmaltheaSession) SessionVolumes() ([]v1.Volume, []v1.VolumeMount) {
	pvc := cr.PVC()
	volumes := []v1.Volume{
		{
			Name: sessionVolumeName,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		},
	}
	volumeMounts := []v1.VolumeMount{
		{
			Name:      sessionVolumeName,
			MountPath: cr.Spec.Session.Storage.MountPath,
		},
	}

	if cr.Spec.Session.ShmSize != nil {
		volumes = append(volumes,
			v1.Volume{
				Name: shmVolumeName,
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{
						Medium:    v1.StorageMediumMemory,
						SizeLimit: cr.Spec.Session.ShmSize,
					},
				},
			},
		)
		volumeMounts = append(volumeMounts,
			v1.VolumeMount{
				Name:      shmVolumeName,
				MountPath: "/dev/shm",
			},
		)
	}

	return volumes, volumeMounts
}

// StatefulSet returns a AmaltheaSession StatefulSet object
func (cr *AmaltheaSession) StatefulSet() (appsv1.StatefulSet, error) {
	labels := labelsForAmaltheaSession(cr.Name)
	replicas := int32(1)
	if cr.Spec.Hibernated {
		replicas = 0
	}

	session := cr.Spec.Session
	volumes := []v1.Volume{}
	volumeMounts := []v1.VolumeMount{}
	initContainers := []v1.Container{}
	containers := []v1.Container{}

	_, dsVols, dsVolMounts := cr.DataSources()
	cloneInit := cr.cloneInit()
	sessionVols, sessionMounts := cr.SessionVolumes()
	volumes = append(volumes, sessionVols...)
	volumes = append(volumes, cloneInit.Volumes...)
	volumes = append(volumes, cr.Spec.ExtraVolumes...)
	volumes = append(volumes, dsVols...)
	volumeMounts = append(volumeMounts, sessionMounts...)
	volumeMounts = append(volumeMounts, cr.Spec.Session.ExtraVolumeMounts...)
	volumeMounts = append(volumeMounts, dsVolMounts...)
	initContainers = append(initContainers, cloneInit.Containers...)
	initContainers = append(initContainers, cr.Spec.ExtraInitContainers...)

	// NOTE: ports on a container are for information purposes only, so they are removed because the port specified
	// in the CR can point to either the session container or another container.
	sessionContainer := v1.Container{
		Image:                    session.Image,
		Name:                     SessionContainerName,
		ImagePullPolicy:          cr.Spec.Session.ImagePullPolicy,
		Args:                     session.Args,
		Command:                  session.Command,
		Env:                      session.Env,
		Resources:                session.Resources,
		VolumeMounts:             volumeMounts,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: v1.TerminationMessageReadFile,
	}

	// Assign a readiness probe to the session container
	switch cr.Spec.Session.ReadinessProbe.Type {
	case HTTP:
		sessionContainer.ReadinessProbe = &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Port: intstr.FromInt32(cr.Spec.Session.Port),
					Path: cr.Spec.Session.URLPath,
				},
			},
			SuccessThreshold:    5,
			PeriodSeconds:       5,
			InitialDelaySeconds: 10,
		}
	case TCP:
		sessionContainer.ReadinessProbe = &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				TCPSocket: &v1.TCPSocketAction{
					Port: intstr.FromInt32(cr.Spec.Session.Port),
				},
			},
			SuccessThreshold:    5,
			PeriodSeconds:       5,
			InitialDelaySeconds: 10,
		}
	}

	securityContext := &v1.SecurityContext{
		RunAsNonRoot: ptr.To(true),
		RunAsUser:    ptr.To(session.RunAsUser),
		RunAsGroup:   ptr.To(session.RunAsGroup),
	}
	if session.RunAsUser == 0 {
		securityContext.RunAsNonRoot = ptr.To(false)
	}
	sessionContainer.SecurityContext = securityContext

	auth := cr.auth()
	containers = append(containers, sessionContainer)
	containers = append(containers, auth.Containers...)
	containers = append(containers, cr.Spec.ExtraContainers...)
	volumes = append(volumes, auth.Volumes...)

	imagePullSecrets := []v1.LocalObjectReference{}
	for _, sec := range cr.Spec.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, v1.LocalObjectReference{Name: sec.Name})
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
					EnableServiceLinks:           ptr.To(false),
					AutomountServiceAccountToken: ptr.To(false),
					SecurityContext:              &v1.PodSecurityContext{FSGroup: &cr.Spec.Session.RunAsGroup},
					Containers:                   containers,
					InitContainers:               initContainers,
					Volumes:                      volumes,
					Tolerations:                  cr.Spec.Tolerations,
					NodeSelector:                 cr.Spec.NodeSelector,
					Affinity:                     cr.Spec.Affinity,
					PriorityClassName:            cr.Spec.PriorityClassName,
					ImagePullSecrets:             imagePullSecrets,
				},
			},
		},
	}
	return sts, nil
}

// Service returns a AmaltheaSession Service object
func (cr *AmaltheaSession) Service() v1.Service {
	labels := labelsForAmaltheaSession(cr.Name)
	targetPort := cr.Spec.Session.Port
	if cr.Spec.Authentication != nil && cr.Spec.Authentication.Enabled {
		targetPort = authProxyPort
	}

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
				TargetPort: intstr.FromInt32(targetPort),
			}},
		},
	}
	return svc
}

// The localhost path prefix URL leading straight to the session. It excludes the auth proxy and the ingress and
// the host is always 127.0.0.1.
func (cr *AmaltheaSession) localhostPathPrefixURL() *url.URL {
	host := fmt.Sprintf("127.0.0.1:%d", cr.Spec.Session.Port)
	path := cr.Spec.Ingress.PathPrefix
	// NOTE: If the url does not end with "/" then the oauth2proxy proxies only the exact path
	// and does not proxy subpaths
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	output := url.URL{Host: host, Scheme: "http", Path: path}
	return &output
}

// Ingress returns a AmaltheaSession Ingress object
func (cr *AmaltheaSession) Ingress() *networkingv1.Ingress {
	labels := labelsForAmaltheaSession(cr.Name)

	ingress := cr.Spec.Ingress

	if ingress == nil {
		return nil
	}

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
							Path: cr.localhostPathPrefixURL().Path,
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
		},
	}

	if ingress.TLSSecret != nil && ingress.TLSSecret.Name != "" {
		ing.Spec.TLS = []networkingv1.IngressTLS{{
			Hosts:      []string{ingress.Host},
			SecretName: ingress.TLSSecret.Name,
		}}
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
			Resources:        v1.VolumeResourceRequirements{Requests: requests},
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

func NewConditions() []AmaltheaSessionCondition {
	now := metav1.Now()
	return []AmaltheaSessionCondition{
		{
			Type:               AmaltheaSessionReady,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: now,
			Reason:             "SessionCreated",
			Message:            "The custom resource was created just now",
		},
		{
			Type:               AmaltheaSessionRoutingReady,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: now,
			Reason:             "SessionCreated",
			Message:            "The custom resource was created just now",
		},
	}
}

func (cr *AmaltheaSession) NeedsDeletion() bool {
	hibernatedDuration := time.Since(cr.Status.HibernatedSince.Time)
	durationIsZero := cr.Spec.Culling.MaxHibernatedDuration == metav1.Duration{}
	return cr.Status.State == Hibernated && !durationIsZero &&
		hibernatedDuration > cr.Spec.Culling.MaxHibernatedDuration.Duration
}

func (cr *AmaltheaSession) GetPod(ctx context.Context, clnt client.Client) (*v1.Pod, error) {
	pod := v1.Pod{}
	podName := fmt.Sprintf("%s-0", cr.Name)
	key := types.NamespacedName{Name: podName, Namespace: cr.GetNamespace()}
	err := clnt.Get(ctx, key, &pod)
	if err != nil {
		return nil, err
	}
	return &pod, err
}

// Returns the list of all the secrets used in this CR
func (cr *AmaltheaSession) AdoptedSecrets() v1.SecretList {
	secrets := v1.SecretList{}

	if cr.Spec.Ingress != nil && cr.Spec.Ingress.TLSSecret.isAdopted() {
		secrets.Items = append(secrets.Items, v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cr.Namespace,
				Name:      cr.Spec.Ingress.TLSSecret.Name,
			},
		})
	}

	auth := cr.Spec.Authentication
	if auth != nil && auth.Enabled && auth.SecretRef.isAdopted() {
		secrets.Items = append(secrets.Items, v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cr.Namespace,
				Name:      auth.SecretRef.Name,
			},
		})
	}

	for _, pv := range cr.Spec.DataSources {
		if pv.SecretRef.isAdopted() {
			secrets.Items = append(secrets.Items, v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pv.SecretRef.Name,
					Namespace: cr.Namespace,
				},
			})
		}
	}

	for _, codeRepo := range cr.Spec.CodeRepositories {
		if codeRepo.CloningConfigSecretRef.isAdopted() {
			secrets.Items = append(secrets.Items, v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      codeRepo.CloningConfigSecretRef.Name,
					Namespace: cr.Namespace,
				},
			})
		}
		if codeRepo.ConfigSecretRef.isAdopted() {
			secrets.Items = append(secrets.Items, v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      codeRepo.ConfigSecretRef.Name,
					Namespace: cr.Namespace,
				},
			})
		}
	}

	for _, imagePull := range cr.Spec.ImagePullSecrets {
		if imagePull.isAdopted() {
			secrets.Items = append(secrets.Items, v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      imagePull.Name,
					Namespace: cr.Namespace,
				},
			})
		}
	}

	return secrets
}

// Assuming that the csi-rclone driver from https://github.com/SwissDataScienceCenter/csi-rclone
// is installed, this will generate PVCs for the data sources that have the rclone type.
func (as *AmaltheaSession) DataSources() ([]v1.PersistentVolumeClaim, []v1.Volume, []v1.VolumeMount) {
	pvcs := []v1.PersistentVolumeClaim{}
	vols := []v1.Volume{}
	volMounts := []v1.VolumeMount{}
	for ids, ds := range as.Spec.DataSources {
		pvcName := fmt.Sprintf("%s%s-ds-%d", prefix, as.Name, ids)
		switch ds.Type {
		case Rclone:
			storageClass := rcloneStorageClass
			readOnly := ds.AccessMode == v1.ReadOnlyMany
			pvcs = append(
				pvcs,
				v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcName,
						Namespace: as.Namespace,
						Annotations: map[string]string{
							rcloneStorageSecretNameAnnotation: ds.SecretRef.Name,
						},
					},
					Spec: v1.PersistentVolumeClaimSpec{
						AccessModes: []v1.PersistentVolumeAccessMode{ds.AccessMode},
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceStorage: rcloneDefaultStorage,
							},
						},
						StorageClassName: &storageClass,
					},
				},
			)
			vols = append(
				vols,
				v1.Volume{
					Name: pvcName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
							ReadOnly:  readOnly,
						},
					},
				},
			)
			volMounts = append(
				volMounts,
				v1.VolumeMount{
					Name:      pvcName,
					ReadOnly:  readOnly,
					MountPath: ds.MountPath,
				},
			)
		default:
			continue
		}
	}
	return pvcs, vols, volMounts
}

func getStorageClass() string {
	sc := os.Getenv("RCLONE_STORAGE_CLASS")
	if sc == "" {
		sc = "csi-rclone-secret-annotation"
	}
	return sc
}

func getSidecarsImage() string {
	sc := os.Getenv("SIDECARS_IMAGE")
	if sc == "" {
		sc = "renku/sidecars:latest"
	}
	return sc
}
