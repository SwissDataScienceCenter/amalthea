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

// package config contains configuration utilities for the remote session controller
package config

import (
	"fmt"
	"net/url"
	"strings"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const RemoteSessionControllerPrefix = "RSC"

const (
	firecrestAPIURLFlag     = "firecrest-api-url"
	firecrestSystemNameFlag = "firecrest-system-name"
	firecrestPartitionFlag  = "firecrest-partition"
	serverPortFlag          = "server-port"
	fakeStartFlag           = "fake-start"
)

type RemoteSessionControllerConfig struct {
	// NOTE: this config struct only support using the FirecREST API for now

	// The URL of the FirecREST API
	FirecrestAPIURL string
	// The system name for FirecREST
	FirecrestSystemName string
	// The partition to use for FirecREST (SLURM option)
	FirecrestPartition string

	// The configuration used to authenticate with the FirecREST API
	FirecrestAuthConfig FirecrestAuthConfig

	// The port the server will listen to
	ServerPort int32

	// FakeStart if true, do not start the remote session and print debug information
	FakeStart bool
}

func SetFlags(cmd *cobra.Command) error {
	cmd.Flags().String(firecrestAPIURLFlag, "", "URL of the FirecREST API")
	if err := viper.BindPFlag(firecrestAPIURLFlag, cmd.Flags().Lookup(firecrestAPIURLFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(firecrestAPIURLFlag, AsEnvVarFlag(firecrestAPIURLFlag)); err != nil {
		return err
	}

	cmd.Flags().String(firecrestSystemNameFlag, "", "system name for FirecREST")
	if err := viper.BindPFlag(firecrestSystemNameFlag, cmd.Flags().Lookup(firecrestSystemNameFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(firecrestSystemNameFlag, AsEnvVarFlag(firecrestSystemNameFlag)); err != nil {
		return err
	}

	cmd.Flags().String(firecrestPartitionFlag, "", "partition to use for FirecREST (SLURM option)")
	if err := viper.BindPFlag(firecrestPartitionFlag, cmd.Flags().Lookup(firecrestPartitionFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(firecrestPartitionFlag, AsEnvVarFlag(firecrestPartitionFlag)); err != nil {
		return err
	}

	cmd.Flags().Int32(serverPortFlag, amaltheadevv1alpha1.RemoteSessionControllerPort, "port to listen to")
	if err := viper.BindPFlag(serverPortFlag, cmd.Flags().Lookup(serverPortFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(serverPortFlag, AsEnvVarFlag(serverPortFlag)); err != nil {
		return err
	}

	cmd.Flags().Bool(fakeStartFlag, false, "will not start the session if set")
	if err := viper.BindPFlag(fakeStartFlag, cmd.Flags().Lookup(fakeStartFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(fakeStartFlag, AsEnvVarFlag(fakeStartFlag)); err != nil {
		return err
	}

	// Set up auth flags
	if err := SetAuthFlags(cmd); err != nil {
		return err
	}

	return nil
}

func GetConfig() (cfg RemoteSessionControllerConfig, err error) {
	cfg.FirecrestAPIURL = viper.GetString(firecrestAPIURLFlag)
	cfg.FirecrestSystemName = viper.GetString(firecrestSystemNameFlag)
	cfg.FirecrestPartition = viper.GetString(firecrestPartitionFlag)
	cfg.ServerPort = viper.GetInt32(serverPortFlag)
	cfg.FakeStart = viper.GetBool(fakeStartFlag)

	firecrestAuthConfig, err := GetAuthConfig()
	if err != nil {
		return cfg, nil
	}
	cfg.FirecrestAuthConfig = firecrestAuthConfig

	return cfg, nil
}

func (cfg *RemoteSessionControllerConfig) Validate() error {
	if cfg.FirecrestAPIURL == "" {
		return fmt.Errorf("firecrestAPIURL is not defined")
	}
	if _, err := url.Parse(cfg.FirecrestAPIURL); err != nil {
		return fmt.Errorf("firecrestAPIURL is not valid: %w", err)
	}
	if cfg.FirecrestSystemName == "" {
		return fmt.Errorf("firecrestSystemName is not defined")
	}
	if err := cfg.FirecrestAuthConfig.Validate(); err != nil {
		return err
	}
	return nil
}

// Converts a flag into its environment variable version, with the "RSC" prefix.
//
// Example: my-flag -> RSC_MY_FLAG
func AsEnvVarFlag(flag string) string {
	withUnderscores := strings.ReplaceAll(RemoteSessionControllerPrefix+"_"+flag, "-", "_")
	return strings.ToUpper(withUnderscores)
}
