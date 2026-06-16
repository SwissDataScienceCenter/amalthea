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

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	firecrestConfig "github.com/SwissDataScienceCenter/amalthea/internal/remote/config/firecrest"
	runaiConfig "github.com/SwissDataScienceCenter/amalthea/internal/remote/config/runai"
	configUtils "github.com/SwissDataScienceCenter/amalthea/internal/remote/config/utils"
)

type RemoteKind string

const (
	RemoteKindFirecrest RemoteKind = "firecrest"
	RemoteKindRunai     RemoteKind = "runai"
)

const (
	serverPortFlag         = "server-port"
	fakeStartFlag          = "fake-start"
	sessionPortFlag        = "session-port"
	sessionURLPathFlag     = "session-url-path"
	readinessProbeTypeFlag = "readiness-probe-type"
)

type RemoteSessionControllerConfig struct {

	// The type of remote infrastructure to use, currently FirecREST or Runai
	RemoteKind RemoteKind

	// The configuration for the FirecREST API
	Firecrest firecrestConfig.FirecrestConfig
	Runai     runaiConfig.RunaiConfig

	// The port the server will listen to
	ServerPort int32

	// FakeStart if true, do not start the remote session and print debug information
	FakeStart bool

	// SessionPort is the port where the remote session is expected to be serving
	SessionPort int32

	// SessionURLPath is the URL path for the HTTP readiness probe
	SessionURLPath string

	// ReadinessProbeType is "none", "tcp", or "http"
	ReadinessProbeType string
}

func SetFlags(cmd *cobra.Command) error {
	cmd.Flags().Int32(serverPortFlag, amaltheadevv1alpha1.RemoteSessionControllerPort, "port to listen to")
	if err := viper.BindPFlag(serverPortFlag, cmd.Flags().Lookup(serverPortFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(serverPortFlag, configUtils.AsEnvVarFlag(serverPortFlag)); err != nil {
		return err
	}

	cmd.Flags().Bool(fakeStartFlag, false, "will not start the session if set")
	if err := viper.BindPFlag(fakeStartFlag, cmd.Flags().Lookup(fakeStartFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(fakeStartFlag, configUtils.AsEnvVarFlag(fakeStartFlag)); err != nil {
		return err
	}

	cmd.Flags().Int32(sessionPortFlag, 0, "port the remote session is expected to be serving on")
	if err := viper.BindPFlag(sessionPortFlag, cmd.Flags().Lookup(sessionPortFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(sessionPortFlag, configUtils.AsEnvVarFlag(sessionPortFlag)); err != nil {
		return err
	}

	cmd.Flags().String(sessionURLPathFlag, "/", "URL path for the HTTP readiness probe")
	if err := viper.BindPFlag(sessionURLPathFlag, cmd.Flags().Lookup(sessionURLPathFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(sessionURLPathFlag, configUtils.AsEnvVarFlag(sessionURLPathFlag)); err != nil {
		return err
	}

	cmd.Flags().String(readinessProbeTypeFlag, "none", "readiness probe type: none, tcp, or http")
	if err := viper.BindPFlag(readinessProbeTypeFlag, cmd.Flags().Lookup(readinessProbeTypeFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(readinessProbeTypeFlag, configUtils.AsEnvVarFlag(readinessProbeTypeFlag)); err != nil {
		return err
	}

	// Set up shared flags
	if err := configUtils.SetFlags(cmd); err != nil {
		return err
	}

	// Set up firecREST flags
	if err := firecrestConfig.SetFlags(cmd); err != nil {
		return err
	}

	// Set up RunAI flags
	if err := runaiConfig.SetFlags(cmd); err != nil {
		return err
	}

	return nil
}

func GetConfig() (cfg RemoteSessionControllerConfig, err error) {
	// This only gets the config, but does not validate it
	cfg.Firecrest = firecrestConfig.GetConfig()
	cfg.Runai = runaiConfig.GetConfig()

	cfg.ServerPort = viper.GetInt32(serverPortFlag)
	cfg.FakeStart = viper.GetBool(fakeStartFlag)
	cfg.SessionPort = viper.GetInt32(sessionPortFlag)
	cfg.SessionURLPath = viper.GetString(sessionURLPathFlag)
	cfg.ReadinessProbeType = viper.GetString(readinessProbeTypeFlag)

	return cfg, nil
}

func (cfg *RemoteSessionControllerConfig) Validate() error {
	if cfg.ReadinessProbeType != string(amaltheadevv1alpha1.None) &&
		cfg.ReadinessProbeType != string(amaltheadevv1alpha1.TCP) &&
		cfg.ReadinessProbeType != string(amaltheadevv1alpha1.HTTP) {
		return fmt.Errorf("invalid readiness probe type: %s", cfg.ReadinessProbeType)
	}

	// FireCREST has priority over Runai
	cfg.RemoteKind = RemoteKindFirecrest
	firecrestConfigErr := cfg.Firecrest.Validate()
	if firecrestConfigErr == nil {
		return nil
	}
	runaiConfigErr := cfg.Runai.Validate()
	if runaiConfigErr != nil {
		return firecrestConfigErr
	}

	// The FirecREST config is not valid, but the Runai config is valid, so use Runai
	cfg.RemoteKind = RemoteKindRunai
	return nil
}
