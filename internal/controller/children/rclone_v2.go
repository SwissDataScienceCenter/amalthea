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
	"cmp"
	"context"
	"fmt"
	"log"
	"maps"
	"math"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	rcloneV2NFSPort     int32  = 65535
	rcloneV2NFSPortName string = "rclone-nfs"
	// Added on Rclone NFS server resources to indicate what session namespace they belong to and to generate reconcile events.
	RcloneV2LabelSessionNamespaceKey string = "amalthea.dev/sessionNamespace"
	// Added on Rclone NFS server resources to indicate what session they belong to and to generate reconcile events.
	RcloneV2LabelSessionNameKey    string = "amalthea.dev/sessionName"
	RcloneV2LabelKey               string = "app.kubernetes.io/component"
	RcloneV2LabelValue             string = "rclone-v2"
	RcloneV2CombinedSourceName     string = "__combined__"
	RcloneV2SecretsMountPath       string = "/secrets"
	RcloneV2ConfigParentMounthPath string = "/rclone"
	RcloneV2ConfigPath             string = "/rclone/rclone-config" // This and the above parent path have to be compatible
)

func RcloneV2ResourceName(cr *amaltheadevv1alpha1.AmaltheaSession) string {
	return cr.RcloneV2ResourceName()
}

func RcloneV2PVName(cr *amaltheadevv1alpha1.AmaltheaSession) string {
	return cr.RcloneV2PVName()
}

func RcloneV2PVCCacheName(cr *amaltheadevv1alpha1.AmaltheaSession) string {
	return cr.RcloneV2ResourceName() + "-cache"
}

func rcloneV2SelectorLabels(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "AmaltheaSession",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/part-of":    "amaltheasession-operator",
		"app.kubernetes.io/created-by": "controller-manager",
		RcloneV2LabelKey:               RcloneV2LabelValue,
	}
}

