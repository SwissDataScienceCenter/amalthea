package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
)

// envVars returns the proxy environment variables, in upper and lower case
// because clients disagree on which one they read and which one wins.
func (p *HTTPProxy) envVars() []v1.EnvVar {
	httpsProxy := p.HTTPSProxy
	if httpsProxy == "" {
		httpsProxy = p.HTTPProxy
	}
	env := []v1.EnvVar{
		{Name: "HTTP_PROXY", Value: p.HTTPProxy},
		{Name: "http_proxy", Value: p.HTTPProxy},
		{Name: "HTTPS_PROXY", Value: httpsProxy},
		{Name: "https_proxy", Value: httpsProxy},
	}
	if p.NoProxy != "" {
		env = append(env,
			v1.EnvVar{Name: "NO_PROXY", Value: p.NoProxy},
			v1.EnvVar{Name: "no_proxy", Value: p.NoProxy},
		)
	}
	return env
}

// withProxyEnv returns copies of the containers with the proxy environment variables added.
// Variables that a container already defines are left untouched.
// The input slice and containers are not modified, since they may belong to the AmaltheaSession spec.
func withProxyEnv(proxy *HTTPProxy, containers []v1.Container) []v1.Container {
	if proxy == nil || proxy.HTTPProxy == "" || len(containers) == 0 {
		return containers
	}
	proxyEnv := proxy.envVars()
	output := make([]v1.Container, len(containers))
	for i, container := range containers {
		existing := make(map[string]struct{}, len(container.Env))
		for _, env := range container.Env {
			existing[env.Name] = struct{}{}
		}
		newEnv := make([]v1.EnvVar, 0, len(container.Env)+len(proxyEnv))
		newEnv = append(newEnv, container.Env...)
		for _, env := range proxyEnv {
			if _, found := existing[env.Name]; !found {
				newEnv = append(newEnv, env)
			}
		}
		output[i] = container
		output[i].Env = newEnv
	}
	return output
}
