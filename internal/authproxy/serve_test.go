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

func TestCheckAuthConfigFailsClosed(t *testing.T) {
	t.Cleanup(func() { config = ""; token = "" })

	config = "/etc/authproxy/config"
	token = ""
	require.Error(t, checkAuthConfig())

	token = "super-secret"
	require.NoError(t, checkAuthConfig())

	config = ""
	token = ""
	require.NoError(t, checkAuthConfig())
}
