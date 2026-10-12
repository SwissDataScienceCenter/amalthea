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
	"sigs.k8s.io/controller-runtime/pkg/log"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	childutils "github.com/SwissDataScienceCenter/amalthea/internal/controller/children"
)

const RcloneStartThreshold time.Duration = 10 * time.Minute

// The observed state of the session and of the rclone NFS server that the startup
// and shutdown order is derived from.
type rcloneObservation struct {
	// A pod of the session still mounts the rclone NFS volume
	podsMountNFSVolume bool
	// The rclone NFS server has a ready replica
	nfsServerReady bool
	// The session statefulset is scaled up or the session job is not suspended
	sessionScaledUp bool
	// The NFS server pod exists - regardless of what its state or phase is
	nfsPodExists bool
}

func observeRclone(ctx context.Context, clnt client.Reader, cr *amaltheadevv1alpha1.AmaltheaSession) (rcloneObservation, error) {
	output := rcloneObservation{}
	if cr.Spec.RcloneDataSource == nil && !controllerutil.ContainsFinalizer(cr, childutils.RcloneNFSFinalizer) {
		return output, nil
	}

	pods := corev1.PodList{}
	err := clnt.List(ctx, &pods, client.InNamespace(cr.Namespace), client.MatchingLabels(amaltheadevv1alpha1.SelectorLabels(cr.Name)))
	if err != nil {
		return output, err
	}
	for _, pod := range pods.Items {
		if podMountsClaim(&pod, cr.RcloneV2ResourceName()) {
			output.podsMountNFSVolume = true
			break
		}
	}

	nfsSset := appsv1.StatefulSet{}
	err = clnt.Get(ctx, client.ObjectKey{Name: cr.RcloneV2ResourceName(), Namespace: cr.Namespace}, &nfsSset)
	if client.IgnoreNotFound(err) != nil {
		return output, err
	}
	output.nfsServerReady = nfsSset.Status.ReadyReplicas > 0

	nfsPod := corev1.Pod{}
	err = clnt.Get(ctx, client.ObjectKey{Name: cr.RcloneV2ResourceName() + "-0", Namespace: cr.Namespace}, &nfsPod)
	if client.IgnoreNotFound(err) != nil {
		return output, err
	}
	output.nfsPodExists = err == nil

	switch cr.Spec.SessionType {
	case amaltheadevv1alpha1.SessionTypeNonInteractive:
		job := batchv1.Job{}
		err = clnt.Get(ctx, client.ObjectKey{Name: cr.JobName(), Namespace: cr.Namespace}, &job)
		if client.IgnoreNotFound(err) != nil {
			return output, err
		}
		// If the job is not found, err != nil, without the err condition a missing job will produce sessionScaledUp = true
		output.sessionScaledUp = err == nil && (job.Spec.Suspend == nil || !*job.Spec.Suspend)
	default:
		sset := appsv1.StatefulSet{}
		err = clnt.Get(ctx, client.ObjectKey{Name: cr.Name, Namespace: cr.Namespace}, &sset)
		if client.IgnoreNotFound(err) != nil {
			return output, err
		}
		output.sessionScaledUp = sset.Spec.Replicas != nil && *sset.Spec.Replicas > 0
	}
	return output, nil
}

// Completed pods that are not deleted are not considered, the kubelet unmounts their volumes.
// A deleted pod can still have its volumes mounted after reaching a terminal phase, the kubelet
// only removes the pod once its volumes are cleaned up.
func podMountsClaim(pod *corev1.Pod, claimName string) bool {
	if pod.DeletionTimestamp == nil && (pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed) {
		return false
	}
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == claimName {
			return true
		}
	}
	return false
}

