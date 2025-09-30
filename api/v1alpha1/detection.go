package v1alpha1

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

type K8sVariant string

const Kubernetes K8sVariant = "kubernetes"
const OpenShift K8sVariant = "openshift"
const UnknownVariant K8sVariant = "unknown"

func DetectClusterType(config *rest.Config) (K8sVariant, error) {
	dcl, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return UnknownVariant, err
	}

	apiList, err := dcl.ServerGroups()
	if err != nil {
		return UnknownVariant, err
	}

	apiGroups := apiList.Groups
	for i := range apiGroups {
		if apiGroups[i].Name == "project.openshift.io" {
			return OpenShift, nil
		}
	}

	return Kubernetes, nil
}
