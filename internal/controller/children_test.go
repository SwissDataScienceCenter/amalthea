package controller

import (
	"context"
	"fmt"
	v1alpha "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
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

func TestEventsInferredFailureWhereEventsFailed(t *testing.T) {
	err := fmt.Errorf("not implemented")
	client := TestClient{
		listError: &err,
	}
	session := v1alpha.AmaltheaSession{}
	result := eventsInferedFailure(&session, client, context.TODO())
	assert.Contains(t, result, "not implemented")
}

func TestEventsInferredFailureWhereNoEvents(t *testing.T) {
	client := TestClient{}
	session := v1alpha.AmaltheaSession{}
	result := eventsInferedFailure(&session, client, context.TODO())
	assert.Equal(t, result, "")
}

func TestEventsInferredFailureWhereFailedScheduling(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{failedSchedulingEvent()}},
	}
	session := v1alpha.AmaltheaSession{}
	result := eventsInferedFailure(&session, client, context.TODO())
	assert.Equal(t, result, "Failed scheduling: this failed")
}

func TestEventsInferredFailureWhereScheduled(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{scheduledEvent()}},
	}
	session := v1alpha.AmaltheaSession{}
	result := eventsInferedFailure(&session, client, context.TODO())
	assert.Equal(t, result, "")
}

func TestEventsInferredFailureWhereTriggeredScaleup(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{triggeredScaleUpEvent()}},
	}
	session := v1alpha.AmaltheaSession{}
	result := eventsInferedFailure(&session, client, context.TODO())
	assert.Equal(t, result, "")
}

func TestEventsInferredFailureWhereTriggeredScaleupAfterFailed(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{
			failedSchedulingEvent(),
			arbitraryEvent(),
			triggeredScaleUpEvent(),
		}},
	}
	session := v1alpha.AmaltheaSession{}
	result := eventsInferedFailure(&session, client, context.TODO())
	assert.Equal(t, result, "")
}

func TestEventsInferredFailureWhereFailedAfterScheduled(t *testing.T) {
	client := TestClient{
		listResult: &v1.EventList{Items: []v1.Event{
			scheduledEvent(),
			arbitraryEvent(),
			triggeredScaleUpEvent(),
			failedSchedulingEvent(),
			scheduledEvent(),
		}},
	}
	session := v1alpha.AmaltheaSession{}
	result := eventsInferedFailure(&session, client, context.TODO())
	assert.Equal(t, result, "")
}
