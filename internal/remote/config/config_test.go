/*
Copyright 2026.

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

package config

import (
	"testing"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		expectedKind RemoteKind
		wantErr      bool
	}{
		{
			name: "firecrest with client credentials",
			args: []string{
				"--firecrest-api-url=https://firecrest.example.com",
				"--firecrest-system-name=test-system",
				"--auth-kind=client_credentials",
				"--auth-token-uri=https://auth.example.com/token",
				"--auth-firecrest-client-id=my-client",
				"--auth-firecrest-client-secret=my-secret",
			},
			expectedKind: RemoteKindFirecrest,
		},
		{
			name: "runai with client credentials",
			args: []string{
				"--runai-base-url=https://runai.example.com",
				"--runai-project=my-project",
				"--auth-kind=client_credentials",
				"--auth-token-uri=https://runai-auth.example.com/token",
				"--auth-runai-client-id=runai-client",
				"--auth-runai-client-secret=runai-secret",
			},
			expectedKind: RemoteKindRunai,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()

			cmd := &cobra.Command{
				Use: "test",
				Run: func(cmd *cobra.Command, args []string) {},
			}
			err := SetFlags(cmd)
			require.NoError(t, err)

			cmd.SetArgs(tt.args)
			err = cmd.Execute()
			require.NoError(t, err)

			cfg, err := GetConfig()
			require.NoError(t, err)

			err = cfg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedKind, cfg.RemoteKind)
		})
	}
}

func TestConfigNewFlags(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		env             map[string]string
		wantProbeType   string
		wantSessionPort int32
		wantURLPath     string
		wantErr         bool
	}{
		{
			name: "default with empty readiness probe",
			args: []string{
				"--firecrest-api-url=https://firecrest.example.com",
				"--firecrest-system-name=test-system",
				"--auth-kind=client_credentials",
				"--auth-token-uri=https://auth.example.com/token",
				"--auth-firecrest-client-id=my-client",
				"--auth-firecrest-client-secret=my-secret",
			},
			wantProbeType:   string(amaltheadevv1alpha1.None),
			wantSessionPort: 0,
			wantURLPath:     "/",
		},
		{
			name: "custom flags via CLI",
			args: []string{
				"--readiness-probe-type=tcp",
				"--session-port=8888",
				"--session-url-path=/lab",
				"--runai-base-url=https://runai.example.com",
				"--runai-project=my-project",
				"--auth-kind=client_credentials",
				"--auth-token-uri=https://runai-auth.example.com/token",
				"--auth-runai-client-id=runai-client",
				"--auth-runai-client-secret=runai-secret",
			},
			wantProbeType:   string(amaltheadevv1alpha1.TCP),
			wantSessionPort: 8888,
			wantURLPath:     "/lab",
		},
		{
			name: "custom flags via environment variables",
			args: []string{
				"--firecrest-api-url=https://firecrest.example.com",
				"--firecrest-system-name=test-system",
				"--auth-kind=client_credentials",
				"--auth-token-uri=https://auth.example.com/token",
				"--auth-firecrest-client-id=my-client",
				"--auth-firecrest-client-secret=my-secret",
			},
			env: map[string]string{
				"RSC_READINESS_PROBE_TYPE": "http",
				"RSC_SESSION_PORT":         "9999",
				"RSC_SESSION_URL_PATH":     "/jupyter",
			},
			wantProbeType:   string(amaltheadevv1alpha1.HTTP),
			wantSessionPort: 9999,
			wantURLPath:     "/jupyter",
		},
		{
			name: "invalid readiness probe type errors",
			args: []string{
				"--readiness-probe-type=invalid",
				"--firecrest-api-url=https://firecrest.example.com",
				"--firecrest-system-name=test-system",
				"--auth-kind=client_credentials",
				"--auth-token-uri=https://auth.example.com/token",
				"--auth-firecrest-client-id=my-client",
				"--auth-firecrest-client-secret=my-secret",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cmd := &cobra.Command{
				Use: "test",
				Run: func(cmd *cobra.Command, args []string) {},
			}
			err := SetFlags(cmd)
			require.NoError(t, err)

			cmd.SetArgs(tt.args)
			err = cmd.Execute()
			require.NoError(t, err)

			cfg, err := GetConfig()
			require.NoError(t, err)

			err = cfg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantProbeType, cfg.ReadinessProbeType)
			assert.Equal(t, tt.wantSessionPort, cfg.SessionPort)
			assert.Equal(t, tt.wantURLPath, cfg.SessionURLPath)
		})
	}
}
