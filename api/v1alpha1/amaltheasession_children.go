package v1alpha1

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NOTE: changing these constant values will result in breaking changes or restarts in existing sessions when a new operator is released
// We should prefix reserved names like below with `amalthea-` and then add checks in our spec to prevent people from naming things where they
// start with the same `amalthea-` prefix.
const prefix string = "amalthea-"
const SessionContainerName string = prefix + "session"
const servicePortName string = prefix + "http"
const serviceMetaPortName string = prefix + "http-meta"
const servicePort int32 = 80
const sessionVolumeName string = prefix + "volume"
const shmVolumeName string = prefix + "dev-shm"
const authenticatedPort int32 = 65535
const AuthProxyMetaPort int32 = 65534
const secondProxyPort int32 = 65533
const RemoteSessionControllerPort int32 = 65532

var sidecarsImage string = getSidecarsImage()
var rcloneStorageClass string = getStorageClass()
var rcloneDefaultStorage resource.Quantity = resource.MustParse("1Gi")

const rcloneStorageSecretNameAnnotation = "csi-rclone.dev/secretName"

func (cr *HpcAmaltheaSession) SessionVolumes() ([]v1.Volume, []v1.VolumeMount) {
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
func (cr *HpcAmaltheaSession) StatefulSet() (appsv1.StatefulSet, error) {
	labels := labelsForAmaltheaSession(cr.Name)
	replicas := int32(1)
	if cr.Spec.Hibernated {
		replicas = 0
	}

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

	// Create the main session container
	sessionContainer := cr.sessionContainer(volumeMounts)

	auth, err := cr.auth()
	if err != nil {
		return appsv1.StatefulSet{}, err
	}
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
					ServiceAccountName:           cr.Spec.ServiceAccountName,
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
	// Set the termination grace period for remote sessions to 60 seconds
	if cr.Spec.SessionLocation == Remote {
		sts.Spec.Template.Spec.TerminationGracePeriodSeconds = ptr.To(int64(60))
	}
	return sts, nil
}

// Service returns a AmaltheaSession Service object
func (cr *HpcAmaltheaSession) Service() v1.Service {
	labels := labelsForAmaltheaSession(cr.Name)
	targetPort := cr.Spec.Session.Port
	if cr.Spec.Authentication != nil && cr.Spec.Authentication.Enabled {
		targetPort = authenticatedPort
	}

	svc := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Name:       servicePortName,
					Port:       servicePort,
					TargetPort: intstr.FromInt32(targetPort),
				},
				{
					Protocol:   v1.ProtocolTCP,
					Name:       serviceMetaPortName,
					Port:       AuthProxyMetaPort,
					TargetPort: intstr.FromInt32(AuthProxyMetaPort),
				}},
		},
	}
	return svc
}

// The path prefix for the session
func (cr *HpcAmaltheaSession) urlPath() string {
	path := cr.Spec.Session.URLPath
	// NOTE: If the url does not end with "/" then the oauth2proxy proxies only the exact path
	// and does not proxy subpaths
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	return path
}

// The path prefix from the ingress spec for the session
func (cr *HpcAmaltheaSession) ingressPathPrefix() string {
	if cr.Spec.Ingress == nil {
		return "/"
	}
	path := cr.Spec.Ingress.PathPrefix
	// NOTE: If the url does not end with "/" then the oauth2proxy proxies only the exact path
	// and does not proxy subpaths
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	return path
}

