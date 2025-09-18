package controller

import (
	"context"
	"fmt"
	"time"

	v1alpha "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestClient struct {
	listError  *error
	listResult client.ObjectList
}

func arbitraryEvent() v1.Event {
	return v1.Event{
		Reason:  "random reason",
		Message: "random message",
	}
}
func triggeredScaleUpEvent() v1.Event {
	return v1.Event{
		Reason:  "TriggeredScaleUp",
		Message: "this didn't fail",
	}
}
func scheduledEvent() v1.Event {
	return v1.Event{
		Reason:  "Scheduled",
		Message: "this didn't fail",
	}
}

func failedSchedulingEvent() v1.Event {
	return v1.Event{
		Reason:  "FailedScheduling",
		Message: "this failed",
	}
}

func (c TestClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return fmt.Errorf("not implemented")
}
func (c TestClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if c.listError != nil {
		return *c.listError
	}
	givenList, ok1 := list.(*v1.EventList)
	resultList, ok2 := c.listResult.(*v1.EventList)
	if ok1 && ok2 {
		givenList.Items = resultList.Items
	}
	return nil
}

func TestEventsInferredStateWhereEventsFailed(t *testing.T) {
	err := fmt.Errorf("not implemented")
	client := TestClient{
		listError: &err,
	}
	session := v1alpha.HpcAmaltheaSession{}
	result, errx := EventsInferedState(context.TODO(), &session, client)
	assert.Contains(t, errx.Error(), "not implemented")
	assert.Equal(t, EisrNone, result)
}

func TestEventsInferredStateWhereNoEvents(t *testing.T) {
	client := TestClient{}
	session := v1alpha.HpcAmaltheaSession{}
	result, err := EventsInferedState(context.TODO(), &session, client)
	assert.Equal(t, EisrNone, result)
	assert.Nil(t, err)
}

func TestEventsInferredStateWhereFailedSchedulingFirstTime(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{failedSchedulingEvent()}},
	}
	session := v1alpha.HpcAmaltheaSession{}
	assert.True(t, session.Status.FailedSchedulingSince.IsZero())
	result, err := EventsInferedState(context.TODO(), &session, client)
	assert.Equal(t, EisrInitiallyFailed, result)
	assert.Nil(t, err)
}

func TestEventsInferredStateWhereFailedSchedulingWithinTimeout(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{failedSchedulingEvent()}},
	}
	session := v1alpha.HpcAmaltheaSession{}
	session.Status.FailedSchedulingSince = metav1.NewTime(time.Now())
	assert.False(t, session.Status.FailedSchedulingSince.IsZero())
	result, err := EventsInferedState(context.TODO(), &session, client)
	assert.Equal(t, EisrTemporaryFailed, result)
	assert.Nil(t, err)
}

func TestEventsInferredStateWhereFailedSchedulingTimeoutExceeded(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{failedSchedulingEvent()}},
	}
	session := v1alpha.HpcAmaltheaSession{}
	time, _ := time.Parse(time.DateTime, "2006-01-02 15:04:05")
	session.Status.FailedSchedulingSince = metav1.NewTime(time)
	assert.False(t, session.Status.FailedSchedulingSince.IsZero())
	result, err := EventsInferedState(context.TODO(), &session, client)
	assert.Equal(t, EisrFinallyFailed, result)
	assert.Contains(t, err.Error(), "failed scheduling:")
}

func TestEventsInferredStateWhereScheduled(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{scheduledEvent()}},
	}
	session := v1alpha.HpcAmaltheaSession{}
	result, err := EventsInferedState(context.TODO(), &session, client)
	assert.Equal(t, EisrAutoScheduling, result)
	assert.Nil(t, err)
}

func TestEventsInferredStateWhereTriggeredScaleup(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{triggeredScaleUpEvent()}},
	}
	session := v1alpha.HpcAmaltheaSession{}
	result, err := EventsInferedState(context.TODO(), &session, client)
	assert.Equal(t, EisrAutoScheduling, result)
	assert.Nil(t, err)
}

func TestEventsInferredStateWhereTriggeredScaleupAfterFailed(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{
			failedSchedulingEvent(),
			arbitraryEvent(),
			triggeredScaleUpEvent(),
		}},
	}
	session := v1alpha.HpcAmaltheaSession{}
	result, err := EventsInferedState(context.TODO(), &session, client)
	assert.Equal(t, EisrAutoScheduling, result)
	assert.Nil(t, err)
}

func TestEventsInferredStateWhereFailedAfterScheduled(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{
			scheduledEvent(),
			arbitraryEvent(),
			triggeredScaleUpEvent(),
			scheduledEvent(),
			failedSchedulingEvent(),
		}},
	}
	session := v1alpha.HpcAmaltheaSession{}
	result, err := EventsInferedState(context.TODO(), &session, client)
	assert.Equal(t, EisrInitiallyFailed, result)
	assert.Nil(t, err)
}
