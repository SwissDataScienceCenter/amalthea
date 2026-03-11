/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	crtlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
)

var (
	// amaltheaSessionState tracks the current state of AmaltheaSessions
	amaltheaSessionState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amalthea_session_state",
			Help: "Current state of an AmaltheaSession (1 when in that state, 0 otherwise)",
		},
		[]string{"namespace", "name", "state"},
	)

	// amaltheaSessionIdle tracks whether a session is idle
	amaltheaSessionIdle = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amalthea_session_idle",
			Help: "Whether an AmaltheaSession is idle (1 for idle, 0 for active)",
		},
		[]string{"namespace", "name"},
	)

	// amaltheaSessionContainerReady tracks ready containers in a session
	amaltheaSessionContainerReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amalthea_session_container_ready",
			Help: "Number of ready containers in an AmaltheaSession",
		},
		[]string{"namespace", "name"},
	)

	// amaltheaSessionContainerTotal tracks total containers in a session
	amaltheaSessionContainerTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amalthea_session_container_total",
			Help: "Total number of containers in an AmaltheaSession",
		},
		[]string{"namespace", "name"},
	)

	// amaltheaSessionIdleDuration tracks how long a session has been idle
	amaltheaSessionIdleDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amalthea_session_idle_duration_seconds",
			Help: "Duration in seconds that an AmaltheaSession has been idle",
		},
		[]string{"namespace", "name"},
	)

	// amaltheaSessionFailingDuration tracks how long a session has been failing
	amaltheaSessionFailingDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amalthea_session_failing_duration_seconds",
			Help: "Duration in seconds that an AmaltheaSession has been in a failed state",
		},
		[]string{"namespace", "name"},
	)

	// amaltheaSessionHibernatedDuration tracks how long a session has been hibernated
	amaltheaSessionHibernatedDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amalthea_session_hibernated_duration_seconds",
			Help: "Duration in seconds that an AmaltheaSession has been hibernated",
		},
		[]string{"namespace", "name"},
	)

	// amaltheaSessionFailedSchedulingDuration tracks how long a session has failed scheduling
	amaltheaSessionFailedSchedulingDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amalthea_session_failed_scheduling_duration_seconds",
			Help: "Duration in seconds that an AmaltheaSession has failed to schedule",
		},
		[]string{"namespace", "name"},
	)

	// amaltheaSessionsTotal counts total sessions by state
	amaltheaSessionsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amalthea_sessions_total",
			Help: "Total number of AmaltheaSessions by state",
		},
		[]string{"state"},
	)

	// amaltheaSessionWillHibernateAt tracks when a session will be hibernated
	amaltheaSessionWillHibernateAt = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amalthea_session_will_hibernate_at",
			Help: "Unix timestamp when an AmaltheaSession will be hibernated",
		},
		[]string{"namespace", "name"},
	)
)

func init() {
	// Register all metrics with the prometheus registry
	crtlmetrics.Registry.MustRegister(
		amaltheaSessionState,
		amaltheaSessionIdle,
		amaltheaSessionContainerReady,
		amaltheaSessionContainerTotal,
		amaltheaSessionIdleDuration,
		amaltheaSessionFailingDuration,
		amaltheaSessionHibernatedDuration,
		amaltheaSessionFailedSchedulingDuration,
		amaltheaSessionsTotal,
		amaltheaSessionWillHibernateAt,
	)
}

