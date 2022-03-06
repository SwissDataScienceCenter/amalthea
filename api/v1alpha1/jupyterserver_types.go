/*
Copyright 2022.

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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// JupyterServerSpec defines the desired state of JupyterServer
type JupyterServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:={oidc: {enabled: false}}
	Auth JupyterServerSpecAuth `json:"auth,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:={idleSecondsThreshold: 0, maxAgeSecondsThreshold: 0}
	Culling JupyterServerSpecCulling `json:"culling,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:={defaultUrl: "/lab", image: "jupyter/minimal-notebook:latest", rootDir: "/home/jovyan/work"}
	JupyterServer JupyterServerSpecJupyterServer `json:"jupyterServer,omitempty"`
	// +kubebuilder:validation:Optional
	Patches []JupyterServerSpecPatch `json:"patches,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:={path: "/", tls: {enabled: false}}
	Routing JupyterServerSpecRouting `json:"routing,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:={pvc: {enabled: false, mountPath: "/home/jovyan/work"}, size: "100Mi"}
	Storage JupyterServerSpecStorage `json:"storage,omitempty"`
}

// JupyterServerStatus defines the observed state of JupyterServer
type JupyterServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// JupyterServer is the Schema for the jupyterservers API
type JupyterServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec    JupyterServerSpec    `json:"spec,omitempty"`
	Status  JupyterServerStatus  `json:"status,omitempty"`
	Secrets JupyterServerSecrets `json:"-"`
	Aux     JupyterServerAux     `json:"-"`
}

//+kubebuilder:object:root=true

// JupyterServerList contains a list of JupyterServer
type JupyterServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JupyterServer `json:"items"`
}

type JupyterServerSpecAuth struct {
	Token string                    `json:"token,omitempty"`
	Oidc  JupyterServerSpecAuthOidc `json:"oidc,omitempty"`
}

type JupyterServerSpecAuthOidc struct {
	AuthorizedEmails []string            `json:"authorizedEmails,omitempty"`
	AuthorizedGroups []string            `json:"authorizedGroups,omitempty"`
	ClientId         string              `json:"clientId,omitempty"`
	ClientSecret     JupyterServerSecret `json:"clientSecret,omitempty"`
	Enabled          bool                `json:"enabled,omitempty"`
	IssuerUrl        string              `json:"issuerUrl,omitempty"`
}

type JupyterServerSecret struct {
	Value        string                    `json:"value,omitempty"`
	SecretKeyRef JupyterServerSecretKeyRef `json:"secretKeyRef,omitempty"`
}

type JupyterServerSecretKeyRef struct {
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
}

type JupyterServerSpecCulling struct {
	IdleSecondsThreshold   int `json:"idleSecondsThreshold,omitempty"`
	MaxAgeSecondsThreshold int `json:"maxAgeSecondsThreshold,omitempty"`
}

type JupyterServerSpecJupyterServer struct {
	DefaultUrl string                  `json:"defaultUrl,omitempty"`
	Image      string                  `json:"image,omitempty"`
	Resources  v1.ResourceRequirements `json:"resources,omitempty"`
	RootDir    string                  `json:"rootDir,omitempty"`
}

type JupyterServerSpecPatch struct {
	Patch []byte `json:"patch,omitempty"`
	// +kubebuilder:validation:Enum=application/json-patch+json;application/merge-patch+json
	Type string `json:"type,omitempty"`
}

type JupyterServerSpecRouting struct {
	Host               string                      `json:"host,omitempty"`
	IngressAnnotations map[string]string           `json:"ingressAnntations,omitempty"`
	Path               string                      `json:"path,omitempty"`
	Tls                JupyterServerSpecRoutingTls `json:"tls,omitempty"`
}

type JupyterServerSpecRoutingTls struct {
	Enabled    bool   `json:"enabled,omitempty"`
	SecretName string `json:"secretName,omitempty"`
}

type JupyterServerSpecStorage struct {
	Pvc  JupyterServerSpecStoragePvc `json:"pvc,omitempty"`
	Size string                      `json:"size,omitempty"`
}

type JupyterServerSpecStoragePvc struct {
	Enabled          bool   `json:"enabled,omitempty"`
	MountPath        string `json:"mountPath,omitempty"`
	StorageClassName string `json:"storageClassName,omitempty"`
}

type JupyterServerSecrets struct {
	JupyterServerAppToken     string
	JupyterServerCookieSecret string
	AuthProviderCookieSecret  string
}

type JupyterServerAux struct {
	FullUrl       string
	SchedulerName string
}

func init() {
	SchemeBuilder.Register(&JupyterServer{}, &JupyterServerList{})
}
