/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package children

func (cr *AmaltheaSession) RcloneV2ResourceName() string {
	return cr.Name + "-nfs"
}

const rcloneV2NFSPort int32 = 65535
const rcloneV2NFSPortName string = "rcloneNFS"

func (cr *AmaltheaSession) RcloneV2DataSource() (DataSource, bool) {
	for _, ds := range cr.Spec.DataSources {
		if ds.Type == RcloneV2 {
			return ds, true
		}
	}
	return DataSource{}, false
}

func (cr *AmaltheaSession) RcloneV2Statefulset() (appsv1.StatefulSet, bool) {
	ds, dsFound := cr.RcloneV2DataSource()
	if !dsFound {
		return appsv1.StatefulSet{}, false
	}
	args := []string{"serve", "nfs", ds.RcloneRemoteName, fmt.Sprintf("--addr=0.0.0.0:%d", rcloneV2NFSPort)}
	args = append(args, cr.Spec.DataSourcesConfig.ExtraArgs...)
	pod := v1.PodSpec{
		SecurityContext: &v1.PodSecurityContext{RunAsNonRoot: ptr.To(true)},
		Containers: []v1.Container{
			{
				Name:      "rclone",
				Image:     cr.Spec.DataSourcesConfig.Image,
				Resources: cr.Spec.DataSourcesConfig.Resources,
				SecurityContext: &v1.SecurityContext{
					Privileged:               ptr.To(false),
					AllowPrivilegeEscalation: ptr.To(false),
					RunAsNonRoot:             ptr.To(true),
					Capabilities:             &v1.Capabilities{Drop: []v1.Capability{"ALL"}},
				},
				Ports: []v1.ContainerPort{
					{Name: rcloneV2NFSPortName, ContainerPort: rcloneV2NFSPort},
				},
				ReadinessProbe: &v1.Probe{
					ProbeHandler: v1.ProbeHandler{
						TCPSocket: &v1.TCPSocketAction{Port: intstr.FromString(rcloneV2NFSPortName)},
					},
				},
				LivenessProbe: &v1.Probe{
					ProbeHandler: v1.ProbeHandler{
						TCPSocket: &v1.TCPSocketAction{Port: intstr.FromString(rcloneV2NFSPortName)},
					},
				},
				Command: []string{"rclone"},
				Args:    args,
			},
		},
	}
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.RcloneV2ResourceName(),
			Namespace:   cr.Namespace,
			Labels:      cr.childLabelsRclone(),
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			// NOTE: Parallel pod management policy is important
			// If set to default (i.e. not parallel) K8s waits for the pod to become ready before restarting on updates
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Replicas:            ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels(cr.RcloneV2ResourceName()),
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      cr.childLabelsRclone(),
					Annotations: cr.Spec.Template.Metadata.Annotations,
				},
				Spec: pod,
			},
		},
	}, true
}

func (cr *AmaltheaSession) RcloneV2Service() v1.Service {
	return v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.RcloneV2ResourceName(),
			Namespace:   cr.Namespace,
			Labels:      cr.childLabelsRclone(),
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{Name: rcloneV2NFSPortName, Port: rcloneV2NFSPort},
			},
			Selector: selectorLabels(cr.RcloneV2ResourceName()),
		},
	}
}

func (cr *AmaltheaSession) RcloneV2Volume(server string) v1.PersistentVolume {
	// TODO: check which args take precedence for rclone if duplicated
	mountOptions := []string{
		// NOTE: Rclone has not implemented NFS v4 and some/most clusters will default to NFS v4 for NFS PVs.
		"vers=3",
		fmt.Sprintf("port=%d", rcloneV2NFSPort),
		fmt.Sprintf("mountport=%d", rcloneV2NFSPort),
		"tcp",
		"timeo=600",
		"retrans=3",
	}
	mountOptions = append(mountOptions, cr.Spec.DataSourcesConfig.MountOptions...)
	return v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.RcloneV2ResourceName(),
			Namespace:   cr.Namespace,
			Labels:      cr.childLabelsRclone(),
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: v1.PersistentVolumeSpec{
			MountOptions: mountOptions,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				NFS: &v1.NFSVolumeSource{
					Server: server,
					Path:   "/",
				},
			},
		},
	}
}

func (cr *AmaltheaSession) RcloneV2PVC(server string) v1.PersistentVolumeClaim {
	return v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.RcloneV2ResourceName(),
			Namespace:   cr.Namespace,
			Labels:      cr.childLabelsRclone(),
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: v1.PersistentVolumeClaimSpec{VolumeName: cr.RcloneV2ResourceName()},
	}
}
