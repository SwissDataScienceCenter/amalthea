package controller

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func getImageOfContainer(pod *v1.Pod, containerName string) string {
	for _, cont := range pod.Spec.InitContainers {
		if cont.Name == containerName {
			return cont.Image
		}
	}
	for _, cont := range pod.Spec.Containers {
		if cont.Name == containerName {
			return cont.Image
		}
	}
	return ""
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
	allStatuses := []v1.ContainerStatus{}
	allStatuses = append(allStatuses, pod.Status.InitContainerStatuses...)
	allStatuses = append(allStatuses, pod.Status.ContainerStatuses...)
	for _, contStatus := range allStatuses {
		switch {
		case contStatus.Started != nil && *contStatus.Started && contStatus.Ready:
			continue
		case contStatus.State.Waiting != nil && contStatus.State.Waiting.Reason == "ImagePullBackOff":
			image := getImageOfContainer(pod, contStatus.Name)
			if image != "" {
				return fmt.Sprintf("the image %s for container %s cannot be found", image, contStatus.Name)
			}
			return fmt.Sprintf("the image for container %s cannot be found", contStatus.Name)
		case contStatus.State.Waiting != nil && contStatus.State.Waiting.Reason == "CrashLoopBackOff":
			return fmt.Sprintf("the command in container %s is crashing, this can occur when you run out of disk space, this is the best reason for the crash we could extract: %q", contStatus.Name, contStatus.State.Waiting.Message)
		case contStatus.State.Waiting != nil && contStatus.State.Waiting.Message != "":
			return fmt.Sprintf("the container %s is failing because %s", contStatus.Name, contStatus.State.Waiting.Message)
		case contStatus.State.Waiting != nil:
			return fmt.Sprintf("the container %s is failing because %s", contStatus.Name, contStatus.State.Waiting.Reason)
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
		case v1.PersistentVolumeClaimNodeResizeFailed, v1.PersistentVolumeClaimControllerResizeFailed:
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
