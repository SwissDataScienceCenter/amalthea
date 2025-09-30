package v1alpha1

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

type ClusterType string

const Kubernetes ClusterType = "kubernetes"
const OpenShift ClusterType = "openshift"
const UnknownVariant ClusterType = "unknown"

func DetectClusterType(config *rest.Config) (ClusterType, error) {
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
