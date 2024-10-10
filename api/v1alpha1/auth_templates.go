package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	st "github.com/SwissDataScienceCenter/amalthea/internal/sidecartemplates"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// NOTE: changing the code of the template for an existing version after it has been released
// may result in the restart of all old sessions that are running when the new version of amalthea
// is deployed.
var authTemplate st.VersionedTemplates[AmaltheaSession, manifests] = st.NewVersionedTemplates(
	st.VersionedTemplate[AmaltheaSession, manifests]{
		MinInclusiveVersion: *semver.MustParse("0.0.1"),
		TemplateFunc: func(session *AmaltheaSession) manifests {
			output := manifests{}
			volumeMounts := []v1.VolumeMount{}
			auth := session.Spec.Authentication

			if auth == nil || !auth.Enabled {
				return output
			}
			if len(auth.ExtraVolumeMounts) > 0 {
				volumeMounts = auth.ExtraVolumeMounts
			}
			output.Volumes = append(output.Volumes, v1.Volume{
				Name: "proxy-configuration-secret",
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
					Image: "bitnami/oauth2-proxy:7.6.0",
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
						volumeMounts...,
					),
				}

				output.Containers = append(output.Containers, authContainer)
			} else if auth.Type == Token {
				authContainer := v1.Container{
					Image: session.Spec.Sidecars.Image,
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
								Name:      "proxy-configuration-secret",
								MountPath: "/etc/authproxy",
							},
						},
						volumeMounts...,
					),
				}

				output.Containers = append(output.Containers, authContainer)
			}
			return output
		},
	},
)
