package kube

import (
	"context"
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var clientset *kubernetes.Clientset
var currentNamespace string

func loadConfig() (*rest.Config, error) {
	// In Cluster Config
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	if !errors.Is(err, rest.ErrNotInCluster) {
		return nil, err
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	config, err = kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	currentNamespace, _, err = kubeConfig.Namespace()
	if err != nil {
		return nil, err
	}

	return config, nil
}

func client() (*kubernetes.Clientset, error) {
	if clientset != nil {
		return clientset, nil
	}
	config, err := loadConfig()
	if err != nil {
		return nil, err
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		clientset = nil
		currentNamespace = ""
		return nil, err
	}
	return clientset, nil
}

func Secret(ctx context.Context, name string) (*v1.Secret, error) {
	cs, err := client()
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("failed to read Secret with K8s client because name is blank")
	}
	return cs.CoreV1().Secrets(currentNamespace).Get(ctx, name, metav1.GetOptions{})
}
