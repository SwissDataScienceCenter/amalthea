package common

import (
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
