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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
)

func updateHibernationState(ctx context.Context, r *AmaltheaSessionReconciler, amaltheasession *amaltheadevv1alpha1.AmaltheaSession) error {
	status := amaltheasession.Status
	log := log.FromContext(ctx)
	if !amaltheasession.Spec.Hibernated {
		hibernationDate := status.WillHibernateAt
		if status.State != amaltheadevv1alpha1.Hibernated && needsScaleDown(hibernationDate) {
			amaltheasession.Spec.Hibernated = true
			err := r.Update(ctx, amaltheasession)
			if err != nil {
				return err
			}
			log.Info("statefulSet scaled down")
		}
	}
	return nil
}

func hibernationDateByMaxAge(creationTimestamp metav1.Time, culling amaltheadevv1alpha1.Culling) metav1.Time {
	zero := time.Duration(0)
	maxAge := culling.MaxAge.Duration
	if maxAge > zero {
		return metav1.NewTime(creationTimestamp.Time.Add(maxAge))
	}
	return metav1.Time{}
}

func hibernationDateByStartingDuration(creationTimestamp metav1.Time, status amaltheadevv1alpha1.AmaltheaSessionStatus, culling amaltheadevv1alpha1.Culling) metav1.Time {
	zero := time.Duration(0)
	maxStartingDuration := culling.MaxStartingDuration.Duration
	starting := status.State == amaltheadevv1alpha1.NotReady
	if maxStartingDuration > zero && starting {
		return metav1.NewTime(creationTimestamp.Add(maxStartingDuration))
	}
	return metav1.Time{}
}

func hibernationDateByFailingSince(status amaltheadevv1alpha1.AmaltheaSessionStatus, culling amaltheadevv1alpha1.Culling) metav1.Time {
	zero := time.Duration(0)
	failingSince := status.FailingSince
	maxFailedDuration := culling.MaxFailedDuration.Duration
	if !failingSince.IsZero() && maxFailedDuration > zero {
		return metav1.NewTime(failingSince.Add(maxFailedDuration))
	}
	return metav1.Time{}
}

func hibernationDateByIdleSince(status amaltheadevv1alpha1.AmaltheaSessionStatus, culling amaltheadevv1alpha1.Culling) metav1.Time {
	zero := time.Duration(0)
	idleSince := status.IdleSince
	lastIdleSince := idleSince
	maxIdleDuration := culling.MaxIdleDuration.Duration
	// if we have a last-interaction, it determines the start of the idle
	// time that is used to decide upon hibernation
	lastInteraction := culling.LastInteraction
	if !lastInteraction.IsZero() && !idleSince.IsZero() && lastInteraction.After(idleSince.Time) {
		lastIdleSince = lastInteraction
	}
	if !lastIdleSince.IsZero() && maxIdleDuration > zero {
		return metav1.NewTime(lastIdleSince.Time.Add(maxIdleDuration))
	}
	return metav1.Time{}
}

func calculateHibernationDate(log logr.Logger, creationTimestamp metav1.Time, status amaltheadevv1alpha1.AmaltheaSessionStatus, culling amaltheadevv1alpha1.Culling) metav1.Time {
	type Pair struct {
		Name string
		Date metav1.Time
	}

	hibernationDates := []Pair{
		{Name: "MaxAge", Date: hibernationDateByMaxAge(creationTimestamp, culling)},
		{Name: "StartDuration", Date: hibernationDateByStartingDuration(creationTimestamp, status, culling)},
		{Name: "FailingDuration", Date: hibernationDateByFailingSince(status, culling)},
		{Name: "IdleDuration", Date: hibernationDateByIdleSince(status, culling)},
	}

	result := Pair{}
	logMsg := ""
	for _, hd := range hibernationDates {
		if result.Date.IsZero() || (!hd.Date.IsZero() && hd.Date.Time.Before(result.Date.Time)) {
			result = hd
		}
		if !hd.Date.IsZero() {
			logMsg += fmt.Sprint(" ", hd.Name, ":", hd.Date)
		}
	}
	if result.Date.IsZero() {
		logMsg += " => NONE"
	} else {
		logMsg += fmt.Sprint(" => ", result.Name, ":", result.Date)
	}
	log.Info("hibernationDate", "DecisionLog", strings.TrimSpace(logMsg))
	return result.Date
}

func needsScaleDown(hibernationDate metav1.Time) bool {
	now := time.Now()
	return !hibernationDate.IsZero() && hibernationDate.Time.Before(now)
}
