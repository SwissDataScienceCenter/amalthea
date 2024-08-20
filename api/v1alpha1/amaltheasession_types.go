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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

// AmaltheaSessionSpec defines the desired state of AmaltheaSession
type AmaltheaSessionSpec struct {
	// Specification for the main session container that the user will access and use
	Session Session `json:"session"`

	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="CodeRepositories is immutable"
	// A list of code repositories and associated configuration that will be cloned in the session
	CodeRepositories []CodeRepository `json:"codeRepositories,omitempty"`

	// +optional
	// A list of data sources that should be added to the session
	DataSources []DataSource `json:"dataSources,omitempty"`

	// Authentication configuration for the session
	Authentication *Authentication `json:"authentication,omitempty"`

	// Culling configuration
	Culling Culling `json:"culling,omitempty"`

	// +kubebuilder:default:=false
	// Will hibernate the session, scaling the session's statefulset to zero.
	Hibernated bool `json:"hibernated"`

	// +kubebuilder:default:=false
	// Whether to adopt all secrets referred to by name in this CR. Adopted secrets will be deleted when the CR is deleted.
	AdoptSecrets bool `json:"adoptSecrets"`

	// +optional
	// Additional containers to add to the session statefulset.
	// NOTE: The container names provided will be partially overwritten and randomized to avoid collisions
	ExtraContainers []v1.Container `json:"extraContainers,omitempty"`

	// +optional
	// Additional init containers to add to the session statefulset
	// NOTE: The container names provided will be partially overwritten and randomized to avoid collisions
	ExtraInitContainers []v1.Container `json:"initContainers,omitempty"`

	// +optional
	// Additional volumes to include in the statefulset for a session
	ExtraVolumes []v1.Volume `json:"extraVolumes,omitempty"`

	// +optional
	// Configuration for an ingress to the session, if omitted a Kubernetes Ingress will not be created
	Ingress *Ingress `json:"ingress,omitempty"`
}

type Session struct {
	Image string `json:"image"`
	// The command to run in the session container, if omitted it will use the Docker image ENTRYPOINT
	Command []string `json:"command,omitempty"`
	// The arguments to run in the session container, if omitted it will use the Docker image CMD
	Args []string    `json:"args,omitempty"`
	Env  []v1.EnvVar `json:"env,omitempty"`
	// Resource requirements and limits in the same format as a Pod in Kubernetes
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// +kubebuilder:default:=8000
	// +kubebuilder:validation:ExclusiveMinimum:=true
	// +kubebuilder:validation:Minimum:=0
	// The TCP port on the session Pod where the session can be accessed. Can point to either the
	// session container or any additional container that is added. K8s port names are not accepted.
	Port int32 `json:"port"`
	// +optional
	// +kubebuilder:default:={}
	Storage Storage `json:"storage,omitempty"`
	// +optional
	// Size of /dev/shm
	ShmSize *resource.Quantity `json:"shmSize,omitempty"`
	// The abolute path for the working directory of the session container, if omitted it will use the image
	// working directory.
	WorkingDir string `json:"workingDir,omitempty"`
	// +optional
	// +kubebuilder:default:=1000
	// +kubebuilder:validation:Minimum:=0
	RunAsUser int64 `json:"runAsUser,omitempty"`
	// +optional
	// +kubebuilder:default:=1000
	// +kubebuilder:validation:Minimum:=0
	RunAsGroup int64 `json:"runAsGroup,omitempty"`
	// +optional
	// +kubebuilder:default:="/"
	// The path where the session can be accessed. If an ingress is specified, this value must
	// be a subpath of the ingress `pathPrefix` field.
	URLPath string `json:"urlPath,omitempty"`
	// +optional
	// Additional volume mounts for the session container
	ExtraVolumeMounts []v1.VolumeMount `json:"extraVolumeMounts,omitempty"`
}

type Ingress struct {
	Annotations map[string]string `json:"annotations,omitempty"`
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Host is immutable"
	Host string `json:"host"`
	// +optional
	// +kubebuilder:default:="/"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="PathPrefix is immutable"
	PathPrefix *string `json:"pathPrefix,omitempty"`
	// +optional
	// The name of the TLS secret, same as what is specified in a regular Kubernetes Ingress.
	TLSSecretName *string `json:"tlsSecretName,omitempty"`
}