func childLabelsRclone(cr *amaltheadevv1alpha1.AmaltheaSession) map[string]string {
	labels := map[string]string{}
	maps.Copy(labels, cr.Spec.Template.Metadata.Labels)
	selectorLabels := rcloneV2SelectorLabels(RcloneV2ResourceName(cr))
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
	// These 2 labels below are needed so that changes in Rclone resources can generate reconcile requests for Amalthea.
	// The reconcile requests for Amalthea contain the session name and namespace.
	labels[RcloneV2LabelSessionNamespaceKey] = cr.Namespace
	labels[RcloneV2LabelSessionNameKey] = cr.Name
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

// RcloneNFSDesiredState is the desired state of the rclone NFS server. It is derived from the session spec and
// from the observed state of the session pods, so that the NFS server always outlives the pods that mount it.
// If the NFS server goes away first the session pod gets stuck in "Terminating" for a long time.
type RcloneNFSDesiredState string

const (
	// The session does not use rclone, nothing is done
	RcloneNFSIgnore RcloneNFSDesiredState = ""
	// The NFS server should be running
	RcloneNFSRun RcloneNFSDesiredState = "run"
	// The NFS server should keep running until the session pod that is shutting down
	// (hibernation, deletion or removal of the rclone section from the spec) stops mounting it
	RcloneNFSDrain RcloneNFSDesiredState = "drain"
	// The NFS server should be scaled down to 0 while its service, PV and PVC are kept
	RcloneNFSHibernate RcloneNFSDesiredState = "hibernate"
	// The rclone resources should be deleted
	RcloneNFSDelete RcloneNFSDesiredState = "delete"
)

func NewRcloneV2Resources(cr *amaltheadevv1alpha1.AmaltheaSession, nfsState RcloneNFSDesiredState) ChildResourcer {
	if nfsState == RcloneNFSIgnore {
		return &RcloneV2Resources{}
	}
	if cr.Spec.RcloneDataSource == nil && nfsState == RcloneNFSDrain {
		// The rclone section was removed from the spec but the NFS volume is still in use.
		// The desired state cannot be generated without the spec, so the resources are left untouched.
		return &RcloneV2Resources{}
	}
	// NOTE: The service, PV and PVC are kept when the session is hibernated, only the statefulset is scaled down.
	// In most clusters the operator will only be allowed to create PVs (not patch or delete them). The PV
	// references the ClusterIP of the service and is pre-bound to the PVC, so neither the service nor
	// the PVC can be recreated without breaking the PV.
	service := NewChildResource(
		(*v1.Service)(nil),
		WithName[*v1.Service](RcloneV2ResourceName(cr)),
		WithNamespace[*v1.Service](cr.Namespace),
		WithMutateFn(func(obj *v1.Service) error {
			if cr.Spec.RcloneDataSource == nil {
				return nil
			}
			desired := rcloneV2Service(cr)
			obj.Labels = CleanWellKnown(obj.Labels, desired.Labels)
			obj.Annotations = CleanWellKnown(obj.Annotations, desired.Annotations)
			obj.Spec.Ports = desired.Spec.Ports
			obj.Spec.Selector = desired.Spec.Selector
			return nil
		}),
		WithDeleteWhen[*v1.Service](func() bool {
			return nfsState == RcloneNFSDelete
		}),
	)
	ss := NewChildResource(
		(*appsv1.StatefulSet)(nil),
		WithName[*appsv1.StatefulSet](RcloneV2ResourceName(cr)),
		WithNamespace[*appsv1.StatefulSet](cr.Namespace),
		WithMutateFn(func(obj *appsv1.StatefulSet) error {
			if cr.Spec.RcloneDataSource == nil {
				return nil
			}
			desired := rcloneV2Statefulset(cr)
			obj.Labels = CleanWellKnown(obj.Labels, desired.Labels)
			obj.Annotations = CleanWellKnown(obj.Annotations, desired.Annotations)
			obj.Spec.Replicas = new(int32(1))
			if nfsState == RcloneNFSHibernate {
				obj.Spec.Replicas = new(int32(0))
			}
			obj.Spec.Selector = desired.Spec.Selector
			obj.Spec.Template = desired.Spec.Template
			obj.Spec.PodManagementPolicy = desired.Spec.PodManagementPolicy
			return nil
		}),
		WithDeleteWhen[*appsv1.StatefulSet](func() bool {
			return nfsState == RcloneNFSDelete
		}),
		WithParent[*appsv1.StatefulSet](service.obj),
	)
	pv := NewChildResource(
		(*v1.PersistentVolume)(nil),
		WithName[*v1.PersistentVolume](RcloneV2PVName(cr)),
		WithMutateFn(func(obj *v1.PersistentVolume) error {
			if cr.Spec.RcloneDataSource == nil {
				return nil
			}
			// NOTE: The PV is write-once, the operator is most likely not allowed to patch PVs.
			// Leaving an existing PV untouched results in no patch request from CreateOrPatch.
			if !obj.CreationTimestamp.IsZero() {
				return nil
			}
			if len(service.obj.Spec.ClusterIP) == 0 {
				return fmt.Errorf("cannot create volume for rclone dataset because host is not known from the Service")
			}
			desired := rcloneV2Volume(cr, service.obj.Spec.ClusterIP)
			obj.Labels = desired.Labels
			obj.Annotations = desired.Annotations
			obj.Spec = desired.Spec
			return nil
		}),
		WithDeleteWhen[*v1.PersistentVolume](func() bool {
			return nfsState == RcloneNFSDelete
		}),
		// In most clusters the operator is not allowed to delete PVs, the PV has to be cleaned up
		// externally. We should not block the cleanup of the rclone resources because of this.
		WithIgnoreForbiddenDelete[*v1.PersistentVolume](),
	)
	pvc := NewChildResource(
		(*v1.PersistentVolumeClaim)(nil),
		WithName[*v1.PersistentVolumeClaim](RcloneV2ResourceName(cr)),
		WithNamespace[*v1.PersistentVolumeClaim](cr.Namespace),
		WithMutateFn(func(obj *v1.PersistentVolumeClaim) error {
			if cr.Spec.RcloneDataSource == nil {
				return nil
			}
			desired := rcloneV2PVC(cr)
			if !obj.CreationTimestamp.IsZero() {
				obj.Labels = CleanWellKnown(obj.Labels, desired.Labels)
				obj.Annotations = CleanWellKnown(obj.Annotations, desired.Annotations)
				return nil
			}
			obj.Labels = CleanWellKnown(obj.Labels, desired.Labels)
			obj.Annotations = CleanWellKnown(obj.Annotations, desired.Annotations)
			obj.Spec = desired.Spec
			return nil
		}),
		WithParent[*v1.PersistentVolumeClaim](service.obj),
		WithDeleteWhen[*v1.PersistentVolumeClaim](func() bool {
			return nfsState == RcloneNFSDelete
		}),
	)
	cachePVC := NewChildResource(
		(*v1.PersistentVolumeClaim)(nil),
		WithName[*v1.PersistentVolumeClaim](RcloneV2PVCCacheName(cr)),
		WithNamespace[*v1.PersistentVolumeClaim](cr.Namespace),
		WithMutateFn(func(obj *v1.PersistentVolumeClaim) error {
			if cr.Spec.RcloneDataSource == nil {
				return nil
			}
			desired := rcloneV2CachePVC(cr)
			if desired == nil {
				return nil
			}
			if !obj.CreationTimestamp.IsZero() {
				obj.Labels = CleanWellKnown(obj.Labels, desired.Labels)
				obj.Annotations = CleanWellKnown(obj.Annotations, desired.Annotations)
				obj.Spec.Resources = desired.Spec.Resources
				return nil
			}
			obj.Labels = CleanWellKnown(obj.Labels, desired.Labels)
			obj.Annotations = CleanWellKnown(obj.Annotations, desired.Annotations)
			obj.Spec = desired.Spec
			return nil
		}),
		WithParent[*v1.PersistentVolumeClaim](service.obj),
		// We dont want to keep the cache and keep paying for storage when the session hibernates.
		WithDeleteWhen[*v1.PersistentVolumeClaim](func() bool {
			return nfsState == RcloneNFSDelete || nfsState == RcloneNFSHibernate || !cr.Spec.RcloneDataSource.GetCache().Persisted
		}),
	)
	// NOTE: The pv must be reconciled after the service, because the ClusterIP
	// is populated by Kubernetes only after the resource has been created.
	output := RcloneV2Resources{resources: []ChildResourcer{&service, &ss, &pv, &pvc, &cachePVC}}
	return &output
}

func rcloneSecretVolumes(cr *amaltheadevv1alpha1.AmaltheaSession) (v1.Volume, v1.VolumeMount) {
	vol := v1.Volume{
		Name: "secrets",
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{SecretName: cr.Spec.RcloneDataSource.Secret.Name},
		},
	}
	mount := v1.VolumeMount{Name: "secrets", MountPath: RcloneV2SecretsMountPath, ReadOnly: true}
	return vol, mount
}

