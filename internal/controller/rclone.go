package controller

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	childutils "github.com/SwissDataScienceCenter/amalthea/internal/controller/children"
)

// The observed state of the session and of the rclone NFS server that the startup
// and shutdown order is derived from.
type rcloneObservation struct {
	// A pod of the session still mounts the rclone NFS volume
	nfsInUse bool
	// The rclone NFS server has a ready replica
	nfsReady bool
	// The session statefulset is scaled up or the session job is not suspended
	sessionUp bool
}

func observeRclone(ctx context.Context, clnt client.Reader, cr *amaltheadevv1alpha1.AmaltheaSession) (rcloneObservation, error) {
	output := rcloneObservation{}
	if cr.Spec.RcloneDataSource == nil && !controllerutil.ContainsFinalizer(cr, rcloneNFSFinalizer) {
		return output, nil
	}

	pods := corev1.PodList{}
	err := clnt.List(ctx, &pods, client.InNamespace(cr.Namespace), client.MatchingLabels(amaltheadevv1alpha1.SelectorLabels(cr.Name)))
	if err != nil {
		return output, err
	}
	for _, pod := range pods.Items {
		if podMountsClaim(&pod, cr.RcloneV2ResourceName()) {
			output.nfsInUse = true
			break
		}
	}

	nfsSset := appsv1.StatefulSet{}
	err = clnt.Get(ctx, client.ObjectKey{Name: cr.RcloneV2ResourceName(), Namespace: cr.Namespace}, &nfsSset)
	if client.IgnoreNotFound(err) != nil {
		return output, err
	}
	output.nfsReady = nfsSset.Status.ReadyReplicas > 0

	switch cr.Spec.SessionType {
	case amaltheadevv1alpha1.SessionTypeNonInteractive:
		job := batchv1.Job{}
		err = clnt.Get(ctx, client.ObjectKey{Name: cr.JobName(), Namespace: cr.Namespace}, &job)
		if client.IgnoreNotFound(err) != nil {
			return output, err
		}
		output.sessionUp = job.Spec.Suspend == nil || !*job.Spec.Suspend
	default:
		sset := appsv1.StatefulSet{}
		err = clnt.Get(ctx, client.ObjectKey{Name: cr.Name, Namespace: cr.Namespace}, &sset)
		if client.IgnoreNotFound(err) != nil {
			return output, err
		}
		output.sessionUp = err == nil && (sset.Spec.Replicas == nil || *sset.Spec.Replicas > 0)
	}
	return output, nil
}

// Pods that have completed are not considered, the kubelet unmounts their volumes.
func podMountsClaim(pod *corev1.Pod, claimName string) bool {
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false
	}
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == claimName {
			return true
		}
	}
	return false
}

// rclonePlan derives the desired state of the NFS server and whether the session is allowed to run.
// The NFS server is kept running for as long as a session pod mounts it, regardless of why the
// session is shutting down (hibernation, deletion or removal of the rclone section from the spec).
// The session is only started once the NFS server is ready, but a running session is never stopped
// because the NFS server becomes unready.
func rclonePlan(cr *amaltheadevv1alpha1.AmaltheaSession, obs rcloneObservation) (childutils.RcloneV2Plan, bool) {
	deleting := cr.GetDeletionTimestamp() != nil
	wanted := cr.Spec.RcloneDataSource != nil && !deleting
	plan := childutils.RcloneV2Plan{
		Manage: cr.Spec.RcloneDataSource != nil || controllerutil.ContainsFinalizer(cr, rcloneNFSFinalizer),
		Exist:  wanted || obs.nfsInUse,
		Run:    (wanted && !cr.Spec.Hibernated) || obs.nfsInUse,
	}
	sessionMayRun := !cr.Spec.Hibernated && !deleting &&
		(cr.Spec.RcloneDataSource == nil || obs.nfsReady || obs.sessionUp)
	return plan, sessionMayRun
}

const rcloneTransitionRequeue = 5 * time.Second

// The session is waiting for the NFS server to become ready, or the NFS server
// is kept up only because the session pod that is hibernating still mounts it.
func rcloneTransitioning(cr *amaltheadevv1alpha1.AmaltheaSession, obs rcloneObservation, sessionMayRun bool) bool {
	waitingForNFS := !sessionMayRun && !cr.Spec.Hibernated && cr.GetDeletionTimestamp() == nil
	drainingNFS := obs.nfsInUse && cr.Spec.Hibernated
	return waitingForNFS || drainingNFS
}

// Keep the session scaled down without hibernating it
func holdSession(c *ChildResources) {
	if c.StatefulSet.Desired != nil {
		c.StatefulSet.Desired.Spec.Replicas = ptr.To(int32(0))
	}
	if c.Job.Desired != nil {
		c.Job.Desired.Spec.Suspend = ptr.To(true)
	}
}

// Delete the session workload so that the NFS server can be shut down once its pods are gone
func deleteSessionWorkload(ctx context.Context, clnt client.Client, cr *amaltheadevv1alpha1.AmaltheaSession) error {
	propagation := &client.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationForeground)}
	sset := appsv1.StatefulSet{}
	sset.Name, sset.Namespace = cr.Name, cr.Namespace
	if err := clnt.Delete(ctx, &sset, propagation); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	job := batchv1.Job{}
	job.Name, job.Namespace = cr.JobName(), cr.Namespace
	if err := clnt.Delete(ctx, &job, propagation); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
