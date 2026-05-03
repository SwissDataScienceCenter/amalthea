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

import (
	"context"
	"fmt"
	"log"
	"maps"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	rcloneV2NFSPort           int32  = 65535
	rcloneV2NFSPortName       string = "rcloneNFS"
	rcloneConfigMountPath     string = "/"
	rcloneConfigProjectedPath string = "rclone_config"
	rcloneConfigFullPath      string = "/rclone_config" // combination of the mount path and the projected path
	rcloneConfigVolumeName    string = "rclone-config"
)

func rcloneV2ResourceName(cr *amaltheadevv1alpha1.AmaltheaSession) string {
	return cr.Name + "-nfs"
}

func childLabelsRclone(cr *amaltheadevv1alpha1.AmaltheaSession) map[string]string {
	labels := map[string]string{}
	maps.Copy(labels, cr.Spec.Template.Metadata.Labels)
	selectorLabels := amaltheadevv1alpha1.SelectorLabels(rcloneV2ResourceName(cr))
	conflicts := amaltheadevv1alpha1.FindConflicts(labels, selectorLabels)
	if len(conflicts) > 0 {
		log.Println(
			"Found conflicts in template labels and selector labels for Rclone, the selector labels will take precedence",
			"template labels",
			labels,
			"selector labels",
			selectorLabels,
			"conflicting keys",
			conflicts,
		)
	}
	// NOTE: stuff from selectorLabels will overwrite conflicts in labels (if there are any)
	// This is the desired behaviour, we do not want to overwrite the selector labels.
	maps.Copy(labels, selectorLabels)
	return labels
}

type RcloneV2Resources struct {
	resources []ChildResourcer
}

