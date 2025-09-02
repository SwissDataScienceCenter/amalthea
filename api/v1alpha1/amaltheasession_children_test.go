package v1alpha1

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestClient struct {
	listError  *error
	listResult client.ObjectList
}

/* Implementing the client.Reader interface */
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

func TestGetPodEventsNoEvents(t *testing.T) {
	client := TestClient{}
	session := HpcAmaltheaSession{}
	res, err := session.GetPodEvents(context.TODO(), client)
	assert.Nil(t, err)
	assert.Equal(t, *res, v1.EventList{})
}

func TestGetPodEventsWithError(t *testing.T) {
	err := fmt.Errorf("oopsies")
	client := TestClient{
		listError: &err,
	}
	session := HpcAmaltheaSession{}
	_, err = session.GetPodEvents(context.TODO(), client)
	assert.NotNil(t, err)
}

func TestGetPodEventsSorted(t *testing.T) {
	ref := metav1.Now()
	ev1 := v1.Event{Reason: "bla", FirstTimestamp: metav1.NewTime(ref.AddDate(0, 1, 1))}
	ev2 := v1.Event{Reason: "blup", EventTime: metav1.MicroTime(ref)}
	ev3 := v1.Event{Reason: "foo", FirstTimestamp: metav1.NewTime(ref.AddDate(0, -1, 0))}
	events := []v1.Event{ev1, ev2, ev3}
	client := TestClient{
		listResult: &v1.EventList{Items: events},
	}
	session := HpcAmaltheaSession{}
	res, err := session.GetPodEvents(context.TODO(), client)
	assert.Nil(t, err)
	assert.Equal(t, res.Items, []v1.Event{ev3, ev2, ev1})
}