// RecordAmaltheaSessionMetrics records Prometheus metrics for an AmaltheaSession
func RecordAmaltheaSessionMetrics(session *amaltheadevv1alpha1.AmaltheaSession) {
	namespace := session.GetNamespace()
	name := session.GetName()
	status := session.Status

	// Record session state (all possible states are set to 0, only current state is 1)
	for _, state := range []amaltheadevv1alpha1.State{
		amaltheadevv1alpha1.NotReady,
		amaltheadevv1alpha1.Running,
		amaltheadevv1alpha1.RunningDegraded,
		amaltheadevv1alpha1.Failed,
		amaltheadevv1alpha1.Hibernated,
	} {
		stateValue := 0.0
		if status.State == state {
			stateValue = 1.0
		}
		amaltheaSessionState.WithLabelValues(namespace, name, string(state)).Set(stateValue)
	}

	// Record idle status
	idleValue := 0.0
	if status.Idle {
		idleValue = 1.0
	}
	amaltheaSessionIdle.WithLabelValues(namespace, name).Set(idleValue)

	// Record container counts
	amaltheaSessionContainerReady.WithLabelValues(namespace, name).Set(float64(status.ContainerCounts.Ready))
	amaltheaSessionContainerTotal.WithLabelValues(namespace, name).Set(float64(status.ContainerCounts.Total))

	// Record idle duration
	if !status.IdleSince.IsZero() {
		idleDuration := time.Since(status.IdleSince.Time).Seconds()
		amaltheaSessionIdleDuration.WithLabelValues(namespace, name).Set(idleDuration)
	} else {
		amaltheaSessionIdleDuration.WithLabelValues(namespace, name).Set(0)
	}

	// Record failing duration
	if !status.FailingSince.IsZero() {
		failingDuration := time.Since(status.FailingSince.Time).Seconds()
		amaltheaSessionFailingDuration.WithLabelValues(namespace, name).Set(failingDuration)
	} else {
		amaltheaSessionFailingDuration.WithLabelValues(namespace, name).Set(0)
	}

	// Record hibernated duration
	if !status.HibernatedSince.IsZero() {
		hibernatedDuration := time.Since(status.HibernatedSince.Time).Seconds()
		amaltheaSessionHibernatedDuration.WithLabelValues(namespace, name).Set(hibernatedDuration)
	} else {
		amaltheaSessionHibernatedDuration.WithLabelValues(namespace, name).Set(0)
	}

	// Record failed scheduling duration
	if !status.FailedSchedulingSince.IsZero() {
		failedSchedulingDuration := time.Since(status.FailedSchedulingSince.Time).Seconds()
		amaltheaSessionFailedSchedulingDuration.WithLabelValues(namespace, name).Set(failedSchedulingDuration)
	} else {
		amaltheaSessionFailedSchedulingDuration.WithLabelValues(namespace, name).Set(0)
	}

	// Record when session will hibernate
	if !status.WillHibernateAt.IsZero() {
		amaltheaSessionWillHibernateAt.WithLabelValues(namespace, name).Set(float64(status.WillHibernateAt.Unix()))
	} else {
		amaltheaSessionWillHibernateAt.WithLabelValues(namespace, name).Set(0)
	}
}

// RemoveAmaltheaSessionMetrics cleans up metrics for a deleted AmaltheaSession
func RemoveAmaltheaSessionMetrics(session *amaltheadevv1alpha1.AmaltheaSession) {
	namespace := session.GetNamespace()
	name := session.GetName()

	// Remove all metric series for this session
	amaltheaSessionState.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace,
		"name":      name,
	})
	amaltheaSessionIdle.DeleteLabelValues(namespace, name)
	amaltheaSessionContainerReady.DeleteLabelValues(namespace, name)
	amaltheaSessionContainerTotal.DeleteLabelValues(namespace, name)
	amaltheaSessionIdleDuration.DeleteLabelValues(namespace, name)
	amaltheaSessionFailingDuration.DeleteLabelValues(namespace, name)
	amaltheaSessionHibernatedDuration.DeleteLabelValues(namespace, name)
	amaltheaSessionFailedSchedulingDuration.DeleteLabelValues(namespace, name)
	amaltheaSessionWillHibernateAt.DeleteLabelValues(namespace, name)
}

// UpdateSessionStateCounter updates the total sessions counter based on all sessions
func UpdateSessionStateCounter(sessionsByState map[amaltheadevv1alpha1.State]int) {
	for _, state := range []amaltheadevv1alpha1.State{
		amaltheadevv1alpha1.NotReady,
		amaltheadevv1alpha1.Running,
		amaltheadevv1alpha1.RunningDegraded,
		amaltheadevv1alpha1.Failed,
		amaltheadevv1alpha1.Hibernated,
	} {
		count := sessionsByState[state]
		amaltheaSessionsTotal.WithLabelValues(string(state)).Set(float64(count))
	}
}
