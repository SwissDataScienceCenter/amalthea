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

package tunnel

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTunnelListenWorkerThreadsFlag(t *testing.T) {
	cmd, err := Command()
	require.NoError(t, err)

	flag := cmd.PersistentFlags().Lookup(wstunnelWorkerThreadsFlag)
	require.NotNil(t, flag, "tunnel listen must accept --%s", wstunnelWorkerThreadsFlag)
	assert.Equal(t, "0", flag.DefValue, "0 keeps the previous behaviour: let wstunnel use the CPU count")
}

// The tunnel container sets the worker count through the environment, so this is the binding that
// actually reaches the tunnel: if the env name drifts, the container silently runs unbounded again.
func TestTunnelListenWorkerThreadsFromEnv(t *testing.T) {
	t.Setenv("WSTUNNEL_WORKER_THREADS", "2")

	_, err := Command()
	require.NoError(t, err)

	assert.Equal(t, uint(2), viper.GetUint(wstunnelPrefix+"."+wstunnelWorkerThreadsFlag))
}
