package config

import (
	"k8s.io/client-go/rest"
)

type AmaltheaSessionConfiguration struct {
	ClusterType        ClusterType
	ImageFieldRewriter ImageFieldRewriter
}

// GetAmaltheaSessionConfiguration loads the configuration
func GetAmaltheaSessionConfiguration(config *rest.Config) (cfg AmaltheaSessionConfiguration, err error) {
	clusterType, err := DetectClusterType(config)
	if err != nil {
		return AmaltheaSessionConfiguration{}, err
	}
	imageFieldRewriter, err := GetImageFieldRewriter()
	if err != nil {
		return AmaltheaSessionConfiguration{}, err
	}
	return AmaltheaSessionConfiguration{ClusterType: clusterType, ImageFieldRewriter: imageFieldRewriter}, nil
}
