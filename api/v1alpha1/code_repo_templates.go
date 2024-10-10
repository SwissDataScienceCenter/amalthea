package v1alpha1

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	st "github.com/SwissDataScienceCenter/amalthea/internal/sidecartemplates"
	v1 "k8s.io/api/core/v1"
)

type manifests struct {
	Containers []v1.Container
	Volumes    []v1.Volume
}

// NOTE: changing the code of the template for an existing version after it has been released
// may result in the restart of all old sessions that are running when the new version of amalthea
// is deployed.
var initCloneTemplate st.VersionedTemplates[AmaltheaSession, manifests] = st.NewVersionedTemplates(
	st.VersionedTemplate[AmaltheaSession, manifests]{
		MinInclusiveVersion: *semver.MustParse("0.0.1"),
		TemplateFunc: func(session *AmaltheaSession) manifests {
			envVars := []v1.EnvVar{}
			volMounts := []v1.VolumeMount{{Name: sessionVolumeName, MountPath: session.Spec.Session.Storage.MountPath}}
			vols := []v1.Volume{}
			containers := []v1.Container{}

			for irepo, repo := range session.Spec.CodeRepositories {
				args := []string{
					"cloner",
					"clone",
					"--strategy",
					"notifexist",
					"--remote",
					repo.Remote,
					"--path",
					fmt.Sprintf("%s/%s", session.Spec.Session.Storage.MountPath, repo.ClonePath),
				}

				if repo.CloningConfigSecretRef != nil {
					secretVolName := fmt.Sprintf("git-clone-cred-volume-%d", irepo)
					secretMountPath := "/git-clone-secrets"
					secretFilePath := fmt.Sprintf("%s/%s", secretMountPath, repo.CloningConfigSecretRef.Key)
					vols = append(
						vols,
						v1.Volume{
							Name: secretVolName,
							VolumeSource: v1.VolumeSource{
								Secret: &v1.SecretVolumeSource{SecretName: repo.CloningConfigSecretRef.Name},
							},
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
					Name:         gitCloneContainerName,
					Image:        session.Spec.Sidecars.Image,
					VolumeMounts: volMounts,
					WorkingDir:   session.Spec.Session.Storage.MountPath,
					Env:          envVars,
					SecurityContext: &v1.SecurityContext{
						RunAsUser:  &session.Spec.Session.RunAsUser,
						RunAsGroup: &session.Spec.Session.RunAsGroup,
					},
					Args: args,
				})
			}
			return manifests{Containers: containers, Volumes: vols}
		},
	},
)
