package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRcloneArgsFromJSON(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		flags    map[string]string
		expected string
	}{
		{"vfs cache mode", `{"CacheMode": "full"}`, vfsOptFlags, "--vfs-cache-mode=full"},
		{"read only has no vfs prefix", `{"ReadOnly": true}`, vfsOptFlags, "--read-only=true"},
		{"chunk size maps to read-chunk-size", `{"ChunkSize": "128Mi"}`, vfsOptFlags, "--vfs-read-chunk-size=128Mi"},
		{"dir cache time has no vfs prefix", `{"DirCacheTime": "60s"}`, vfsOptFlags, "--dir-cache-time=60s"},
		{"mount attr timeout", `{"AttrTimeout": "41s"}`, mountOptFlags, "--attr-timeout=41s"},
		{"array repeats the flag", `{"ExtraOptions": ["a", "b"]}`, mountOptFlags, "--option=a --option=b"},
		{"integer keeps exact form", `{"UID": 1000}`, vfsOptFlags, "--uid=1000"},
		{"multiple keys are sorted", `{"DirCacheTime": "60s", "ChunkSize": "128Mi"}`, vfsOptFlags, "--vfs-read-chunk-size=128Mi --dir-cache-time=60s"},
		{"empty object", `{}`, vfsOptFlags, ""},
		{"empty string", ``, vfsOptFlags, ""},
		{"unknown option is skipped", `{"Nope": "x"}`, vfsOptFlags, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rcloneArgsFromJSON(tt.raw, tt.flags)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestRcloneArgsFromJSONInvalid(t *testing.T) {
	_, err := rcloneArgsFromJSON(`not json`, vfsOptFlags)
	require.Error(t, err)
}

func TestRcloneArgsFromJSONMountCaseInsensitive(t *testing.T) {
	got, err := rcloneArgsFromJSON(`{"CaseInsensitive": true}`, mountOptFlags)
	require.NoError(t, err)
	assert.Equal(t, "--mount-case-insensitive=true", got)
}

func writeDataConnectorFixture(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, name), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, name, "configData"), []byte("[my-s3]\ntype = s3\n"), 0o600))
	return root
}

func TestConfigFilesEmitsRcloneArgs(t *testing.T) {
	root := writeDataConnectorFixture(t, "dc")
	dc := &DataConnector{
		Root:       root,
		Name:       "dc",
		Remote:     "my-s3",
		RemotePath: "projectname",
		MountOpt:   `{"AttrTimeout": "41s"}`,
		VfsOpt:     `{"CacheMode": "full"}`,
	}
	files, err := dc.ConfigFiles()
	require.NoError(t, err)
	assert.Equal(t, "--attr-timeout=41s", string(files["mountArgs"]))
	assert.Equal(t, "--vfs-cache-mode=full", string(files["vfsArgs"]))
}

func TestConfigFilesOmitsArgsWhenEmptyOrMalformed(t *testing.T) {
	for _, tt := range []struct {
		name     string
		mountOpt string
		vfsOpt   string
	}{
		{"empty", "", ""},
		{"unknown key only", "", `{"Nope": "x"}`},
		{"malformed mount opt", `not json`, ""},
		{"malformed vfs opt", "", `not json`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := writeDataConnectorFixture(t, "dc")
			dc := &DataConnector{Root: root, Name: "dc", Remote: "my-s3", RemotePath: "projectname", MountOpt: tt.mountOpt, VfsOpt: tt.vfsOpt}
			files, err := dc.ConfigFiles()
			require.NoError(t, err)
			assert.NotContains(t, files, "mountArgs")
			assert.NotContains(t, files, "vfsArgs")
		})
	}
}
