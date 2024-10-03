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
const sessionContainerName string = prefix + "session"
const servicePortName string = prefix + "http"
const servicePort int32 = 80
const sessionVolumeName string = prefix + "volume"
const shmVolumeName string = prefix + "dev-shm"
const authProxyPort int32 = 65535
const oauth2ProxyImage = "bitnami/oauth2-proxy:7.6.0"
const authProxyImage = "renku/authproxy:0.0.1-test-1"

var rcloneStorageClass string = getStorageClass()
var rcloneDefaultStorage resource.Quantity = resource.MustParse("1Gi")

const rcloneStorageSecretNameAnnotation = "csi-rclone.dev/secretName"

// StatefulSet returns a AmaltheaSession StatefulSet object
func (cr *AmaltheaSession) StatefulSet() appsv1.StatefulSet {
	labels := labelsForAmaltheaSession(cr.Name)
	replicas := int32(1)
	if cr.Spec.Hibernated {
		replicas = 0
	}

	session := cr.Spec.Session
	pvc := cr.PVC()
	extraMounts := []v1.VolumeMount{}
	if len(cr.Spec.Session.ExtraVolumeMounts) > 0 {
		extraMounts = cr.Spec.Session.ExtraVolumeMounts
	}
	_, dsVols, dsVolMounts := cr.DataSources()

	volumeMounts := append(
		[]v1.VolumeMount{
			{
				Name:      sessionVolumeName,
				MountPath: session.Storage.MountPath,
			},
		},
		extraMounts...,
	)
	volumeMounts = append(volumeMounts, dsVolMounts...)

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
	volumes = append(volumes, dsVols...)

	if len(cr.Spec.ExtraVolumes) > 0 {
		volumes = append(volumes, cr.Spec.ExtraVolumes...)
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

	initContainers := []v1.Container{}

	if len(cr.Spec.CodeRepositories) > 0 {
		gitCloneContainers, gitCloneVols := cr.initClones()
		initContainers = append(initContainers, gitCloneContainers...)
		volumes = append(volumes, gitCloneVols...)
	}

	initContainers = append(initContainers, cr.Spec.ExtraInitContainers...)

	// NOTE: ports on a container are for information purposes only, so they are removed because the port specified
	// in the CR can point to either the session container or another container.
	sessionContainer := v1.Container{
		Image:                    session.Image,
		Name:                     sessionContainerName,
		ImagePullPolicy:          v1.PullIfNotPresent,
		Args:                     session.Args,
		Command:                  session.Command,
		Env:                      session.Env,
		Resources:                session.Resources,
		VolumeMounts:             volumeMounts,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: v1.TerminationMessageReadFile,
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

	containers := []v1.Container{sessionContainer}
	containers = append(containers, cr.Spec.ExtraContainers...)

	if auth := cr.Spec.Authentication; auth != nil && auth.Enabled {
		extraAuthMounts := []v1.VolumeMount{}
		if len(auth.ExtraVolumeMounts) > 0 {
			extraAuthMounts = auth.ExtraVolumeMounts
		}
		volumes = append(volumes, v1.Volume{
			Name: "proxy-configuration-secret",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: auth.SecretRef.Name,
					Optional:   ptr.To(false),
				},
			},
		})

		if auth.Type == Oidc {
			sessionURL := cr.sessionLocalhostURL().String()
			if !strings.HasSuffix(sessionURL, "/") {
				// NOTE: If the url does not end with "/" then the oauth2proxy proxies only the exact path
				// and does not proxy subpaths
				sessionURL += "/"
			}
			authContainer := v1.Container{
				Image: oauth2ProxyImage,
				Name:  "oauth2-proxy",
				SecurityContext: &v1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					RunAsNonRoot:             ptr.To(true),
				},
				Args: []string{
					fmt.Sprintf("--upstream=%s", sessionURL),
					fmt.Sprintf("--http-address=:%d", authProxyPort),
					"--silence-ping-logging",
					"--config=/etc/oauth2-proxy/" + auth.SecretRef.Key,
				},
				VolumeMounts: append(
					[]v1.VolumeMount{
						{
							Name:      "proxy-configuration-secret",
							MountPath: "/etc/oauth2-proxy",
						},
					},
					extraAuthMounts...,
				),
			}

			containers = append(containers, authContainer)
		} else if auth.Type == Token {
			authContainer := v1.Container{
				Image: authProxyImage,
				Name:  "authproxy",
				SecurityContext: &v1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					RunAsNonRoot:             ptr.To(true),
					RunAsUser:                ptr.To(int64(1000)),
					RunAsGroup:               ptr.To(int64(1000)),
				},
				Args: []string{"serve", "--config", "/etc/authproxy/" + auth.SecretRef.Key},
				Env: []v1.EnvVar{
					{Name: "AUTHPROXY_PORT", Value: fmt.Sprintf("%d", authProxyPort)},
					// NOTE: The url for the remote has to not have a path at all, if it does, then the path
					// in the url is appended to any path that is already there when the request comes in.
					{Name: "AUTHPROXY_REMOTE", Value: fmt.Sprintf("http://127.0.0.1:%d", cr.Spec.Session.Port)},
				},
				VolumeMounts: append(
					[]v1.VolumeMount{
						{
							Name:      "proxy-configuration-secret",
							MountPath: "/etc/authproxy",
						},
					},
					extraAuthMounts...,
				),
			}

			containers = append(containers, authContainer)
		}
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
					SecurityContext: &v1.PodSecurityContext{FSGroup: &cr.Spec.Session.RunAsGroup},
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

