package v1alpha1

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const authproxyImage string = "harbor.renkulab.io/bitnami-mirror/oauth2-proxy:7.6.0"

func (as *AmaltheaSession) auth() (manifests, error) {
	output := manifests{}
	volumeMounts := []v1.VolumeMount{}
	auth := as.Spec.Authentication

	if auth == nil || !auth.Enabled {
		return output, nil
	}
	if (auth.Type == OauthProxy || auth.Type == Token) && len(auth.SecretRef.Key) == 0 {
		// NOTE: For oidc we need the whole secret - we dont need a specific key of the secret
		return output, fmt.Errorf("the authentication secret key has to be defined when using %s authentication", auth.Type)
	}
	if len(auth.ExtraVolumeMounts) > 0 {
		volumeMounts = auth.ExtraVolumeMounts
	}

	var authContainer v1.Container

	switch auth.Type {
	case OauthProxy:
		volName := fmt.Sprintf("%sproxy-configuration-secret", prefix)
		output.Volumes = append(output.Volumes, v1.Volume{
			Name: volName,
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: auth.SecretRef.Name,
					Optional:   ptr.To(false),
				},
			},
		})
		probeHandler := v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Path: "/ping",
				Port: intstr.FromInt32(authenticatedPort),
			},
		}
		oauth2ProxyContainer := v1.Container{
			Image: authproxyImage,
			Name:  "oauth2-proxy",
			SecurityContext: &v1.SecurityContext{
				AllowPrivilegeEscalation: ptr.To(false),
				RunAsNonRoot:             ptr.To(true),
			},
			Args: []string{
				fmt.Sprintf("--upstream=%s", fmt.Sprintf("http://127.0.0.1:%d", secondProxyPort)),
				fmt.Sprintf("--http-address=:%d", authenticatedPort),
				"--silence-ping-logging",
				"--config=/etc/oauth2-proxy/" + auth.SecretRef.Key,
			},
			VolumeMounts: append(
				[]v1.VolumeMount{
					{
						Name:      volName,
						MountPath: "/etc/oauth2-proxy",
					},
				},
				volumeMounts...,
			),
			ReadinessProbe: &v1.Probe{ProbeHandler: probeHandler},
			LivenessProbe:  &v1.Probe{ProbeHandler: probeHandler},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					"memory": resource.MustParse("16Mi"),
					"cpu":    resource.MustParse("20m"),
				},
				Limits: v1.ResourceList{
					"memory": resource.MustParse("32Mi"),
					// NOTE: Cpu limit not set on purpose
					// Without cpu limit if there is spare you can go over the request
					// If there is no spare cpu then all things get throttled relative to their request
					// With cpu limits you get throttled when you go over the request always, even with spare capacity
				},
			},
		}
		authContainer = as.get_rewrite_authn_proxy(secondProxyPort, AuthProxyMetaPort, as.Spec.Session.Port)

		output.Containers = append(output.Containers, oauth2ProxyContainer)
	case Token:
		volName := fmt.Sprintf("%sproxy-configuration-secret", prefix)
		output.Volumes = append(output.Volumes, v1.Volume{
			Name: volName,
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: auth.SecretRef.Name,
					Optional:   ptr.To(false),
				},
			},
		})
		authContainer = as.get_rewrite_authn_proxy(authenticatedPort, AuthProxyMetaPort, as.Spec.Session.Port)
		authContainer.Args = []string{
			"proxy",
			"serve",
			"--config",
			fmt.Sprintf("/etc/authproxy/%s", auth.SecretRef.Key),
		}
		authContainer.VolumeMounts = append(
			[]v1.VolumeMount{
				{
					Name:      volName,
					MountPath: "/etc/authproxy",
				},
			},
			volumeMounts...,
		)
	case Oidc:
		volNameFixedConfig := fmt.Sprintf("%s-fixed-proxy-configuration-secret", prefix)
		volNameAuthorizedEmails := fmt.Sprintf("%s-authorized-emails-secret", prefix)
		fixedConfigVol := v1.Volume{
			Name: volNameFixedConfig,
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: as.Name,
					Optional:   ptr.To(false),
				},
			},
		}
		authorizedEmailsVol := v1.Volume{
			Name: volNameAuthorizedEmails,
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: auth.SecretRef.Name,
					Optional:   ptr.To(false),
				},
			},
		}
		output.Volumes = append(output.Volumes, fixedConfigVol, authorizedEmailsVol)
		probeHandler := v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Path: "/ping",
				Port: intstr.FromInt32(authenticatedPort),
			},
		}
		oauth2ProxyContainer := v1.Container{
			Image: authproxyImage,
			Name:  "oauth2-proxy",
			SecurityContext: &v1.SecurityContext{
				AllowPrivilegeEscalation: ptr.To(false),
				RunAsNonRoot:             ptr.To(true),
			},
			Args: []string{
				"--silence-ping-logging",
				"--alpha-config=/etc/oauth2-proxy/oauth2-proxy-alpha-config.yaml",
				"--config=/etc/oauth2-proxy/oauth2-proxy-config.yaml",
			},
			EnvFrom: []v1.EnvFromSource{
				{
					// This secret contains the client ID, secret and issuer URL for oidc
					SecretRef: &v1.SecretEnvSource{
						LocalObjectReference: v1.LocalObjectReference{Name: auth.SecretRef.Name},
					},
				},
			},
			VolumeMounts: append(
				[]v1.VolumeMount{
					{
						Name:      volNameFixedConfig,
						MountPath: "/etc/oauth2-proxy",
					},
					{
						Name:      volNameAuthorizedEmails,
						MountPath: "/authorized_emails",
						SubPath:   "AUTHORIZED_EMAILS",
					},
				},
				volumeMounts...,
			),
			ReadinessProbe: &v1.Probe{ProbeHandler: probeHandler},
			LivenessProbe:  &v1.Probe{ProbeHandler: probeHandler},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					"memory": resource.MustParse("16Mi"),
					"cpu":    resource.MustParse("20m"),
				},
				Limits: v1.ResourceList{
					"memory": resource.MustParse("32Mi"),
					// NOTE: Cpu limit not set on purpose
					// Without cpu limit if there is spare you can go over the request
					// If there is no spare cpu then all things get throttled relative to their request
					// With cpu limits you get throttled when you go over the request always, even with spare capacity
				},
			},
		}
		authContainer = as.get_rewrite_authn_proxy(secondProxyPort, AuthProxyMetaPort, as.Spec.Session.Port)
		output.Containers = append(output.Containers, oauth2ProxyContainer)
	default:
		return output, fmt.Errorf("unexpected authentication type %v when trying to template authentication containers", auth.Type)
	}
	output.Containers = append(output.Containers, authContainer)
	return output, nil
}