type Storage struct {
	// +optional
	ClassName *string `json:"className,omitempty"`
	// +optional
	// +kubebuilder:default:="1Gi"
	Size *resource.Quantity `json:"size,omitempty"`
	// The absolute mount path for the session volume
	// +optional
	// +kubebuilder:default:="/workspace"
	MountPath string `json:"mountPath,omitempty"`
}

// +kubebuilder:validation:Enum={git}
type CodeRepositoryType string

const Git CodeRepositoryType = "git"

type CodeRepository struct {
	// +kubebuilder:default:=git
	// The type of the code repository - currently the only supported kind is git.
	Type CodeRepositoryType `json:"type,omitempty"`
	// +kubebuilder:example:=repositories/project1
	// +kubebuilder:default:="."
	// Path relative to the session working directory where the repository should be cloned into.
	ClonePath string `json:"clonePath,omitempty"`
	// +kubebuilder:example:="https://github.com/SwissDataScienceCenter/renku"
	// The HTTP url to the code repository
	Remote string `json:"remote"`
	// +kubebuilder:example:=main
	// The tag, branch or commit SHA to checkout, if omitted then will be the tip of the default branch of the repo
	Revision string `json:"revision,omitempty"`
	// The Kubernetes secret that contains the code repository configuration to be used during cloning.
	// For 'git' this should contain either:
	// The username and password
	// The private key and its corresponding password
	// An empty value can be used when cloning from public repositories using the http protocol
	// NOTE: you have to specify the whole config in a single key in the secret.
	CloningConfigSecretRef *SessionSecretRef `json:"cloningConfigSecretRef,omitempty"`
	// The Kubernetes secret that contains the code repository configuration to be used when the session is running.
	// For 'git' this is the git configuration which can be used to inject credentials in addition to any other repo-specific Git configuration.
	// NOTE: you have to specify the whole config in a single key in the secret.
	ConfigSecretRef *SessionSecretRef `json:"configSecretRef,omitempty"`
}

// +kubebuilder:validation:Enum={rclone}
type StorageType string

const Rclone StorageType = "rclone"

type DataSource struct {
	// +kubebuilder:default:=rclone
	// The data source type
	Type StorageType `json:"type,omitempty"`
	// +kubebuilder:example:=data/storages
	// +kubebuilder:default:="data"
	// Path relative to the session working directory where the data should be mounted
	MountPath string `json:"mountPath,omitempty"`
	// The secret containing the configuration or credentials needed for access to the data.
	// The format of the configuration that is expected depends on the storage type.
	// NOTE: define all values in a single key of the Kubernetes secret.
	// rclone: any valid rclone configuration for a single remote, see the output of `rclone config providers` for validation and format.
	SecretRef *SessionSecretRef `json:"secretRef,omitempty"`
}

type Culling struct {
	// +kubebuilder:validation:Format:=duration
	// The maximum allowed age for a session, regardless of whether it
	// is active or not. When the threshold is reached the session is hibernated.
	// A value of zero indicates that Amalthea will not automatically hibernate
	// the session based on its age.
	// Golang's time.ParseDuration is used to parse this, so values like 2h5min will work,
	// valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
	MaxAge metav1.Duration `json:"maxAge,omitempty"`
	// +kubebuilder:validation:Format:=duration
	// How long should a server be idle for before it is hibernated. A value of
	// zero indicates that Amalthea will not automatically hibernate inactive sessions.
	// Golang's time.ParseDuration is used to parse this, so values like 2h5min will work,
	// valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
	MaxIdleDuration metav1.Duration `json:"maxIdleDuration,omitempty"`
	// +kubebuilder:validation:Format:=duration
	// How long can a server be in starting state before it gets hibernated. A
	// value of zero indicates that the server will not be automatically hibernated
	// by Amalthea because it took to long to start.
	// Golang's time.ParseDuration is used to parse this, so values like 2h5min will work,
	// valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
	MaxStartingDuration metav1.Duration `json:"maxStartingDuration,omitempty"`
	// +kubebuilder:validation:Format:=duration
	// How long can a server be in failed state before it gets hibernated. A
	// value of zero indicates that the server will not be automatically
	// hibernated by Amalthea if it is failing.
	// Golang's time.ParseDuration is used to parse this, so values like 2h5min will work,
	// valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
	MaxFailedDuration metav1.Duration `json:"maxFailedDuration,omitempty"`
	// +kubebuilder:validation:Format:=duration
	// How long can a session be in hibernated state before
	// it gets completely deleted. A value of zero indicates that hibernated servers
	// will not be automatically be deleted by Amalthea after a period of time.
	// Golang's time.ParseDuration is used to parse this, so values like 2h5min will work,
	// valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
	MaxHibernatedDuration metav1.Duration `json:"maxHibernatedDuration,omitempty"`
}

