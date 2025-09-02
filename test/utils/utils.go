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

package utils

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/SwissDataScienceCenter/amalthea/internal/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	prometheusOperatorVersion = "v0.68.0"
	prometheusOperatorURL     = "https://github.com/prometheus-operator/prometheus-operator/" +
		"releases/download/%s/bundle.yaml"

	certmanagerVersion = "v1.5.3"
	certmanagerURLTmpl = "https://github.com/jetstack/cert-manager/releases/download/%s/cert-manager.yaml"

	metricsServerVersion = "v0.7.2"
	metricsServerURLTmpl = "https://github.com/kubernetes-sigs/metrics-server/releases/download/%s/components.yaml"

	sdscHelmRepository = "https://swissdatasciencecenter.github.io/helm-charts/"
	helmRepoName       = "renku-test"
)

func warnError(err error) {
	GinkgoLogr.Error(err, "warning error occurred")
}

// InstallPrometheusOperator installs the prometheus Operator to be used to export the enabled metrics.
func InstallPrometheusOperator() error {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command("kubectl", "create", "-f", url)
	_, err := Run(cmd)
	return err
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) ([]byte, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		GinkgoLogr.Error(err, "changing directory failed")
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	GinkgoLogr.Info("running command", "command", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s failed with error: (%v) %s", command, err, string(output))
	}

	return output, nil
}