func (as *AmaltheaSession) get_rewrite_authn_proxy(listenPort int32, metaListenPort int32, remotePort int32) v1.Container {
	probeHandler := v1.ProbeHandler{
		HTTPGet: &v1.HTTPGetAction{
			Path: "/__amalthea__/health",
			Port: intstr.FromInt32(listenPort),
		},
	}
	authContainer := v1.Container{
		Image: sidecarsImage,
		Name:  "authproxy",
		SecurityContext: &v1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			RunAsNonRoot:             ptr.To(true),
			RunAsUser:                ptr.To(int64(1000)),
			RunAsGroup:               ptr.To(int64(1000)),
		},
		Args: []string{
			"proxy",
			"serve",
			"--verbose",
		},
		Env: []v1.EnvVar{
			{Name: "AUTHPROXY_PORT", Value: fmt.Sprintf("%d", listenPort)},
			{Name: "AUTHPROXY_META_PORT", Value: fmt.Sprintf("%d", metaListenPort)},
			// NOTE: The url for the remote has to not have a path at all, if it does, then the path
			// in the url is appended to any path that is already there when the request comes in.
			{Name: "AUTHPROXY_REMOTE", Value: fmt.Sprintf("http://127.0.0.1:%d", remotePort)},
		},
		ReadinessProbe: &v1.Probe{ProbeHandler: probeHandler},
		LivenessProbe:  &v1.Probe{ProbeHandler: probeHandler},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				"memory": resource.MustParse("16Mi"),
				"cpu":    resource.MustParse("20m"),
			},
			Limits: v1.ResourceList{
				"memory": resource.MustParse("32Mi"),
				// NOTE: Cpu limit not set on purpose
				// Without cpu limit if there is spare you can go over the request
				// If there is no spare cpu then all things get throttled relative to their request
				// With cpu limits you get throttled when you go over the request always, even with spare capacity
			},
		},
	}
	if as.Spec.Session.StripURLPath {
		authContainer.Env = append(authContainer.Env, v1.EnvVar{
			Name: "AUTHPROXY_STRIP_PATH_PREFIX", Value: as.urlPath(),
		})
	}
	return authContainer
}
