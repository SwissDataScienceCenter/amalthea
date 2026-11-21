package authproxy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigTopLevelKeys(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte("remote: http://localhost:8080\ntoken: super-secret\ncookie_key: my-cookie\n"), 0o600)
	require.NoError(t, err)

	cmd, err := Command()
	require.NoError(t, err)
	require.NoError(t, cmd.ParseFlags(nil))

	config = cfgPath
	token = ""
	cookieKey = ""
	t.Cleanup(func() { config = ""; token = ""; cookieKey = "" })

	require.NoError(t, loadConfig(cmd, nil))
	require.Equal(t, "super-secret", token)
	require.Equal(t, "my-cookie", cookieKey)
}

func TestCheckAuthConfig(t *testing.T) {
	t.Cleanup(func() { enableAuthn = false; token = "" })

	enableAuthn = true
	token = ""
	require.Error(t, checkAuthConfig())

	token = "super-secret"
	require.NoError(t, checkAuthConfig())

	enableAuthn = false
	token = ""
	require.NoError(t, checkAuthConfig())
}

func TestEnableAuthnFromFlagEnvAndConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("enable_authn: true\n"), 0o600))

	for name, tc := range map[string]struct {
		args []string
		env  string
	}{
		"flag":        {args: []string{"--enable_authn"}},
		"env":         {env: "true"},
		"config file": {args: []string{"--config", cfgPath}},
	} {
		t.Run(name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(func() { viper.Reset(); config = ""; enableAuthn = false; token = "" })
			enableAuthn = false
			token = ""
			if tc.env != "" {
				t.Setenv("AUTHPROXY_ENABLE_AUTHN", tc.env)
			}

			cmd, err := Command()
			require.NoError(t, err)
			require.NoError(t, cmd.ParseFlags(tc.args))
			require.NoError(t, loadConfig(cmd, nil))

			require.True(t, enableAuthn)
			require.Error(t, checkAuthConfig())
		})
	}
}
