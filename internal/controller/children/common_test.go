package children

import (
	"log"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

func test() {
	var a = NewChildResource(
		new(v1.Service),
		WithName[*v1.Service]("test"),
		WithNamespace[*v1.Service]("test"),
		WithMutateFn(func(cr *v1.Service) error {
			return nil
		}),
		WithStatusCallback[*v1.Service](func(status *amaltheadevv1alpha1.AmaltheaSessionStatus) {
			log.Println("Hi")
		}),
	)
	a.mutateFn(a.obj)
	a.obj.GroupVersionKind()
}
