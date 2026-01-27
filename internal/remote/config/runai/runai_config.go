/*
Copyright 2026.

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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	configUtils "github.com/SwissDataScienceCenter/amalthea/internal/remote/config/utils"
)

const (
	runaiAPIURLFlag  = "runai-api-url"
	runaiProjectFlag = "runai-project"
)

type RunaiConfig struct {
	// The URL of the Runai API
	APIURL string
	// The configuration used to authenticate with the Runai API
	AuthConfig RunaiAuthConfig
	// The Runai Project to use for running sessions
	Project string
}

func SetFlags(cmd *cobra.Command) error {
	cmd.Flags().String(runaiAPIURLFlag, "", "URL of the Runai API")
	if err := viper.BindPFlag(runaiAPIURLFlag, cmd.Flags().Lookup(runaiAPIURLFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(runaiAPIURLFlag, configUtils.AsEnvVarFlag(runaiAPIURLFlag)); err != nil {
		return err
	}

	cmd.Flags().String(runaiProjectFlag, "", "Runai project to use for running sessions")
	if err := viper.BindPFlag(runaiProjectFlag, cmd.Flags().Lookup(runaiProjectFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(runaiProjectFlag, configUtils.AsEnvVarFlag(runaiProjectFlag)); err != nil {
		return err
	}

	// Set up auth flags
	if err := SetAuthFlags(cmd); err != nil {
		return err
	}

	return nil
}

func GetConfig() (cfg RunaiConfig) {
	cfg = RunaiConfig{}
	cfg.APIURL = viper.GetString(runaiAPIURLFlag)
	cfg.Project = viper.GetString(runaiProjectFlag)

	runaiAuthConfig := GetAuthConfig(cfg.APIURL)
	cfg.AuthConfig = runaiAuthConfig
	return cfg
}

func (cfg *RunaiConfig) Validate() error {
	if cfg.APIURL == "" {
		return fmt.Errorf("runai.APIURL is not defined")
	}
	if _, err := url.Parse(cfg.APIURL); err != nil {
		return fmt.Errorf("runai.APIURL is not valid: %w", err)
	}

	if err := cfg.AuthConfig.Validate(); err != nil {
		return err
	}
	return nil
}