// rcloneDesiredState derives the desired state of the NFS server and whether the session is allowed to run.
// The NFS server is kept running for as long as a session pod mounts it, regardless of why the
// session is shutting down (hibernation, deletion or removal of the rclone section from the spec).
// The session is only started once the NFS server is ready, but a running session is never stopped
// because the NFS server becomes unready.
func rcloneDesiredState(ctx context.Context, cr *amaltheadevv1alpha1.AmaltheaSession, obs rcloneObservation) (nfsState childutils.RcloneNFSDesiredState, sessionMayRun bool) {
	deleting := cr.GetDeletionTimestamp() != nil
	hibernated := cr.Spec.Hibernated
	nfsInUse := obs.podsMountNFSVolume
	nfsConfigured := cr.Spec.RcloneDataSource != nil || controllerutil.ContainsFinalizer(cr, childutils.RcloneNFSFinalizer)
	// NOTE: nfsWanted implies nfsConfigured
	nfsWanted := cr.Spec.RcloneDataSource != nil && !deleting
	// NOTE: The cases below should be kept mutually exclusive, so their order does not matter and the intent is clear.
	switch {
	case !nfsConfigured:
		// There is no Rclone NFS server at all
		nfsState = childutils.RcloneNFSIgnore
	case nfsWanted && !hibernated:
		nfsState = childutils.RcloneNFSRun
	case nfsWanted && hibernated && !nfsInUse:
		nfsState = childutils.RcloneNFSHibernate
	case nfsConfigured && nfsInUse && (!nfsWanted || hibernated):
		nfsState = childutils.RcloneNFSDrain
	case nfsConfigured && !nfsInUse && !nfsWanted:
		nfsState = childutils.RcloneNFSDelete
	default:
		// Unreachable: the cases above cover every combination. Ignore is the safest fallback
		// because it neither deletes nor scales down anything.
		log.FromContext(ctx).Error(
			nil,
			"Could not determine the desired state of the rclone NFS server, leaving it untouched",
			"nfsConfigured", nfsConfigured,
			"nfsWanted", nfsWanted,
			"hibernated", hibernated,
			"inUse", nfsInUse,
		)
		nfsState = childutils.RcloneNFSIgnore
	}
	sessionMayRun = !cr.Spec.Hibernated && !deleting &&
		(cr.Spec.RcloneDataSource == nil || obs.nfsServerReady || obs.sessionScaledUp)
	return nfsState, sessionMayRun
}

const rcloneTransitionRequeue = 5 * time.Second

// The session is waiting for the NFS server to become ready, or the NFS server
// is kept up only because the session pod that is hibernating still mounts it.
func rcloneTransitioning(cr *amaltheadevv1alpha1.AmaltheaSession, obs rcloneObservation, sessionMayRun bool) bool {
	waitingForNFS := !sessionMayRun && !cr.Spec.Hibernated && cr.GetDeletionTimestamp() == nil
	drainingNFS := obs.podsMountNFSVolume && cr.Spec.Hibernated
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

type rcloneStatus struct {
	// The NFS server is stuck in starting mode for a long time.
	Stuck     bool
	Condition amaltheadevv1alpha1.AmaltheaSessionCondition
}

func getRcloneStatus(cr *amaltheadevv1alpha1.AmaltheaSession, obs rcloneObservation, desiredState childutils.RcloneNFSDesiredState) rcloneStatus {
	newCond := func(status metav1.ConditionStatus, reason, msg string) amaltheadevv1alpha1.AmaltheaSessionCondition {
		return amaltheadevv1alpha1.AmaltheaSessionCondition{
			Type: amaltheadevv1alpha1.AmaltheaSessionRcloneReady, Status: status, Reason: reason, Message: msg,
		}
	}

	var condition amaltheadevv1alpha1.AmaltheaSessionCondition
	switch desiredState {
	case childutils.RcloneNFSIgnore:
		condition = newCond(metav1.ConditionTrue, amaltheadevv1alpha1.RcloneReasonNotRequired, "Rclone is not needed at all.")
	case childutils.RcloneNFSDelete:
		condition = newCond(metav1.ConditionFalse, amaltheadevv1alpha1.RcloneReasonShuttingDown, "Starting to shut down the Rclone server.")
	case childutils.RcloneNFSHibernate:
		if obs.nfsPodExists {
			condition = newCond(metav1.ConditionFalse, amaltheadevv1alpha1.RcloneReasonShuttingDown, "Starting to shut down the Rclone server for hibernation.")
		} else {
			condition = newCond(metav1.ConditionFalse, amaltheadevv1alpha1.RcloneReasonHibernated, "The Rclone server is hibernated")
		}
	case childutils.RcloneNFSDrain:
		condition = newCond(metav1.ConditionFalse, amaltheadevv1alpha1.RcloneReasonShuttingDown, "The Rclone server is waiting for the session to stop before it can shut down.")
	case childutils.RcloneNFSRun:
		if obs.nfsPodExists && obs.nfsServerReady {
			condition = newCond(metav1.ConditionTrue, amaltheadevv1alpha1.RcloneReasonReady, "The Rclone server is up and running.")
		} else {
			condition = newCond(metav1.ConditionFalse, amaltheadevv1alpha1.RcloneReasonStarting, "The Rclone server is starting up.")
		}
	}

	stuck := false
	if condition.Reason == amaltheadevv1alpha1.RcloneReasonStarting {
		now := time.Now()
		old := getCondition(cr.Status.Conditions, amaltheadevv1alpha1.AmaltheaSessionRcloneReady)
		stuck = old != nil && old.Reason == amaltheadevv1alpha1.RcloneReasonStarting &&
			now.Sub(old.LastTransitionTime.Time) > RcloneStartThreshold
	}

	return rcloneStatus{Condition: condition, Stuck: stuck}
}
