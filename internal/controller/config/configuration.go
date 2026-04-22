package config

import (
	"k8s.io/client-go/rest"
)

type AmaltheaSessionConfiguration struct {
	ClusterType   ClusterType
	ImageRewriter ImageFieldRewriter
}

// GetImageRewriter loads the configuration
func GetAmaltheaSessionConfiguration(config *rest.Config) (cfg AmaltheaSessionConfiguration, err error) {
	clusterType, err := DetectClusterType(config)
	if err != nil {
		return AmaltheaSessionConfiguration{}, err
	}
	imageRewriter, err := GetImageFieldRewriter()
	if err != nil {
		return AmaltheaSessionConfiguration{}, err
	}
	return AmaltheaSessionConfiguration{ClusterType: clusterType, ImageRewriter: imageRewriter}, nil
}
