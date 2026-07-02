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
		return nil, err
	}
	return clientset, nil
}

func ListSecret(ctx context.Context, namespace, name string) (*v1.SecretList, error) {
	cs, err := client()
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("failed to list secret because name is blank")
	}
	return cs.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", name)})
}