func (r *RcloneV2Resources) Reconcile(ctx context.Context, clnt client.Client) error {
	for _, res := range r.resources {
		err := res.Reconcile(ctx, clnt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *RcloneV2Resources) StatusCallback(status *amaltheadevv1alpha1.AmaltheaSessionStatus) {
}

func NewRcloneV2Resources(cr *amaltheadevv1alpha1.AmaltheaSession) ChildResourcer {
	_, found := cr.RcloneV2DataSource()
	if !found {
		return &RcloneV2Resources{}
	}
	ss := NewChildResource(
		WithName[*appsv1.StatefulSet](rcloneV2ResourceName(cr)),
		WithNamespace[*appsv1.StatefulSet](cr.Namespace),
		WithMutateFn(func(obj *appsv1.StatefulSet) error {
			desired, err := rcloneV2Statefulset(cr)
			if err != nil {
				return err
			}
			obj.ObjectMeta = desired.ObjectMeta
			obj.Spec = desired.Spec
			return nil
		}),
	)
	service := NewChildResource(
		WithName[*v1.Service](rcloneV2ResourceName(cr)),
		WithNamespace[*v1.Service](cr.Namespace),
		WithMutateFn(func(obj *v1.Service) error {
			desired := rcloneV2Service(cr)
			obj.ObjectMeta = desired.ObjectMeta
			obj.Spec = desired.Spec
			return nil
		}),
	)
	pv := NewChildResource(
		WithName[*v1.PersistentVolume](rcloneV2ResourceName(cr)),
		WithNamespace[*v1.PersistentVolume](cr.Namespace),
		WithMutateFn(func(obj *v1.PersistentVolume) error {
			if len(service.obj.Spec.ClusterIP) == 0 {
				return fmt.Errorf("cannot create volume for rclone dataset because host is not known from the Service")
			}
			desired := rcloneV2Volume(cr, service.obj.Spec.ClusterIP)
			obj.ObjectMeta = desired.ObjectMeta
			obj.Spec = desired.Spec
			return nil
		}),
	)
	pvc := NewChildResource(
		WithName[*v1.PersistentVolumeClaim](rcloneV2ResourceName(cr)),
		WithNamespace[*v1.PersistentVolumeClaim](cr.Namespace),
		WithMutateFn(func(obj *v1.PersistentVolumeClaim) error {
			desired := rcloneV2PVC(cr)
			obj.ObjectMeta = desired.ObjectMeta
			obj.Spec = desired.Spec
			return nil
		}),
	)
	// NOTE: The pv must be reconciled after the service, because the ClusterIP
	// is populated by Kubernetes only after the resource has been created.
	output := RcloneV2Resources{resources: []ChildResourcer{&ss, &service, &pv, &pvc}}
	return &output
}

func rcloneV2DataSource(cr amaltheadevv1alpha1.AmaltheaSession) (amaltheadevv1alpha1.DataSource, bool) {
	for _, ds := range cr.Spec.DataSources {
		if ds.Type == amaltheadevv1alpha1.RcloneV2 {
			return ds, true
		}
	}
	return amaltheadevv1alpha1.DataSource{}, false
}

func rcloneV2Statefulset(cr *amaltheadevv1alpha1.AmaltheaSession) (appsv1.StatefulSet, error) {
	ds, dsFound := cr.RcloneV2DataSource()
	if !dsFound {
		return appsv1.StatefulSet{}, fmt.Errorf("data source of rclonev2 is not defined")
	}
	if ds.SecretRef == nil {
		return appsv1.StatefulSet{}, fmt.Errorf("the secret for the data source has to be defined")
	}
	if len(ds.SecretRef.Key) == 0 {
		return appsv1.StatefulSet{}, fmt.Errorf("the secret for the data source has to have a key")
	}
	args := []string{"serve", "nfs", ds.RcloneRemoteName, fmt.Sprintf("--addr=0.0.0.0:%d", rcloneV2NFSPort), "--config"}
	args = append(args, cr.Spec.DataSourcesConfig.ExtraArgs...)
	volumes := []v1.Volume{}
	volumeMounts := []v1.VolumeMount{}
	volumes = append(
		volumes,
		v1.Volume{
			Name: rcloneConfigVolumeName,
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: ds.SecretRef.Name,
					Items:      []v1.KeyToPath{{Key: ds.SecretRef.Key, Path: rcloneConfigProjectedPath}},
				},
			},
		},
	)
	volumeMounts = append(
		volumeMounts,
		v1.VolumeMount{
			Name:      rcloneConfigVolumeName,
			MountPath: rcloneConfigMountPath,
		},
	)
	args = append(args, "--config", rcloneConfigFullPath)
	pod := v1.PodSpec{
		Volumes:         volumes,
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
				Command:      []string{"rclone"},
				Args:         args,
				VolumeMounts: volumeMounts,
			},
		},
	}
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rcloneV2ResourceName(cr),
			Namespace:   cr.Namespace,
			Labels:      childLabelsRclone(cr),
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			// NOTE: Parallel pod management policy is important
			// If set to default (i.e. not parallel) K8s waits for the pod to become ready before restarting on updates
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Replicas:            ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: amaltheadevv1alpha1.SelectorLabels(rcloneV2ResourceName(cr)),
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      childLabelsRclone(cr),
					Annotations: cr.Spec.Template.Metadata.Annotations,
				},
				Spec: pod,
			},
		},
	}, nil
}

func rcloneV2Service(cr *amaltheadevv1alpha1.AmaltheaSession) v1.Service {
	return v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rcloneV2ResourceName(cr),
			Namespace:   cr.Namespace,
			Labels:      childLabelsRclone(cr),
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{Name: rcloneV2NFSPortName, Port: rcloneV2NFSPort},
			},
			Selector: amaltheadevv1alpha1.SelectorLabels(rcloneV2ResourceName(cr)),
		},
	}
}

func rcloneV2Volume(cr *amaltheadevv1alpha1.AmaltheaSession, server string) v1.PersistentVolume {
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
			Name:        rcloneV2ResourceName(cr),
			Namespace:   cr.Namespace,
			Labels:      childLabelsRclone(cr),
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

func rcloneV2PVC(cr *amaltheadevv1alpha1.AmaltheaSession) v1.PersistentVolumeClaim {
	return v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rcloneV2ResourceName(cr),
			Namespace:   cr.Namespace,
			Labels:      childLabelsRclone(cr),
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: v1.PersistentVolumeClaimSpec{VolumeName: rcloneV2ResourceName(cr)},
	}
}