// UninstallPrometheusOperator uninstalls the prometheus
func UninstallPrometheusOperator() {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command("kubectl", "delete", "--ignore-not-found", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager() {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "delete", "--ignore-not-found", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// InstallCertManager installs the cert manager bundle.
func InstallCertManager() error {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "apply", "-f", url)
	if _, err := Run(cmd); err != nil {
		return err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.Command("kubectl", "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)

	_, err := Run(cmd)
	return err
}

// InstallMetricsServer installs the metrics server
func InstallMetricsServer() error {
	url := fmt.Sprintf(metricsServerURLTmpl, metricsServerVersion)
	cmd := exec.Command("kubectl", "apply", "-f", url)
	if _, err := Run(cmd); err != nil {
		return err
	}

	cmd = exec.Command("kubectl", "patch", "-n", "kube-system", "deployment", "metrics-server", "--type=json",
		"-p", "[{\"op\":\"add\",\"path\":\"/spec/template/spec/containers/0/args/-\",\"value\":\"--kubelet-insecure-tls\"}]")

	if _, err := Run(cmd); err != nil {
		return err
	}

	// Wait for metrics-server pod to be ready, which can take time
	cmd = exec.Command("kubectl", "wait", "deployment.apps/metrics-server",
		"--for", "condition=Available",
		"--namespace", "kube-system",
		"--timeout", "5m",
	)

	_, err := Run(cmd)

	return err
}

// UninstallMetricsServer uninstalls the metrics server
func UninstallMetricsServer() {
	url := fmt.Sprintf(metricsServerURLTmpl, metricsServerVersion)
	cmd := exec.Command("kubectl", "delete", "--ignore-not-found", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

func CreateNamespace(namespace string) {
	cmd := exec.Command("kubectl", "create", "ns", namespace)
	_, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
		warnError(err)
	}
}

// LoadImageToKindCluster loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.SplitSeq(output, "\n")
	for element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.ReplaceAll(wd, "/test/e2e", "")
	return wd, nil
}

func InstallHelmChart(ctx context.Context, namespace string, releaseName string, chart string) error {
	cmd := exec.CommandContext(ctx, "chartpress")
	dir, err := GetProjectDir()
	if err != nil {
		return err
	}
	cmd.Dir = dir
	_, err = Run(cmd)
	if err != nil {
		return err
	}
	cmd = exec.CommandContext(ctx, "make", "-s", "list-chartpress-images")
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	projDir, err := GetProjectDir()
	if err != nil {
		return err
	}
	cmd.Dir = projDir
	GinkgoLogr.Info("running command", "command", cmd)
	err = cmd.Run()
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		image := scanner.Text()
		if image == "" {
			continue
		}
		err = LoadImageToKindClusterWithName(image)
		if err != nil {
			return err
		}
	}
	err = scanner.Err()
	if err != nil {
		return err
	}
	cmd = exec.CommandContext(
		ctx,
		"helm",
		"repo",
		"add",
		helmRepoName,
		sdscHelmRepository,
	)
	_, err = Run(cmd)
	if err != nil {
		return err
	}
	cmd = exec.CommandContext(
		ctx,
		"helm",
		"dep",
		"build",
		chart,
	)
	_, err = Run(cmd)
	if err != nil {
		return err
	}
	cmd = exec.CommandContext(
		ctx,
		"helm",
		"-n",
		namespace,
		"upgrade",
		"--create-namespace",
		"--install",
		"--wait",
		"--timeout",
		"6m",
		releaseName,
		chart,
	)
	_, err = Run(cmd)
	if err != nil {
		return err
	}
	return nil
}

func UninstallHelmChart(ctx context.Context, namespace string, releaseName string) error {
	cmd := exec.CommandContext(ctx, "helm", "-n", namespace, "uninstall", releaseName, "--wait", "--timeout", "5m")
	_, errUninstall := Run(cmd)
	cmd = exec.CommandContext(ctx, "helm", "repo", "remove", helmRepoName)
	_, errRemove := Run(cmd)

	return errors.Join(errUninstall, errRemove)
}

func getScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	err := clientgoscheme.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = amaltheadevv1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	return scheme, nil
}

func GetK8sClient(ctx context.Context, namespace string) (client.Client, error) {
	scheme, err := getScheme()
	if err != nil {
		return nil, err
	}

	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}

	mgr, err := ctrl.NewManager(
		config,
		ctrl.Options{
			Scheme: scheme,
			Client: client.Options{
				Cache: &client.CacheOptions{
					DisableFor: []client.Object{
						&amaltheadevv1alpha1.HpcAmaltheaSession{},
						&corev1.Pod{},
						&corev1.Event{},
					},
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return mgr.GetClient(), nil
}

func GetRandomName() string {
	prefix := "amalthea-test-"
	const length int = 8
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return prefix + string(result)
}

func GetController(namespace string) (manager.Manager, error) {
	scheme, err := getScheme()
	if err != nil {
		return nil, err
	}
	cacheOptions := cache.Options{
		DefaultNamespaces: map[string]cache.Config{namespace: {}},
	}
	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: scheme,
		Cache:  cacheOptions,
	})
	if err != nil {
		return nil, err
	}
	metricsClient := metricsv.NewForConfigOrDie(config).MetricsV1beta1()
	reconciler := &controller.AmaltheaSessionReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		MetricsClient: metricsClient,
	}
	err = reconciler.SetupWithManager(mgr)
	if err != nil {
		return nil, err
	}
	ctx := ctrl.SetupSignalHandler()
	field_ctx, cancel := context.WithTimeoutCause(
		ctx,
		30*time.Second,
		errors.New("timeout exceeded for setting up field indexers"),
	)
	err = mgr.GetFieldIndexer().IndexField(
		field_ctx,
		&corev1.Event{},
		"involvedObject.name",
		func(obj client.Object) []string {
			return []string{obj.(*corev1.Event).InvolvedObject.Name}
		})
	if err != nil {
		fmt.Printf("unable to index field involvedObject.name on events: %s", err)
	}
	err = mgr.GetFieldIndexer().IndexField(field_ctx,
		&corev1.Event{},
		"involvedObject.namespace",
		func(obj client.Object) []string {
			return []string{obj.(*corev1.Event).InvolvedObject.Namespace}
		})
	if err != nil {
		fmt.Printf("unable to index field involvedObject.namespace on events: %s", err)
	}
	err = mgr.GetFieldIndexer().IndexField(field_ctx,
		&corev1.Event{},
		"involvedObject.kind",
		func(obj client.Object) []string {
			return []string{obj.(*corev1.Event).InvolvedObject.Kind}
		})
	if err != nil {
		fmt.Printf("unable to index field involvedObject.kind on events: %s", err)
	}
	cancel()
	return mgr, nil
}
