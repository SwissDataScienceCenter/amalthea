package controller

import (
	"context"
	"fmt"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var cpuUsageIdlenessThreshold resource.Quantity = *resource.NewMilliQuantity(300, resource.DecimalSI)

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

func metrics(ctx context.Context, clnt metricsv1beta1.PodMetricsesGetter, cr *amaltheadevv1alpha1.AmaltheaSession) (v1.ResourceList, error) {
	podName := fmt.Sprintf("%s-0", cr.Name)
	podMetricses, err := clnt.PodMetricses(cr.Namespace).List(
		ctx,
		metav1.SingleObject(
			metav1.ObjectMeta{Name: podName},
		),
	)

	if err != nil {
		return nil, err
	}

	if len(podMetricses.Items) == 0 {
		return nil, fmt.Errorf("pod %s not found", podName)
	}

	podMetrics := podMetricses.Items[0]

	for _, container := range podMetrics.Containers {
		if container.Name == amaltheadevv1alpha1.SessionContainerName {
			return container.Usage, nil
		}
	}

	return nil, fmt.Errorf("could not find the metrics for the session container %s", amaltheadevv1alpha1.SessionContainerName)
}

func isIdle(ctx context.Context, clnt metricsv1beta1.PodMetricsesGetter, cr *amaltheadevv1alpha1.AmaltheaSession) bool {
	log := log.FromContext(ctx)
	if cr == nil {
		return false
	}

	metrics, err := metrics(ctx, clnt, cr)
	if err != nil {
		log.Info("Metrics returned error", "error", err)
		return false
	}

	cpuUsage := metrics.Cpu()
	if cpuUsage != nil && cpuUsage.Cmp(cpuUsageIdlenessThreshold) == -1 {
		if cr.Status.IdleSince.IsZero() {
			log.Info(
				"the session was found to be idle",
				"cpu usage milicores",
				cpuUsage.MilliValue(),
				"idle threshold milicores",
				cpuUsageIdlenessThreshold.MilliValue(),
			)
		}
		return true
	}

	return false
}