func rcloneV2Statefulset(cr *amaltheadevv1alpha1.AmaltheaSession) appsv1.StatefulSet {
	if cr.Spec.RcloneDataSource == nil {
		return appsv1.StatefulSet{}
	}
	secVol, secVolMnt := rcloneSecretVolumes(cr)
	volumes := []v1.Volume{}
	volumeMounts := []v1.VolumeMount{}
	volumes = append(volumes, secVol)
	volumeMounts = append(volumeMounts, secVolMnt)
	args := []string{
		"serve",
		"nfs",
		cr.Spec.RcloneDataSource.RemoteName + ":",
		fmt.Sprintf("--addr=0.0.0.0:%d", rcloneV2NFSPort),
	}
	cache := cr.Spec.RcloneDataSource.GetCache()
	args = append(args, cr.Spec.RcloneDataSource.ExtraArgs...)
	args = append(args, "--config", RcloneV2SecretsMountPath+"/"+cr.Spec.RcloneDataSource.Secret.Key)
	args = append(args, "--vfs-cache-mode", string(cache.CacheMode))
	args = append(args, fmt.Sprintf("--vfs-cache-max-size=%dB", int(math.Floor(0.95*float64(cache.Size.Value())))))
	args = append(args, "--rc")
	args = append(args, "--log-level", "INFO")
	if cache.Persisted {
		volumes = append(
			volumes,
			v1.Volume{
				Name:         "cache",
				VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: RcloneV2PVCCacheName(cr)}},
			},
		)
		volumeMounts = append(volumeMounts, v1.VolumeMount{Name: "cache", MountPath: "/cache"})
		// Without the nfs-cache args below the user session pod will have to be restarted in order to recover the mounts
		// when the nfs server pod restarts.
		args = append(args, "--nfs-cache-type=disk", "--nfs-cache-dir=/cache/handles")
		args = append(args, "--cache-dir=/cache/vfs")
	}
	preStopCommand := `
echo "preStop: waiting for rclone uploads to finish..."

while true; do
	QUEUE=$(rclone rc vfs/queue)
	STATS=$(rclone rc core/stats)

	if echo "$STATS" | grep -q '"transferring"' || echo "$QUEUE" | grep -q "uploading"; then
		echo "preStop: still busy, waiting..."
		sleep 2
	else
		echo "preStop: idle, proceeding with shutdown"
		break
	fi
done`
	pod := v1.PodSpec{
		Volumes:                       volumes,
		SecurityContext:               &v1.PodSecurityContext{RunAsNonRoot: ptr.To(true)},
		TerminationGracePeriodSeconds: ptr.To(int64(300)),
		AutomountServiceAccountToken:  ptr.To(false),
		Containers: []v1.Container{
			{
				Name:      "rclone",
				Image:     cr.Spec.RcloneDataSource.Image,
				Resources: *cmp.Or(cr.Spec.RcloneDataSource.Resources, ptr.To(v1.ResourceRequirements{})),
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
				Lifecycle:    &v1.Lifecycle{PreStop: &v1.LifecycleHandler{Exec: &v1.ExecAction{Command: []string{"/bin/sh", "-c", preStopCommand}}}},
			},
		},
	}
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        RcloneV2ResourceName(cr),
			Namespace:   cr.Namespace,
			Labels:      childLabelsRclone(cr),
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			// NOTE: Parallel pod management policy is important
			// If set to default (i.e. not parallel) K8s waits for the pod to become ready before restarting on updates
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: rcloneV2SelectorLabels(RcloneV2ResourceName(cr)),
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      childLabelsRclone(cr),
					Annotations: cr.Spec.Template.Metadata.Annotations,
				},
				Spec: pod,
			},
		},
	}
}

