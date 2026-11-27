package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SwissDataScienceCenter/amalthea/internal/controller/config"
)

func envMap(env []v1.EnvVar) map[string]string {
	output := make(map[string]string, len(env))
	for _, e := range env {
		output[e.Name] = e.Value
	}
	return output
}

func TestWithProxyEnvNil(t *testing.T) {
	containers := []v1.Container{{Name: "c", Env: []v1.EnvVar{{Name: "FOO", Value: "bar"}}}}
	assert.Equal(t, containers, withProxyEnv(nil, containers))
	assert.Equal(t, containers, withProxyEnv(&HTTPProxy{}, containers))
}

func TestWithProxyEnvDefaultsAndPreservesExisting(t *testing.T) {
	proxy := &HTTPProxy{HTTPProxy: "http://proxy:3128", NoProxy: "localhost,.svc"}
	containers := []v1.Container{
		{Name: "plain"},
		{Name: "custom", Env: []v1.EnvVar{{Name: "https_proxy", Value: "http://other:8080"}}},
	}

	output := withProxyEnv(proxy, containers)

	require.Len(t, output, 2)
	assert.Equal(t, map[string]string{
		"HTTP_PROXY":  "http://proxy:3128",
		"http_proxy":  "http://proxy:3128",
		"HTTPS_PROXY": "http://proxy:3128",
		"https_proxy": "http://proxy:3128",
		"NO_PROXY":    "localhost,.svc",
		"no_proxy":    "localhost,.svc",
	}, envMap(output[0].Env))
	custom := envMap(output[1].Env)
	assert.Equal(t, "http://other:8080", custom["https_proxy"])
	assert.Equal(t, "http://proxy:3128", custom["HTTPS_PROXY"])
	assert.Len(t, output[1].Env, 6)
	// The input is not modified
	assert.Empty(t, containers[0].Env)
	assert.Len(t, containers[1].Env, 1)
}

func TestWithProxyEnvNoNoProxy(t *testing.T) {
	proxy := &HTTPProxy{HTTPProxy: "http://proxy:3128", HTTPSProxy: "http://secure-proxy:3129"}
	output := withProxyEnv(proxy, []v1.Container{{Name: "c"}})
	env := envMap(output[0].Env)
	assert.Equal(t, "http://secure-proxy:3129", env["HTTPS_PROXY"])
	assert.NotContains(t, env, "NO_PROXY")
	assert.NotContains(t, env, "no_proxy")
}

func TestPodProxyEnvAllContainers(t *testing.T) {
	session := AmaltheaSession{
		ObjectMeta: metav1.ObjectMeta{Name: "session", Namespace: "ns"},
		Spec: AmaltheaSessionSpec{
			Session: Session{
				Image: "image",
				Env:   []v1.EnvVar{{Name: "RENKU_X", Value: "1"}},
			},
			Authentication: &Authentication{
				Enabled:   true,
				Type:      Token,
				SecretRef: SessionSecretRef{Name: "auth", Key: "key"},
			},
			CodeRepositories:    []CodeRepository{{Remote: "https://example.com/repo.git"}},
			ExtraContainers:     []v1.Container{{Name: "extra"}},
			ExtraInitContainers: []v1.Container{{Name: "extra-init"}},
			Proxy:               &HTTPProxy{HTTPProxy: "http://proxy:3128", NoProxy: "localhost"},
		},
	}

	pod, err := session.Pod(config.AmaltheaSessionConfiguration{})
	require.NoError(t, err)

	all := append(append([]v1.Container{}, pod.Containers...), pod.InitContainers...)
	require.GreaterOrEqual(t, len(pod.Containers), 3)
	require.GreaterOrEqual(t, len(pod.InitContainers), 2)
	for _, c := range all {
		env := envMap(c.Env)
		assert.Equal(t, "http://proxy:3128", env["HTTPS_PROXY"], "container %s", c.Name)
		assert.Equal(t, "localhost", env["no_proxy"], "container %s", c.Name)
	}
	// The session spec is not modified
	assert.Len(t, session.Spec.Session.Env, 1)
	assert.Empty(t, session.Spec.ExtraContainers[0].Env)
	assert.Empty(t, session.Spec.ExtraInitContainers[0].Env)
}

func TestPodWithoutProxy(t *testing.T) {
	session := AmaltheaSession{
		ObjectMeta: metav1.ObjectMeta{Name: "session", Namespace: "ns"},
		Spec: AmaltheaSessionSpec{
			Session:         Session{Image: "image"},
			ExtraContainers: []v1.Container{{Name: "extra"}},
		},
	}
	pod, err := session.Pod(config.AmaltheaSessionConfiguration{})
	require.NoError(t, err)
	for _, c := range pod.Containers {
		assert.NotContains(t, envMap(c.Env), "HTTP_PROXY", "container %s", c.Name)
	}
}
