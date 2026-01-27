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
	firecrestAPIURLFlag     = "firecrest-api-url"
	firecrestSystemNameFlag = "firecrest-system-name"
	firecrestPartitionFlag  = "firecrest-partition"
)

type FirecrestConfig struct {
	// The URL of the FirecREST API
	APIURL string
	// The system name for FirecREST
	SystemName string
	// The partition to use for FirecREST (SLURM option)
	Partition string
	// The configuration used to authenticate with the FirecREST API
	AuthConfig FirecrestAuthConfig
}

func SetFlags(cmd *cobra.Command) error {
	cmd.Flags().String(firecrestAPIURLFlag, "", "URL of the FirecREST API")
	if err := viper.BindPFlag(firecrestAPIURLFlag, cmd.Flags().Lookup(firecrestAPIURLFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(firecrestAPIURLFlag, configUtils.AsEnvVarFlag(firecrestAPIURLFlag)); err != nil {
		return err
	}

	cmd.Flags().String(firecrestSystemNameFlag, "", "system name for FirecREST")
	if err := viper.BindPFlag(firecrestSystemNameFlag, cmd.Flags().Lookup(firecrestSystemNameFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(firecrestSystemNameFlag, configUtils.AsEnvVarFlag(firecrestSystemNameFlag)); err != nil {
		return err
	}

	cmd.Flags().String(firecrestPartitionFlag, "", "partition to use for FirecREST (SLURM option)")
	if err := viper.BindPFlag(firecrestPartitionFlag, cmd.Flags().Lookup(firecrestPartitionFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(firecrestPartitionFlag, configUtils.AsEnvVarFlag(firecrestPartitionFlag)); err != nil {
		return err
	}

	// Set up auth flags
	if err := SetAuthFlags(cmd); err != nil {
		return err
	}

	return nil
}

func GetConfig() (cfg FirecrestConfig) {
	cfg = FirecrestConfig{}
	cfg.APIURL = viper.GetString(firecrestAPIURLFlag)
	cfg.SystemName = viper.GetString(firecrestSystemNameFlag)
	cfg.Partition = viper.GetString(firecrestPartitionFlag)

	firecrestAuthConfig := GetAuthConfig()
	cfg.AuthConfig = firecrestAuthConfig

	return cfg
}

func (cfg *FirecrestConfig) Validate() error {
	if cfg.APIURL == "" {
		return fmt.Errorf("firecrest.APIURL is not defined")
	}
	if _, err := url.Parse(cfg.APIURL); err != nil {
		return fmt.Errorf("firecrest.APIURL is not valid: %w", err)
	}
	if cfg.SystemName == "" {
		return fmt.Errorf("firecrest.SystemName is not defined")
	}
	if err := cfg.AuthConfig.Validate(); err != nil {
		return err
	}
	return nil
}
