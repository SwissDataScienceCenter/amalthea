package controller

import (
	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

// How many times should a pod restart before we consider it Failed. This is required because if we take
// a value that is too low it is possible that the pod will "heal itself" after a few restarts. If the
// value is too high then the user will wait for a long time before they see their pod is failing.
const restartThreshold int32 = 3

// countainerCounts provides from the total and completed/fully running containers in a pod.
// The output is a tuple with the init container counts followed by the regular container counts.
func containerCounts(pod *v1.Pod) (amaltheadevv1alpha1.ContainerCounts, amaltheadevv1alpha1.ContainerCounts) {
	initCounts := amaltheadevv1alpha1.ContainerCounts{}
	counts := amaltheadevv1alpha1.ContainerCounts{}
	for _, container := range pod.Status.InitContainerStatuses {
		containerCompleted := container.State.Terminated != nil && container.State.Terminated.ExitCode == 0 && container.State.Terminated.Reason == "Completed"
		initCounts.Total += 1
		if containerCompleted {
			initCounts.Ready += 1
		}
	}
	for _, container := range pod.Status.ContainerStatuses {
		containerRunning := container.Ready && container.State.Running != nil
		containerCompleted := container.State.Terminated != nil && container.State.Terminated.ExitCode == 0 && container.State.Terminated.Reason == "Completed"
		counts.Total += 1
		if containerRunning || containerCompleted {
			counts.Ready += 1
		}
	}
	return initCounts, counts
}

func podIsReady(pod *v1.Pod) bool {
	if pod == nil || pod.GetDeletionTimestamp() != nil {
		// A missing pod or a pod being deleted is not considered ready
		return false
	}
	phaseOk := pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodRunning
	initCounts, counts := containerCounts(pod)
	return initCounts.Ok() && counts.Ok() && phaseOk
}

func podIsFailed(pod *v1.Pod) bool {
	if pod == nil || pod.GetDeletionTimestamp() != nil {
		// A missing pod or a pod being deleted is not considered failed
		return false
	}
	for _, container := range pod.Status.InitContainerStatuses {
		containerFailed := container.State.Waiting != nil && container.RestartCount >= restartThreshold
		if containerFailed {
			return true
		}
	}
	for _, container := range pod.Status.ContainerStatuses {
		containerFailed := container.State.Waiting != nil && container.RestartCount >= restartThreshold
		if containerFailed {
			return true
		}
	}
	return false
}

func isIdle() bool {
	return false
}
