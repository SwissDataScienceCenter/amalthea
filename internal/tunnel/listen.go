/*
Copyright 2025.

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
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Restriction struct {
	Name  string       `yaml:"name"`
	Match []*yaml.Node `yaml:"match"`
	Allow []*yaml.Node `yaml:"allow"`
}

type Config struct {
	Restrictions []Restriction `yaml:"restrictions"`
}

const (
	wstunnelSecretFlag   = "secret"
	wstunnelPortFlag     = "port"
	wstunnelLogLevelFlag = "log-level"
	wstunnelPrefix       = "wstunnel"
)

func listen(cmd *cobra.Command, args []string) error {
	wstunnelSecret := viper.GetString(wstunnelPrefix + "." + wstunnelSecretFlag)
	if wstunnelSecret == "" {
		return fmt.Errorf("wstunnel secret is not set, use --secret or WSTUNNEL_SECRET environment variable")
	}

	wstunnelPort := viper.GetString(wstunnelPrefix + "." + wstunnelPortFlag)
	if wstunnelPort == "" {
		wstunnelPort = "5050"
	}

	wstunnelLogLevel := viper.GetString(wstunnelPrefix + "." + wstunnelLogLevelFlag)
	if wstunnelLogLevel == "" {
		wstunnelLogLevel = "INFO"
	}

	matchNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!Authorization",
		Value: fmt.Sprintf("^[Bb]earer +%s$", wstunnelSecret),
	}

	tunnelNode := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!Tunnel",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "protocol"},
			{Kind: yaml.SequenceNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "Tcp"}}},
		},
	}

	reverseTunnelNode := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!ReverseTunnel",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "protocol"},
			{Kind: yaml.SequenceNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "Tcp"}}},
		},
	}

	configData := Config{
		Restrictions: []Restriction{
			{
				Name: "My config",
				Match: []*yaml.Node{
					matchNode,
				},
				Allow: []*yaml.Node{
					tunnelNode,
					reverseTunnelNode,
				},
			},
		},
	}

	yamlContent, err := yaml.Marshal(configData)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML config: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "wstunnel-restrict-config-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.Write(yamlContent); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write YAML to temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	wstunnelBin := "wstunnel"

	wstunnelAddr := fmt.Sprintf("ws://0.0.0.0:%s", wstunnelPort)

	wstunnelArgs := []string{
		"server",
		"--restrict-config", tmpFile.Name(),
		"--log-lvl", wstunnelLogLevel,
		wstunnelAddr,
	}

	cmd.Printf("Executing wstunnel: %s %s\n", wstunnelBin, strings.Join(wstunnelArgs, " "))

	wCmd := exec.Command(wstunnelBin, wstunnelArgs...)
	wCmd.Stdout = os.Stdout
	wCmd.Stderr = os.Stderr

	if err := wCmd.Run(); err != nil {
		return fmt.Errorf("wstunnel command failed: %w", err)
	}

	return nil
}
