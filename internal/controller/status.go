package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/SwissDataScienceCenter/amalthea/internal/authproxy"
	v1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// If the cpu usage of the session is less than this then the session is considered idle
var cpuUsageIdlenessThreshold resource.Quantity = *resource.NewMilliQuantity(300, resource.DecimalSI)

// If the last request performed in the user session is older than this the session is considered idle
const lastRequestAgeThreshold time.Duration = time.Minute * 30

// countainerCounts provides from the total and completed/fully running containers in a pod.
// The output is a tuple with the init container counts followed by the regular container counts.
func containerCounts(pod *v1.Pod) (amaltheadevv1alpha1.ContainerCounts, amaltheadevv1alpha1.ContainerCounts) {
	initCounts := amaltheadevv1alpha1.ContainerCounts{}
	counts := amaltheadevv1alpha1.ContainerCounts{}
	if pod == nil {
		return initCounts, counts
	}
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

func getLastRequestTime(cr *amaltheadevv1alpha1.AmaltheaSession) (time.Time, error) {
	url := fmt.Sprintf("http://%s:%d/request_stats", cr.Service().Name, amaltheadevv1alpha1.AuthProxyMetaPort)

	resp, err := http.Get(url)
	if err != nil {
		return time.Time{}, err
	}
	if resp.StatusCode != 200 {
		return time.Time{}, fmt.Errorf("couldn't get last request time due to status: %d", resp.StatusCode)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Log.Error(err, "couldn't close request body")
		}
	}()
	req_stats := authproxy.RequestStatsResponse{}
	err = json.NewDecoder(resp.Body).Decode(&req_stats)
	if err != nil {
		return time.Time{}, err
	}
	return req_stats.LastRequestTime, nil
}

type IdleDecision int

// The values are setup in a way so that max(x, y, z) where x, y, z are
// one of the values below will give the final decision.
const Unknown IdleDecision = 0
const Idle IdleDecision = 1
const NotIdle IdleDecision = 2

func getIdleState(
	ctx context.Context,
	r *AmaltheaSessionReconciler,
	cr *amaltheadevv1alpha1.AmaltheaSession,
) (metav1.Time, bool) {
	log := log.FromContext(ctx)
	if cr == nil {
		return metav1.Time{}, false
	}
	idleSince := cr.Status.IdleSince

	cpuIdle := Unknown
	var cpuUsage *resource.Quantity
	metrics, err := metrics(ctx, r.MetricsClient, cr)
	if err != nil {
		log.Info("Metrics returned error when checking idleness", "error", err)
	} else {
		cpuUsage = metrics.Cpu()
	}
	if cpuUsage != nil {
		if cpuUsage.Cmp(cpuUsageIdlenessThreshold) == -1 {
			cpuIdle = Idle
		} else {
			cpuIdle = NotIdle
		}
	}

	requestIdle := Unknown
	lastRequesTime, err := getLastRequestTime(cr)
	if err != nil {
		log.Info("Request time check returned an error when checking idleness", "error", err)
	} else {
		if time.Since(lastRequesTime) >= lastRequestAgeThreshold {
			requestIdle = Idle
		} else {
			requestIdle = NotIdle
		}
	}

	idle := false
	// If the decision is Unknown or there is at least 1 NotIdle then we keep the final status not idle.
	// If there is 2 Idle or an Unknown and an Idle then the status is Idle.
	decision := max(cpuIdle, requestIdle)
	if decision == Idle {
		idle = true
	}
	if idle && idleSince.IsZero() {
		idleSince = metav1.NewTime(time.Now())
	} else if !idle && !idleSince.IsZero() {
		idleSince = metav1.Time{}
	}

	log.Info("session idle check", "idle", idle, "session", cr.Name, "cpuUsage", cpuUsage, "cpuUsageThreshold", cpuUsageIdlenessThreshold, "lastRequestTime", lastRequesTime, "lastRequestAgeThreshold", lastRequestAgeThreshold)

	return idleSince, idle
}
