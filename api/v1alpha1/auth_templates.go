package v1alpha1

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const authproxyImage string = "bitnami/oauth2-proxy:7.6.0"

func (session *AmaltheaSession) auth() manifests {
	output := manifests{}
	volumeMounts := []v1.VolumeMount{}
	auth := session.Spec.Authentication

	if auth == nil || !auth.Enabled {
		return output
	}
	if len(auth.ExtraVolumeMounts) > 0 {
		volumeMounts = auth.ExtraVolumeMounts
	}
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

	if auth.Type == Oidc {
		sessionURL := session.sessionLocalhostURL().String()
		if !strings.HasSuffix(sessionURL, "/") {
			// NOTE: If the url does not end with "/" then the oauth2proxy proxies only the exact path
			// and does not proxy subpaths
			sessionURL += "/"
		}
		authContainer := v1.Container{
			Image: authproxyImage,
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
						Name:      volName,
						MountPath: "/etc/oauth2-proxy",
					},
				},
				volumeMounts...,
			),
		}

		output.Containers = append(output.Containers, authContainer)
	} else if auth.Type == Token {
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
				"--config",
				fmt.Sprintf("/etc/authproxy/%s", auth.SecretRef.Key),
			},
			Env: []v1.EnvVar{
				{Name: "AUTHPROXY_PORT", Value: fmt.Sprintf("%d", authProxyPort)},
				// NOTE: The url for the remote has to not have a path at all, if it does, then the path
				// in the url is appended to any path that is already there when the request comes in.
				{Name: "AUTHPROXY_REMOTE", Value: fmt.Sprintf("http://127.0.0.1:%d", session.Spec.Session.Port)},
			},
			VolumeMounts: append(
				[]v1.VolumeMount{
					{
						Name:      volName,
						MountPath: "/etc/authproxy",
					},
				},
				volumeMounts...,
			),
		}

		output.Containers = append(output.Containers, authContainer)
	}
	return output
}