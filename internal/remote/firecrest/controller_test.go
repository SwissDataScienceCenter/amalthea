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
	assert.Contains(t, foundMounts, "\"/scratch\"")
	assert.Contains(t, foundMounts, "\"/store\"")
	assert.Contains(t, foundMounts, "\"/users:/home/users:ro\"")
	assert.Contains(t, foundMounts, "\"/secrets:/secrets:ro\"")

	t.Run("resources provided", func(t *testing.T) {
		t.Setenv("RSC_SESSION_CPU", "2")
		t.Setenv("RSC_SESSION_MEMORY", "2048")
		t.Setenv("RSC_SESSION_GPUS", "1")

		script := renderSessionScriptStatic(sessionScript, partition, &fileSystems, secretsPath)
		assert.Contains(t, script, "#SBATCH --cpus-per-task=2")
		assert.Contains(t, script, "#SBATCH --mem=2048M")
		assert.Contains(t, script, "#SBATCH --gpus=1")
	})

	t.Run("ignore flag omits resources", func(t *testing.T) {
		t.Setenv("RSC_SESSION_CPU", "2")
		t.Setenv("RSC_SESSION_MEMORY", "2048")
		t.Setenv("RSC_SESSION_GPUS", "1")
		t.Setenv("RSC_FIRECREST_IGNORE_RESOURCE_CLASS_VALUES", "true")

		script := renderSessionScriptStatic(sessionScript, partition, &fileSystems, secretsPath)
		assert.NotContains(t, script, "--cpus-per-task")
		assert.NotContains(t, script, "--mem")
		assert.NotContains(t, script, "--gpus")
	})

	t.Run("only cpu provided", func(t *testing.T) {
		t.Setenv("RSC_SESSION_CPU", "4")
		t.Setenv("RSC_SESSION_MEMORY", "")
		t.Setenv("RSC_SESSION_GPUS", "")

		script := renderSessionScriptStatic(sessionScript, partition, &fileSystems, secretsPath)
		assert.Contains(t, script, "#SBATCH --cpus-per-task=4")
		assert.NotContains(t, script, "--mem")
		assert.NotContains(t, script, "--gpus")
	})
}

func TestStreamsToFetch(t *testing.T) {
	tests := []struct {
		name       string
		stdoutPath string
		stderrPath string
		want       []string
	}{
		{"different paths", "/out", "/err", []string{"stdout", "stderr"}},
		{"same path", "/out", "/out", []string{"stdout"}},
		{"empty stderr", "/out", "", []string{"stdout"}},
		{"stderr explicitly eq stdout", "/out", "/out", []string{"stdout"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &FirecrestRemoteSessionController{
				stdoutPath: tt.stdoutPath,
				stderrPath: tt.stderrPath,
			}
			assert.Equal(t, tt.want, c.streamsToFetch())
		})
	}
}
