/*
Copyright 2026.

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

package children

import (
	"context"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/labstack/gommon/log"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ChildResourceType interface {
	metav1.Object
	runtime.Object
	*v1.Service | *v1.PersistentVolumeClaim | *appsv1.StatefulSet | *v1.Secret | *v1.PersistentVolume
}

type MutateFn[T ChildResourceType] func(T) error
type StatusCallback func(*amaltheadevv1alpha1.AmaltheaSessionStatus)

type ChildResource[T ChildResourceType] struct {
	mutateFn       MutateFn[T]
	obj            T
	statusCallback StatusCallback
}

func NewChildResource[T ChildResourceType](opts ...ChildResourceOption[T]) ChildResource[T] {
	obj := *new(T)
	output := ChildResource[T]{
		obj: obj,
	}
	for _, opt := range opts {
		opt(&output)
	}
	return output
}

type ChildResourceOption[T ChildResourceType] func(*ChildResource[T])

func WithName[T ChildResourceType](name string) ChildResourceOption[T] {
	return func(cr *ChildResource[T]) {
		cr.obj.SetName(name)
	}
}

func WithNamespace[T ChildResourceType](namespace string) ChildResourceOption[T] {
	return func(cr *ChildResource[T]) {
		cr.obj.SetNamespace(namespace)
	}
}

func WithMutateFn[T ChildResourceType](mutateFn MutateFn[T]) ChildResourceOption[T] {
	return func(cr *ChildResource[T]) {
		cr.mutateFn = mutateFn
	}
}

func WithStatusCallback[T ChildResourceType](statusCallback StatusCallback) ChildResourceOption[T] {
	return func(cr *ChildResource[T]) {
		cr.statusCallback = statusCallback
	}
}

func (cr *ChildResource[T]) Reconcile(ctx context.Context, clnt client.Client) error {
	if cr.mutateFn == nil {
		log.Warnf("Mutate function not set for type %s with name %s, namespace: %s", cr.obj.GetName(), cr.obj.GetNamespace(), cr.obj.GetObjectKind().GroupVersionKind().String())
		return nil
	}
	return cr.mutateFn(cr.obj)
}

func (cr *ChildResource[T]) StatusCallback(status *amaltheadevv1alpha1.AmaltheaSessionStatus) {
	if cr.statusCallback == nil {
		return
	}
	cr.statusCallback(status)
}

type ChildResourcer interface {
	StatusCallback(status *amaltheadevv1alpha1.AmaltheaSessionStatus)
	Reconcile(ctx context.Context, clnt client.Client) error
}
