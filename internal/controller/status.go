package controller

import (
	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

func getCounts(statuses []v1.ContainerStatus) amaltheadevv1alpha1.ContainerCounts {
	total := 0
	ready := 0
	for _, status := range statuses {
		total += 1
		terminatedSuccessfully := status.State.Terminated != nil && status.State.Terminated.ExitCode == 0
		runningSuccessfully := status.State.Running != nil && status.Started != nil && *status.Started
		containerReady := status.Ready && (terminatedSuccessfully || runningSuccessfully)
		if containerReady {
			ready += 1
		}
	}
	return amaltheadevv1alpha1.ContainerCounts{Ready: ready, Total: total}
}

func isIdle() bool {
	return false
}

func isFailing() bool {
	return false
}
