package controller

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func stsFailureReason(sts *appsv1.StatefulSet) string {
	return ""
}

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
	if pod.Status.Phase == v1.PodRunning || pod.Status.Phase == v1.PodSucceeded {
		return ""
	}
	// NOTE: The messages from the container statuses list are more descriptive than the conditions
	// so we check these first.
	for _, contStatus := range pod.Status.ContainerStatuses {
		if contStatus.Started != nil && *contStatus.Started && contStatus.Ready {
			continue
		}
		if contStatus.State.Waiting != nil {
			if contStatus.State.Waiting.Reason == "ImagePullBackOff" {
				image := getImageOfContainer(pod, contStatus.Name)
				if image != "" {
					return fmt.Sprintf("the image %s for container %s cannot be found", contStatus.Name, image)
				}
			}
			return fmt.Sprintf("the container %s is failing because %s", contStatus.Name, contStatus.State.Waiting.Message)
		}
		if contStatus.State.Terminated != nil && contStatus.State.Terminated.ExitCode != 0 {
			return fmt.Sprintf("the container %s terminated with an error %s", contStatus.Name, contStatus.State.Terminated.Message)
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