// The URL where the session can be accessed. It excludes the auth proxy and the ingress and
// the host is always 127.0.0.1.
func (cr *AmaltheaSession) sessionLocalhostURL() *url.URL {
	host := fmt.Sprintf("127.0.0.1:%d", cr.Spec.Session.Port)
	output := url.URL{Host: host, Scheme: "http", Path: cr.Spec.Session.URLPath}
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
							Path: cr.sessionLocalhostURL().Path,
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
	hibernatedDuration := time.Now().Sub(cr.Status.HibernatedSince.Time)
	return cr.Status.State == Hibernated &&
		hibernatedDuration > cr.Spec.Culling.MaxHibernatedDuration.Duration
}

func (cr *AmaltheaSession) Pod(ctx context.Context, clnt client.Client) (*v1.Pod, error) {
	pod := v1.Pod{}
	podName := fmt.Sprintf("%s-0", cr.Name)
	key := types.NamespacedName{Name: podName, Namespace: cr.GetNamespace()}
	err := clnt.Get(ctx, key, &pod)
	return &pod, err
}

// Generates the init containers that clones the specified Git repositories
func (cr *AmaltheaSession) initClones() ([]v1.Container, []v1.Volume) {
	envVars := []v1.EnvVar{}
	volMounts := []v1.VolumeMount{{Name: sessionVolumeName, MountPath: cr.Spec.Session.Storage.MountPath}}
	vols := []v1.Volume{}
	containers := []v1.Container{}

	for irepo, repo := range cr.Spec.CodeRepositories {
		args := []string{"clone", "--strategy", "notifexist", "--remote", repo.Remote, "--path", cr.Spec.Session.Storage.MountPath + "/" + repo.ClonePath}

		if repo.CloningConfigSecretRef != nil {
			secretVolName := fmt.Sprintf("git-clone-cred-volume-%d", irepo)
			secretMountPath := "/git-clone-secrets"
			secretFilePath := fmt.Sprintf("%s/%s", secretMountPath, repo.CloningConfigSecretRef.Key)
			vols = append(
				vols,
				v1.Volume{
					Name:         secretVolName,
					VolumeSource: v1.VolumeSource{Secret: &v1.SecretVolumeSource{SecretName: repo.CloningConfigSecretRef.Name}},
				},
			)
			volMounts = append(volMounts, v1.VolumeMount{Name: secretVolName, MountPath: secretMountPath})

			args = append(args, []string{"--config", secretFilePath}...)
		}

		if repo.Revision != "" {
			args = append(args, []string{"--revision", repo.Revision}...)
		}

		gitCloneContainerName := fmt.Sprintf("git-clone-%d", irepo)
		containers = append(containers, v1.Container{
			Name:            gitCloneContainerName,
			Image:           "renku/cloner:0.0.1",
			VolumeMounts:    volMounts,
			WorkingDir:      cr.Spec.Session.Storage.MountPath,
			Env:             envVars,
			SecurityContext: &v1.SecurityContext{RunAsUser: &cr.Spec.Session.RunAsUser, RunAsGroup: &cr.Spec.Session.RunAsGroup},
			Args:            args,
		})
	}

	return containers, vols
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

	return secrets
}

// Assuming that the csi-rclone driver from https://github.com/SwissDataScienceCenter/csi-rclone
// is installed, this will generate PVCs for the data sources that have the rclone type.
func (cr *AmaltheaSession) DataSources() ([]v1.PersistentVolumeClaim, []v1.Volume, []v1.VolumeMount) {
	pvcs := []v1.PersistentVolumeClaim{}
	vols := []v1.Volume{}
	volMounts := []v1.VolumeMount{}
	for ids, ds := range cr.Spec.DataSources {
		pvcName := fmt.Sprintf("%s-ds-%d", cr.Name, ids)
		switch ds.Type {
		case Rclone:
			storageClass := rcloneStorageClass
			readOnly := ds.AccessMode == v1.ReadOnlyMany
			pvcs = append(
				pvcs,
				v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcName,
						Namespace: cr.Namespace,
						Annotations: map[string]string{
							rcloneStorageSecretNameAnnotation: ds.SecretRef.Name,
						},
					},
					Spec: v1.PersistentVolumeClaimSpec{
						AccessModes: []v1.PersistentVolumeAccessMode{ds.AccessMode},
						Resources: v1.ResourceRequirements{
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
