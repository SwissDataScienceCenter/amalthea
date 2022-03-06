/*
Copyright 2021.

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

package jupyterserver

import (
	"context"
	"fmt"

	api "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// JupyterServerReconciler reconciles a JupyterServer object
type JupyterServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=amalthea.dev.olevski90,resources=jupyterservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=amalthea.dev.olevski90,resources=jupyterservers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the JupyterServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *JupyterServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	manifest := &api.JupyterServer{}
	err := r.Get(ctx, req.NamespacedName, manifest)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("JupyterServer resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Cannot get jupyterserver resource.")
		return ctrl.Result{}, err
	}
	secret := &corev1.Secret{}
	err = r.Get(ctx, req.NamespacedName, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			newSecrets, err := generateServerSecrets()
			if err != nil {
				return ctrl.Result{}, err
			}
			fmt.Println("Generated secrets")
			fmt.Printf("%+v\n", newSecrets)
			manifest.Secrets = newSecrets
		} else {
			return ctrl.Result{}, err
		}
	} else {
		fmt.Println("Found existing secret!!!")
		fmt.Println(string(secret.Data["jupyterServerAppToken"]))
		fmt.Println(string(secret.Data["jupyterServerCookieSecret"]))
		fmt.Println(string(secret.Data["authProviderCookieSecret"]))
		manifest.Secrets = api.JupyterServerSecrets{
			JupyterServerAppToken:     string(secret.Data["jupyterServerAppToken"]),
			JupyterServerCookieSecret: string(secret.Data["jupyterServerCookieSecret"]),
			AuthProviderCookieSecret:  string(secret.Data["authProviderCookieSecret"]),
		}
	}
	aux, err := getAuxFromManifest(manifest)
	if err != nil {
		fmt.Println("Could not get Aux from manifest")
		return ctrl.Result{}, err
	}
	manifest.Aux = aux
	js, err := NewJupyterServerFromManifest(manifest)
	if err != nil {
		return ctrl.Result{}, err
	}
	fmt.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	fmt.Printf("%+v\n", js.Manifest)
	fmt.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	err = js.RenderTemplates()
	if err != nil {
		return ctrl.Result{}, err
	}
	js.ApplyPatches()
	if err != nil {
		return ctrl.Result{}, err
	}
	err = js.GetUnstructuredResources()
	if err != nil {
		log.Error(err, "Cannot parse resources.")
		return ctrl.Result{}, err
	}
	missingResources, err := js.GetMissingResources()
	if err != nil {
		log.Error(err, "Cannot get missing resources.")
		return ctrl.Result{}, err
	}
	err = js.CreateResources(missingResources)
	if err != nil {
		log.Error(err, "Cannot create missing resources.")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JupyterServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.JupyterServer{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
