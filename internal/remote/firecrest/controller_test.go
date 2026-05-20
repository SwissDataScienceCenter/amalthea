package firecrest

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestBranchRegExp(t *testing.T) {
	line := "[branch \"main\"]"
	line = strings.TrimSpace(line)
	res := branchRegExp.FindStringSubmatch(line)
	assert.Len(t, res, 2)
	assert.Equal(t, "[branch \"main\"]", res[0])
	assert.Equal(t, "main", res[1])
}

func TestRenderSessionScriptStatic(t *testing.T) {
	partition := "my-partition"
	fileSystems := []FileSystem{
		{
			DataType:       "apps",
			DefaultWorkDir: ptr.To(true),
			Path:           "/apps",
		},
		{
			DataType:       "archive",
			DefaultWorkDir: ptr.To(false),
			Path:           "/archive",
		},
		{
			DataType:       "project",
			DefaultWorkDir: ptr.To(false),
			Path:           "/project",
		},
		{
			DataType:       "scratch",
			DefaultWorkDir: ptr.To(true),
			Path:           "/scratch",
		},
		{
			DataType:       "store",
			DefaultWorkDir: ptr.To(false),
			Path:           "/store",
		},
		{
			DataType:       "users",
			DefaultWorkDir: ptr.To(false),
			Path:           "/users",
		},
	}
	secretsPath := "/secrets"

	sessionScriptFinal := renderSessionScriptStatic(sessionScript, partition, &fileSystems, secretsPath)

	// Check that the rendered script starts with "#!/bin/bash"
	assert.Regexp(t, regexp.MustCompile("^#!/bin/bash"), sessionScriptFinal)

	// Check the SBATCH directives
	assert.Contains(t, sessionScriptFinal, "#SBATCH --nodes=1")
	assert.Contains(t, sessionScriptFinal, "#SBATCH --ntasks-per-node=1")
	assert.Contains(t, sessionScriptFinal, "#SBATCH --partition=my-partition")

	// Check the mounts
	mountsRegExp := regexp.MustCompile(`mounts(?:\s*)=(?:\s*)[[]([^]]*)]`)
	matches := mountsRegExp.FindStringSubmatch(sessionScriptFinal)
	assert.Len(t, matches, 2)
	foundMounts := matches[1]
	assert.Contains(t, foundMounts, "\"/apps:/apps\"")
	assert.Contains(t, foundMounts, "\"/archive:/archive\"")
	assert.Contains(t, foundMounts, "\"/project:/project\"")
	assert.Contains(t, foundMounts, "\"/scratch:/scratch\"")
	assert.Contains(t, foundMounts, "\"/store:/store\"")
	assert.Contains(t, foundMounts, "\"/users/${USER}:/home/users/${USER}:ro\"")
	assert.Contains(t, foundMounts, "\"/secrets:/secrets:ro\"")
}
