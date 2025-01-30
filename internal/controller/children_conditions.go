package controller

import (
	"fmt"
	"slices"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func handleBadWaitingState(status v1.ContainerStatus) string {
	waitingState := status.State.Waiting

	switch waitingState.Reason {
	// NOTE: both reasons concerns the same issue but happen at different time of the pod life cycle
	case "ErrImagePull", "ImagePullBackOff":
		return fmt.Sprintf("failure to retrieve image for container %s: %s", status.Name, waitingState.Message)
	case "CrashLoopBackOff":
		return fmt.Sprintf("the command in container %s is crashing, this can occur when you run out of disk space, this is the best reason for the crash we could extract: %q", status.Name, waitingState.Message)
	}

	if waitingState.Message != "" {
		return fmt.Sprintf("the container %s is failing because %s", status.Name, waitingState.Message)
	}

	return fmt.Sprintf("the container %s is failing because %s", status.Name, waitingState.Reason)
}

func podFailureReason(pod *v1.Pod) string {
	if pod == nil {
		return ""
	}
	if pod.GetDeletionTimestamp() != nil {
		return ""
	}
	// NOTE: Checking the pod phase is not useful because it will still say "Running" when a container
	// fails to start for example when the command is referencing an executable that does not exist.
	// NOTE: The messages from the container statuses list are more descriptive than the conditions
	// so we check these first.
	valid_waiting_reasons := []string{
		// NOTE: This is the default status the pod starts with when it is starting up when it has no init containers
		"ContainerCreating",
		// NOTE: This is the default status the pod starts with when it is starting up and it has init containers
		"PodInitializing",
	}

	allStatuses := []v1.ContainerStatus{}
	allStatuses = append(allStatuses, pod.Status.InitContainerStatuses...)
	allStatuses = append(allStatuses, pod.Status.ContainerStatuses...)
	for _, contStatus := range allStatuses {
		switch {
		case contStatus.Started != nil && *contStatus.Started && contStatus.Ready:
			continue
		case contStatus.State.Waiting != nil:
			if slices.Contains(valid_waiting_reasons, contStatus.State.Waiting.Reason) {
				continue
			}
			return handleBadWaitingState(contStatus)
		case contStatus.State.Terminated != nil && contStatus.State.Terminated.ExitCode != 0:
			if contStatus.State.Terminated.Message != "" {
				return fmt.Sprintf("the container %s terminated with an error %s", contStatus.Name, contStatus.State.Terminated.Message)
			}
			return fmt.Sprintf("the command in container %s terminated with an error exit code, this could be because you are running out of disk space", contStatus.Name)
		}
	}
	for _, condition := range pod.Status.Conditions {
		switch condType := condition.Type; condType {
		case v1.DisruptionTarget:
			if condition.Status != v1.ConditionTrue {
				continue
			}
			if condition.Message != "" {
				return condition.Message
			}
			return condition.Reason
		case v1.ContainersReady, v1.PodInitialized, v1.PodReady, v1.PodScheduled:
			continue
		}
	}
	return ""
}

func ingressFailureReason(ing *networkingv1.Ingress) string {
	if ing == nil {
		return ""
	}
	if ing.GetDeletionTimestamp() != nil {
		return ""
	}
	for _, ingStatus := range ing.Status.LoadBalancer.Ingress {
		for _, portStatus := range ingStatus.Ports {
			if portStatus.Error != nil {
				return *portStatus.Error
			}
		}
	}
	return ""
}

func serviceFailureReason(svc *v1.Service) string {
	if svc == nil {
		return ""
	}
	if svc.GetDeletionTimestamp() != nil {
		return ""
	}
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		for _, portStatus := range ing.Ports {
			if portStatus.Error != nil {
				return *portStatus.Error
			}
		}
	}
	return ""
}

func pvcFailureReason(pvc *v1.PersistentVolumeClaim) string {
	if pvc == nil {
		return ""
	}
	if pvc.GetDeletionTimestamp() != nil {
		return ""
	}
	if pvc.Status.Phase == v1.ClaimBound {
		return ""
	}
	for resName, resStatus := range pvc.Status.AllocatedResourceStatuses {
		switch resStatus {
		case v1.PersistentVolumeClaimNodeResizeInfeasible, v1.PersistentVolumeClaimControllerResizeInfeasible:
			return fmt.Sprintf("the PVC %q failed with status %q for resource %s", pvc.GetName(), resStatus, resName)
		default:
			continue
		}
	}
	for _, condition := range pvc.Status.Conditions {
		switch condType := condition.Type; condType {
		case v1.PersistentVolumeClaimResizing:
			if condition.Status == v1.ConditionFalse {
				return condition.Message
			}
		case v1.PersistentVolumeClaimFileSystemResizePending:
			if condition.Status == v1.ConditionFalse {
				return condition.Message
			}
		}
	}
	return ""
}
