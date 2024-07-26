/*
Copyright 2024.

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

package cmd_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"os/exec"

	"github.com/stretchr/testify/assert"
)

var (
	AuthProxyPath = "../bin/authproxy"
)

func runCommandWithContext(ctx context.Context, t *testing.T, cmd, args string, env ...string) (bytes.Buffer, error) {
	var combinedOutput bytes.Buffer

	command := exec.CommandContext(ctx, cmd, strings.Fields(args)...)
	command.Stdout = &combinedOutput
	command.Stderr = &combinedOutput

	for _, v := range env {
		command.Env = append(command.Env, v)
	}

	t.Logf("Running %q with env: %v\n", command.String(), command.Env)

	return combinedOutput, command.Run()
}

func TestVersion(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	output, err := runCommandWithContext(ctx, t, AuthProxyPath, "version")
	assert.Nil(err, nil, "version command failed")
	out := output.String()

	assert.NotContains(out, "development")
}
