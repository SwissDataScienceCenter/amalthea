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
	"net/url"
	"os"
	"path"
	"strings"

	api "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type K8sClients struct {
	DynamicClient   dynamic.Interface
	Kubeconfig      string
	DiscoveryClient *discovery.DiscoveryClient
	RestConfig      *rest.Config
	RestMapper      *restmapper.DeferredDiscoveryRESTMapper
	ClientSet       *kubernetes.Clientset
}

type JypterServerType struct {
	Manifest  api.JupyterServer
	Templates []byte
	K8s       K8sClients
	Children  []*unstructured.Unstructured
	// OwnerRef  metav1.OwnerReference
}

func NewJupyterServerFromManifest(manifest api.JupyterServer) (JypterServerType, error) {
	k8sClients, err := SetupK8sClients()
	if err != nil {
		fmt.Println("Could not setup k8s clients")
		return JypterServerType{}, err
	}
	// OwnerRefController := true
	// OwnerRefBlockOwnerDeletion := false
	js := JypterServerType{
		Manifest: manifest,
		K8s:      *k8sClients,
		Children: make([]*unstructured.Unstructured, 0, 8),
		// OwnerRef: metav1.OwnerReference{
		// 	APIVersion:         manifest.APIVersion,
		// 	Kind:               manifest.Kind,
		// 	Name:               manifest.Name,
		// 	UID:                manifest.UID,
		// 	Controller:         &OwnerRefController,
		// 	BlockOwnerDeletion: &OwnerRefBlockOwnerDeletion,
		// },
	}
	manifest.Aux, err = getAuxFromManifest(manifest)
	if err != nil {
		fmt.Println("Could not get Aux from manifest")
		return JypterServerType{}, err
	}
	manifest.Secrets = js.getSecrets()

	return js, nil
}

func getAuxFromManifest(manifest api.JupyterServer) (api.JupyterServerAux, error) {
	http := "http"
	if manifest.Spec.Routing.Tls.Enabled {
		http += "s"
	}
	fullUrl, err := url.Parse(http + "://" + manifest.Spec.Routing.Host)
	if err != nil {
		fmt.Println("Could not parse url")
		return api.JupyterServerAux{}, err
	}
	serverPath := strings.TrimSuffix(manifest.Spec.Routing.Path, "/")
	fullUrl.Path = path.Join(fullUrl.Path, serverPath)
	return api.JupyterServerAux{
		FullUrl:       fullUrl.String(),
		SchedulerName: os.Getenv("SERVER_SCHEDULER_NAME"),
	}, nil
}

func (js *JypterServerType) getSecrets() api.JupyterServerSecrets {
	secret, err := js.K8s.ClientSet.CoreV1().Secrets(js.Manifest.Namespace).Get(context.TODO(), js.Manifest.Name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			// there is no secret in the cluster, make the secrets
			return api.JupyterServerSecrets{
				JupyterServerAppToken:     "fdf",
				JupyterServerCookieSecret: "fdsfds",
				AuthProviderCookieSecret:  "dsfasdfafa",
			}
		} else {
			fmt.Println("Something went wront with locating secrets")
		}
	}
	// the secret exists in the cluster
	return api.JupyterServerSecrets{
		JupyterServerAppToken:     secret.StringData["jupyterServerAppToken"],
		JupyterServerCookieSecret: secret.StringData["jupyterServerCookieSecret"],
		AuthProviderCookieSecret:  secret.StringData["authProviderCookieSecret"],
	}
}
