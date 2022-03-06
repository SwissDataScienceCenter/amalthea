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
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path"
	"strings"

	api "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
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
	Manifest  *api.JupyterServer
	Templates []byte
	K8s       K8sClients
	Children  []*unstructured.Unstructured
}

func NewJupyterServerFromManifest(manifest *api.JupyterServer) (JypterServerType, error) {
	k8sClients, err := SetupK8sClients()
	if err != nil {
		fmt.Println("Could not setup k8s clients")
		return JypterServerType{}, err
	}
	js := JypterServerType{
		Manifest: manifest,
		K8s:      *k8sClients,
		Children: make([]*unstructured.Unstructured, 0, 8),
	}

	return js, nil
}

func getAuxFromManifest(manifest *api.JupyterServer) (api.JupyterServerAux, error) {
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

func generateServerSecrets() (api.JupyterServerSecrets, error) {
	fmt.Println("Generating secrets!!!!!")
	// there is no secret in the cluster, make the secrets
	jsTokenSecret, err := GenerateRandomString(32)
	if err != nil {
		return api.JupyterServerSecrets{}, err
	}
	jsCookieSecret, err := GenerateRandomString(32)
	if err != nil {
		return api.JupyterServerSecrets{}, err
	}
	authCookieSecret, err := GenerateRandomString(32)
	if err != nil {
		return api.JupyterServerSecrets{}, err
	}
	fmt.Println("Secret values:!!!")
	fmt.Println(jsTokenSecret, jsCookieSecret, authCookieSecret)
	return api.JupyterServerSecrets{
		JupyterServerAppToken:     jsTokenSecret,
		JupyterServerCookieSecret: jsCookieSecret,
		AuthProviderCookieSecret:  authCookieSecret,
	}, nil
}

func GenerateRandomString(n int) (string, error) {
	// Taken from: https://gist.github.com/dopey/c69559607800d2f2f90b1b1ed4e550fb
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret), nil
}