func rcloneV2Service(cr *amaltheadevv1alpha1.AmaltheaSession) v1.Service {
	return v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        RcloneV2ResourceName(cr),
			Namespace:   cr.Namespace,
			Labels:      childLabelsRclone(cr),
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{Name: rcloneV2NFSPortName, Port: rcloneV2NFSPort},
				// NOTE: Port 111 is used by the kubelet to send the unmount command via rpc
				// Without this unmounting and shutting down the pods is a few minutes slower.
				{Name: "rpcbind", Port: 111, TargetPort: intstr.FromInt32(111)},
			},
			Selector: rcloneV2SelectorLabels(RcloneV2ResourceName(cr)),
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
		"proto=tcp",
		"mountproto=tcp",
		"mountvers=3",
		// Timeout is in deciseconds - tenths of second, 600 = 60s
		"timeo=600",
		"retrans=3",
		"hard",
		"nolock",
	}
	labels := childLabelsRclone(cr)
	mountOptions = append(mountOptions, cr.Spec.RcloneDataSource.MountOptions...)
	return v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        RcloneV2PVName(cr),
			Labels:      labels,
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: v1.PersistentVolumeSpec{
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Capacity:         v1.ResourceList{"storage": resource.MustParse("1Gi")},
			MountOptions:     mountOptions,
			StorageClassName: "",
			// NOTE: Pre-bind the PV to the session PVC so that no other PVC can claim it and the
			// operator never has to patch the PV for binding. The UID is filled in by Kubernetes.
			ClaimRef: &v1.ObjectReference{
				APIVersion: "v1",
				Kind:       "PersistentVolumeClaim",
				Namespace:  cr.Namespace,
				Name:       RcloneV2ResourceName(cr),
			},
			// NOTE: There is no deleter for NFS volumes, so with "Delete" the PV would just end up "Failed"
			// after the PVC is removed. The PV is deleted by the operator when the session is deleted
			// (if it has the permissions to do so).
			PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimRetain,
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
			Name:        RcloneV2ResourceName(cr),
			Namespace:   cr.Namespace,
			Labels:      childLabelsRclone(cr),
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			VolumeName:       RcloneV2PVName(cr),
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources:        v1.VolumeResourceRequirements{Requests: v1.ResourceList{"storage": resource.MustParse("1Gi")}},
			StorageClassName: ptr.To(""),
		},
	}
}

// NOTE: Whether the cache PVC should be deleted (e.g. on hibernation) is decided by the RcloneNFSDesiredState
func rcloneV2CachePVC(cr *amaltheadevv1alpha1.AmaltheaSession) *v1.PersistentVolumeClaim {
	if cr.Spec.RcloneDataSource == nil {
		return nil
	}
	if !cr.Spec.RcloneDataSource.GetCache().Persisted {
		return nil
	}
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        RcloneV2PVCCacheName(cr),
			Namespace:   cr.Namespace,
			Labels:      childLabelsRclone(cr),
			Annotations: cr.Spec.Template.Metadata.Annotations,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources:        v1.VolumeResourceRequirements{Requests: v1.ResourceList{"storage": cr.Spec.RcloneDataSource.GetCache().Size}},
			StorageClassName: cr.Spec.RcloneDataSource.GetCache().StorageClassName,
		},
	}
}
