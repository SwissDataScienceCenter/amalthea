package v1alpha1

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

type manifests struct {
	Containers []v1.Container
	Volumes    []v1.Volume
}

func (as *AmaltheaSession) cloneInit() manifests {
	envVars := []v1.EnvVar{}
	volMounts := []v1.VolumeMount{{Name: sessionVolumeName, MountPath: as.Spec.Session.Storage.MountPath}}
	vols := []v1.Volume{}
	containers := []v1.Container{}

	for irepo, repo := range as.Spec.CodeRepositories {
		args := []string{
			"cloner",
			"clone",
			"--strategy",
			"notifexist",
			"--remote",
			repo.Remote,
			"--path",
			fmt.Sprintf("%s/%s", as.Spec.Session.Storage.MountPath, repo.ClonePath),
		}

		if repo.CloningConfigSecretRef != nil {
			secretVolName := fmt.Sprintf("%sgit-clone-cred-volume-%d", prefix, irepo)
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
			Image:        sidecarsImage,
			VolumeMounts: volMounts,
			WorkingDir:   as.Spec.Session.Storage.MountPath,
			Env:          envVars,
			SecurityContext: &v1.SecurityContext{
				RunAsUser:  &as.Spec.Session.RunAsUser,
				RunAsGroup: &as.Spec.Session.RunAsGroup,
			},
			Args: args,
		})
	}
	return manifests{containers, vols}
}