// +kubebuilder:validation:Enum={token,oauth2proxy}
type AuthenticationType string

const Token AuthenticationType = "token"
const Oidc AuthenticationType = "oauth2proxy"

type Authentication struct {
	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=true
	Enabled bool               `json:"enabled"`
	Type    AuthenticationType `json:"type"`
	// Kubernetes secret that contains the authentication configuration
	// For `token` generate a hard to guess string / password-like string.
	// this value can be used as Authorization header or as a cookie with the name `amaltheaSessionToken` to
	// access the session.
	// For `oauth2proxy` please see https://oauth2-proxy.github.io/oauth2-proxy/configuration/overview#config-file.
	SecretRef SessionSecretRef `json:"secretRef"`
	// +optional
	// Additional volume mounts for the authentication container.
	ExtraVolumeMounts []v1.VolumeMount `json:"extraVolumeMounts,omitempty"`
}

// A reference to a Kubernetes secret and a specific field in the secret to be used in a session
type SessionSecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// +kubebuilder:validation:Enum={Running,Failed,Hibernated,NotReady,RunningDegraded}
type State string

const Running State = "Running"
const Failed State = "Failed"
const Hibernated State = "Hibernated"
const NotReady State = "NotReady"
const RunningDegraded State = "RunningDegraded"

// Counts of the total and ready containers, can represent either regular or init containers.
type ContainerCounts struct {
	Ready int `json:"ready,omitempty"`
	Total int `json:"total,omitempty"`
}

func (c ContainerCounts) Ok() bool {
	return c.Ready == c.Total
}

// AmaltheaSessionStatus defines the observed state of AmaltheaSession
type AmaltheaSessionStatus struct {
	// Conditions store the status conditions of the AmaltheaSessions. This is a standard thing that
	// many operators implement see https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []AmaltheaSessionCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// +kubebuilder:default:=NotReady
	State               State           `json:"state,omitempty"`
	URL                 string          `json:"url,omitempty"`
	ContainerCounts     ContainerCounts `json:"containerCounts,omitempty"`
	InitContainerCounts ContainerCounts `json:"initContainerCounts,omitempty"`
	Idle                bool            `json:"idle,omitempty"`
	// +kubebuilder:validation:Format:=date-time
	IdleSince metav1.Time `json:"idleSince,omitempty"`
	// +kubebuilder:validation:Format:=date-time
	FailingSince metav1.Time `json:"failingSince,omitempty"`
	// +kubebuilder:validation:Format:=date-time
	HibernatedSince metav1.Time `json:"hibernatedSince,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=`.status.state`,description="The overall status of the session."
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.containerCounts.ready`,description="The number of containers in a ready state for the session, disregarding init containers."
// +kubebuilder:printcolumn:name="Total",type="string",JSONPath=`.status.containerCounts.total`,description="The total numeber of containers in the session, disregarding init containers."
// +kubebuilder:printcolumn:name="Idle",type="boolean",JSONPath=`.status.idle`,description="Whether the session is idle or not."
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=`.status.url`,description="The URL where the session can be accessed."
// AmaltheaSession is the Schema for the amaltheasessions API
type AmaltheaSession struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AmaltheaSessionSpec   `json:"spec,omitempty"`
	Status AmaltheaSessionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AmaltheaSessionList contains a list of AmaltheaSession
type AmaltheaSessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AmaltheaSession `json:"items"`
}

type AmaltheaSessionConditionType string

const (
	AmaltheaSessionReady        AmaltheaSessionConditionType = "Ready"
	AmaltheaSessionRoutingReady AmaltheaSessionConditionType = "RoutingReady"
)

type AmaltheaSessionCondition struct {
	Type   AmaltheaSessionConditionType `json:"type"`
	Status metav1.ConditionStatus       `json:"status"`
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

func init() {
	SchemeBuilder.Register(&AmaltheaSession{}, &AmaltheaSessionList{})
}