// Ingress returns a AmaltheaSession Ingress object
func (cr *HpcAmaltheaSession) Ingress() *networkingv1.Ingress {
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
							Path: cr.ingressPathPrefix(),
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
func (cr *HpcAmaltheaSession) PVC() v1.PersistentVolumeClaim {
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

func (cr *HpcAmaltheaSession) NeedsDeletion() bool {
	hibernatedDuration := time.Since(cr.Status.HibernatedSince.Time)
	durationIsZero := cr.Spec.Culling.MaxHibernatedDuration == metav1.Duration{}
	return cr.Status.State == Hibernated && !durationIsZero &&
		hibernatedDuration > cr.Spec.Culling.MaxHibernatedDuration.Duration
}

func (cr *HpcAmaltheaSession) GetPod(ctx context.Context, clnt client.Client) (*v1.Pod, error) {
	pod := v1.Pod{}
	podName := cr.PodName()
	key := types.NamespacedName{Name: podName, Namespace: cr.GetNamespace()}
	err := clnt.Get(ctx, key, &pod)
	if err != nil {
		return nil, err
	}
	return &pod, err
}

// FirstTimestamp maybe null or eventTime is null… then it is
// available, but defaulted to their "zero" values…
func eventTimestamp(ev v1.Event) time.Time {
	t := ev.EventTime.Time
	if t.IsZero() {
		t = ev.FirstTimestamp.Time
	}
	return t
}

// GetPodEvents finds all events where the pod of the given session is
// involved in. It will be sorted by timestamp
func (as *HpcAmaltheaSession) GetPodEvents(ctx context.Context, c client.Reader) (*v1.EventList, error) {
	log := log.FromContext(ctx)
	events := v1.EventList{}
	podName := as.PodName()
	log.Info("Getting event list for pod", "pod", podName)
	err := c.List(ctx,
		&events,
		client.MatchingFields{
			"involvedObject.namespace": as.Namespace,
			"involvedObject.kind":      "Pod",
			"involvedObject.name":      podName,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cannot get pod events: %w", err)
	} else {
		sort.Slice(events.Items, func(i, j int) bool {
			t1 := eventTimestamp(events.Items[i])
			t2 := eventTimestamp(events.Items[j])
			return t1.Before(t2)
		})
		return &events, nil
	}
}

// Returns the list of all the secrets used in this CR
func (cr *HpcAmaltheaSession) AdoptedSecrets() v1.SecretList {
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
func (as *HpcAmaltheaSession) DataSources() ([]v1.PersistentVolumeClaim, []v1.Volume, []v1.VolumeMount) {
	// TODO: Configure this for remote sessions
	if as.Spec.SessionLocation == Remote {
		return []v1.PersistentVolumeClaim{}, []v1.Volume{}, []v1.VolumeMount{}
	}

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

// InternalSecretName returns the name of the secret that is a child
// of the AmaltheaSession CR, as opposed to all other adopted secrets that
// are not children of the AmaltheaSession CR and are created by the creator of each AmaltheaSession CR.
// This secret is both created and deleted by Amalthea.
func (as *HpcAmaltheaSession) InternalSecretName() string {
	return fmt.Sprintf("%s---internal", as.Name)
}

// The secret created by this method is populated with data only when the type of authentication is 'oidc'.
// If the type of authentication is 'oauth2proxy', then it is expected that
// the secret with OAuth configuration created by the creator of the AmaltheaSession CR will be in
// a format acceptable to oauth2proxy. With the 'oidc' method we do not have to expose
// the oauth2proxy configuration API in the format of the secret we expect from users.
// We define our own API - specific only to OIDC and limited strictly to fields we need.
func (as *HpcAmaltheaSession) Secret() v1.Secret {
	labels := labelsForAmaltheaSession(as.Name)
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      as.InternalSecretName(),
			Namespace: as.Namespace,
			Labels:    labels,
		},
	}
	if as.Spec.Authentication == nil || as.Spec.Authentication.Type != Oidc {
		// In this case we do not need anything in the secret - we just return an empty one
		return secret
	}

	pathPrefix := as.ingressPathPrefix()
	sessionURL := as.GetURL()
	pathPrefixURL := url.URL{Host: sessionURL.Host, Path: pathPrefix, Scheme: sessionURL.Scheme}
	cookieSecret := make([]byte, 32)
	_, err := rand.Read(cookieSecret)
	if err != nil {
		// NOTE: Read cannot panic except for on legacy Linux systems
		// See: https://pkg.go.dev/crypto/rand#Read
		panic(err)
	}
	oldConfigLines := []string{
		"session_cookie_minimal = true",
		"skip_provider_button = true",
		fmt.Sprintf("redirect_url = \"%s\"", pathPrefixURL.JoinPath("oauth2/callback").String()),
		fmt.Sprintf("cookie_path = \"%s\"", pathPrefix),
		fmt.Sprintf("proxy_prefix = \"%soauth2\"", pathPrefix),
		"authenticated_emails_file = \"/authorized_emails\"",
		fmt.Sprintf("cookie_secret = \"%s\"", base64.URLEncoding.EncodeToString(cookieSecret)),
	}
	upstreamPort := secondProxyPort
	upstreamConfig := map[string]any{
		"upstreams": []map[string]any{
			{
				"id":                    "amalthea-upstream",
				"path":                  pathPrefix,
				"uri":                   fmt.Sprintf("http://127.0.0.1:%d", upstreamPort),
				"insecureSkipTLSVerify": true,
				"passHostHeader":        true,
				"proxyWebSockets":       true,
			},
		},
	}
	newConfig := map[string]any{
		"providers": []map[string]any{
			{
				"clientID":     "${OIDC_CLIENT_ID}",
				"clientSecret": "${OIDC_CLIENT_SECRET}",
				"id":           "amalthea-oidc",
				"provider":     "oidc",
				"oidcConfig": map[string]any{
					"insecureSkipNonce":            false,
					"issuerURL":                    "${OIDC_ISSUER_URL}",
					"insecureAllowUnverifiedEmail": "${ALLOW_UNVERIFIED_EMAILS}",
					"emailClaim":                   "email",
					"audienceClaims":               []string{"aud"},
				},
			},
		},
		"server": map[string]string{
			"bindAddress": fmt.Sprintf("0.0.0.0:%d", authenticatedPort),
		},
		"upstreamConfig": upstreamConfig,
	}
	newConfigStr, err := yaml.Marshal(newConfig)
	if err != nil {
		panic(err)
	}

	secret.StringData = map[string]string{
		"oauth2-proxy-alpha-config.yaml": string(newConfigStr),
		"oauth2-proxy-config.yaml":       strings.Join(oldConfigLines, "\n"),
	}
	return secret
}

// sessionContainer returns the main session container
func (cr *HpcAmaltheaSession) sessionContainer(volumeMounts []v1.VolumeMount) v1.Container {
	session := cr.Spec.Session
	if cr.Spec.SessionLocation == Local {
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
		// Assign security context
		securityContext := &v1.SecurityContext{
			RunAsNonRoot: ptr.To(true),
			RunAsUser:    ptr.To(session.RunAsUser),
			RunAsGroup:   ptr.To(session.RunAsGroup),
		}
		if session.RunAsUser == 0 {
			securityContext.RunAsNonRoot = ptr.To(false)
		}
		sessionContainer.SecurityContext = securityContext

		return sessionContainer
	}

	// cr.Spec.SessionLocation == Remote
	sessionContainer := v1.Container{
		Image: sidecarsImage,
		Name:  SessionContainerName,
		SecurityContext: &v1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			RunAsNonRoot:             ptr.To(true),
		},
		Args: []string{
			"remote-session-controller",
			"run",
		},
		// TODO: Properly configure env vars
		Env: session.Env,
		// TODO: Set fixed resources here
		Resources:                session.Resources,
		VolumeMounts:             volumeMounts,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: v1.TerminationMessageReadFile,
	}

	sessionContainer.Env = append(sessionContainer.Env, v1.EnvVar{
		Name:  "SERVER_PORT",
		Value: fmt.Sprintf("%d", RemoteSessionControllerPort),
	})

	if session.RemoteSecretRef != nil {
		sessionContainer.EnvFrom = append(sessionContainer.EnvFrom, v1.EnvFromSource{
			// This secret contains the configuration for the remote session controller
			SecretRef: &v1.SecretEnvSource{
				LocalObjectReference: v1.LocalObjectReference{Name: session.RemoteSecretRef.Name},
			},
		})
	}

	sessionContainer.LivenessProbe = &v1.Probe{
		ProbeHandler: v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Port: intstr.FromInt32(RemoteSessionControllerPort),
				Path: "/live",
			},
		},
		PeriodSeconds:       1,
		InitialDelaySeconds: 10,
	}
	sessionContainer.ReadinessProbe = &v1.Probe{
		ProbeHandler: v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Port: intstr.FromInt32(RemoteSessionControllerPort),
				Path: "/ready",
			},
		},
		SuccessThreshold:    5,
		PeriodSeconds:       5,
		InitialDelaySeconds: 10,
	}

	return sessionContainer
}
