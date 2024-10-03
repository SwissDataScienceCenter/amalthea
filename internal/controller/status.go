package controller

import (
	"context"
	"encoding/json"
	"fmt"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"

	v1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

type MetricsSummary struct {
	CPUMilicores int64
	MemoryBytes  int64
}

type PodMetrics struct {
	Kind       string            `json:"kind"`
	APIVersion string            `json:"apiVersion"`
	Metadata   metav1.ObjectMeta `json:"metadata"`

	// The following fields define time interval from which metrics were
	// collected in the following format [Timestamp-Window, Timestamp].
	Timestamp metav1.Time     `json:"timestamp"`
	Window    metav1.Duration `json:"window"`

	// Metrics for all containers are collected within the same time window.
	Containers []ContainerMetrics `json:"containers"`
}

type ContainerMetrics struct {
	// Container name corresponding to the one from v1.Pod.Spec.Containers.
	Name string `json:"name"`
	// The memory usage is the memory working set.
	Usage v1.ResourceList `json:"usage"`
}

func metrics(ctx context.Context, clnt *kubernetes.Clientset, cr *amaltheadevv1alpha1.AmaltheaSession) (MetricsSummary, error) {
	podName := fmt.Sprintf("%s-0", cr.Name)
	url := "apis/metrics.k8s.io/v1beta1/namespaces/" + cr.Namespace + "/pods/" + podName

	data, err := clnt.RESTClient().Get().AbsPath(url).DoRaw(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// The pod might not yet be available or was deleted
			return MetricsSummary{-1, -1}, nil
		}
		return MetricsSummary{}, err
	}
	podMetrics := PodMetrics{}
	err = json.Unmarshal(data, &podMetrics)
	if err != nil {
		return MetricsSummary{}, err
	}

	totalCpu := resource.NewMilliQuantity(0, resource.DecimalSI)
	totalMemory := resource.NewMilliQuantity(0, resource.BinarySI)

	for _, container := range podMetrics.Containers {
		totalCpu.Add(*container.Usage.Cpu())
		totalMemory.Add(*container.Usage.Memory())
	}

	return MetricsSummary{totalCpu.MilliValue(), totalMemory.MilliValue()}, nil
}

func isIdle(ctx context.Context, clnt *kubernetes.Clientset, cr *amaltheadevv1alpha1.AmaltheaSession) bool {
	log := log.FromContext(ctx)
	if cr == nil {
		return false
	}

	metrics, err := metrics(ctx, clnt, cr)
	if err != nil {
		log.Info("Metrics returned error", "error", err)
		return false
	}

	if metrics.CPUMilicores == 0 {
		log.Info("Is doing nothing")
		return true
	}

	return false
}
