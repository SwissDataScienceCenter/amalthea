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

package main

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func mapOutput(cmd *cobra.Command, out io.Writer, errOut io.Writer) {
	if cmd == nil {
		return
	}
	if out != nil {
		cmd.SetOut(out)
	}
	if errOut != nil {
		cmd.SetErr(errOut)
	}
}

func readCloseOutput(out io.Reader, errOut io.Reader) (string, string, error) {
	var stdout, stderr []byte
	var err error
	if out != nil {
		stdout, err = io.ReadAll(out)
		if err != nil {
			return "", "", err
		}
	}
	if errOut != nil {
		stderr, err = io.ReadAll(errOut)
		if err != nil {
			return "", "", err
		}
	}
	return string(stdout), string(stderr), nil
}

func TestVersion(t *testing.T) {
	assert := assert.New(t)
	stdoutBuf := bytes.NewBufferString("")
	stderrBuf := bytes.NewBufferString("")
	cmd := buildCommands()
	mapOutput(cmd, stdoutBuf, stderrBuf)
	cmd.SetArgs([]string{"version"})
	err := cmd.Execute()
	assert.NoError(err)
	stdout, stderr, err := readCloseOutput(stdoutBuf, stderrBuf)
	assert.NoError(err)
	assert.Empty(stderr)
	assert.NotEmpty(stdout)
	fmt.Println(stdout)
	assert.Equal("sidecars (devel)\n", stdout)
}

func TestBasicAPI(t *testing.T) {
	assert := assert.New(t)
	cmd := buildCommands()
	cmd.SetArgs([]string{"notexist", "test", "--help"})
	err := cmd.Execute()
	assert.Error(err)
	cmd = buildCommands()
	cmd.SetArgs([]string{"cloner", "clone", "--help"})
	err = cmd.Execute()
	assert.NoError(err)
	cmd = buildCommands()
	cmd.SetArgs([]string{"proxy", "serve", "--help"})
	err = cmd.Execute()
	assert.NoError(err)
	cmd.SetArgs([]string{"tunnel", "listen", "--help"})
	err = cmd.Execute()
	assert.NoError(err)
	cmd.SetArgs([]string{"gitproxy", "proxy", "--help"})
	err = cmd.Execute()
	assert.NoError(err)
}
