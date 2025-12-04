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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
)

func updateHibernationState(ctx context.Context, r *AmaltheaSessionReconciler, amaltheasession *amaltheadevv1alpha1.AmaltheaSession) error {
	culling := amaltheasession.Spec.Culling
	status := amaltheasession.Status
	log := log.FromContext(ctx)
	if !amaltheasession.Spec.Hibernated {
		pod, err := amaltheasession.GetPod(ctx, r.Client)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		// then check whether we want to scale down the StatefulSet and do it
		creationTimestamp := pod.GetCreationTimestamp()
		hibernationDate := calculateHibernationDate(log, creationTimestamp, status, culling)

		if status.State != amaltheadevv1alpha1.Hibernated && needsScaleDown(hibernationDate) {
			amaltheasession.Spec.Hibernated = true
			err = r.Update(ctx, amaltheasession)
			if err != nil {
				return err
			}
			log.Info("statefulSet scaled down")
		} else {
			if !hibernationDate.IsZero() && !hibernationDate.Time.Equal(status.WillHibernateAt.Time) {
				log.Info(fmt.Sprint("ENTER UPDATE: time-equal:", hibernationDate.Time, " vs. ", status.WillHibernateAt.Time, hibernationDate.Time.Equal(status.WillHibernateAt.Time)))
				amaltheasession.Status.WillHibernateAt = hibernationDate
				err = r.Client.Status().Update(ctx, amaltheasession)
				if err != nil {
					log.Error(err, "OH NO UPDATE FAILED")
					return err
				}
			}
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
	return !hibernationDate.IsZero() && (hibernationDate.Time.Equal(now) || hibernationDate.Time.Before(now))
}

func needsScaleDownOld(log logr.Logger, creationTimestamp metav1.Time, status amaltheadevv1alpha1.AmaltheaSessionStatus, culling amaltheadevv1alpha1.Culling) bool {
	if status.State == amaltheadevv1alpha1.Hibernated {
		return false
	}
	now := time.Now()
	age := now.Sub(creationTimestamp.Time)
	starting := status.State == amaltheadevv1alpha1.NotReady
	idleSince := status.IdleSince
	lastIdleSince := idleSince

	// if we have a last-interaction, it determines the start of the idle
	// time that is used to decide upon hibernation
	lastInteraction := culling.LastInteraction
	if !lastInteraction.IsZero() && !idleSince.IsZero() && lastInteraction.After(idleSince.Time) {
		lastIdleSince = lastInteraction
	}
	idleDuration := now.Sub(lastIdleSince.Time)
	failingSince := status.FailingSince
	failedDuration := now.Sub(failingSince.Time)
	zero := time.Duration(0)
	maxAge := culling.MaxAge.Duration
	maxStartingDuration := culling.MaxStartingDuration.Duration
	maxIdleDuration := culling.MaxIdleDuration.Duration
	maxFailedDuration := culling.MaxFailedDuration.Duration

	decideViaLog := ""

	decideViaMaxAge := maxAge > zero && age > maxAge
	if maxAge > zero {
		decideViaLog += fmt.Sprint("age>maxAge: ", age, ">", maxAge, "=", decideViaMaxAge)
	}

	decideViaMaxStartingDuration := maxStartingDuration > zero && age > maxStartingDuration && starting
	if maxStartingDuration > zero && starting {
		decideViaLog += fmt.Sprint(" age>maxStartingDuration: ", age, ">", maxStartingDuration, "=", decideViaMaxStartingDuration)
	}

	decideViaIdleSince := !lastIdleSince.IsZero() && maxIdleDuration > zero && idleDuration > maxIdleDuration
	if !lastIdleSince.IsZero() && maxIdleDuration > zero {
		decideViaLog += fmt.Sprint(" idleDuration>maxIdleDuration: ", idleDuration, ">", maxIdleDuration, "=", decideViaIdleSince)
	}

	decideViaFailingSince := !failingSince.IsZero() && maxFailedDuration > zero && failedDuration > maxFailedDuration
	if !failingSince.IsZero() && maxFailedDuration > zero {
		decideViaLog += fmt.Sprint(" failedDuration>maxFailedDuration: ", failedDuration, ">", maxFailedDuration, "=", decideViaFailingSince)
	}

	result := decideViaMaxAge || decideViaMaxStartingDuration || decideViaIdleSince || decideViaFailingSince

	log.Info("needs scaledown",
		"Result", result,
		"Age", age,
		"idleSince", idleSince,
		"lastInteraction", lastInteraction,
		"idleDuration", idleDuration,
		"decideLog", decideViaLog,
	)

	return result
}
