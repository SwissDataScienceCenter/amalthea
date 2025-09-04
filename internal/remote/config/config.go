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

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/spf13/viper"
)

// const RemoteSessionControllerPort int32 = 65532

type RemoteSessionControllerConfig struct {
	// NOTE: this config struct only support using the FirecREST API for now

	// The URL of the FirecREST API
	FirecrestAPIURL string `mapstructure:"firecrest_api_url"`

	// The URI of the authentication endpoint for the FirecREST API
	FirecrestAuthTokenURI string `mapstructure:"firecrest_auth_token_uri"`
	// The client ID for the FirecREST API
	FirecrestClientID RedactedString `mapstructure:"firecrest_client_id"`
	// The client secret for the FirecREST API
	FirecrestClientSecret RedactedString `mapstructure:"firecrest_client_secret"`

	// Fields for the renku auth
	RenkuAccessToken  RedactedString `mapstructure:"renku_access_token"`
	RenkuRefreshToken RedactedString `mapstructure:"renku_refresh_token"`
	RenkuTokenURI     string         `mapstructure:"renku_auth_token_uri"`
	RenkuClientID     string         `mapstructure:"renku_client_id"`
	RenkuClientSecret RedactedString `mapstructure:"renku_client_secret"`

	// The port the server will listen to
	ServerPort int `mapstructure:"server_port"`
}

func GetConfig() (cfg RemoteSessionControllerConfig, err error) {
	v := viper.New()
	v.SetConfigType("env")
	v.AutomaticEnv()

	v.SetDefault("firecrest_api_url", "")

	// Auth - Client credentials grant
	v.SetDefault("firecrest_auth_token_uri", "")
	v.SetDefault("firecrest_client_id", "")
	v.SetDefault("firecrest_client_secret", "")

	// Auth - Renku auth
	v.SetDefault("renku_access_token", "")
	v.SetDefault("renku_refresh_token", "")
	v.SetDefault("renku_auth_token_uri", "")
	v.SetDefault("renku_client_id", "")
	v.SetDefault("renku_client_secret", "")

	v.SetDefault("server_port", amaltheadevv1alpha1.RemoteSessionControllerPort)

	if err := v.Unmarshal(&cfg); err != nil {
		return RemoteSessionControllerConfig{}, err
	}
	return cfg, nil
}

func (cfg *RemoteSessionControllerConfig) Validate() error {
	if cfg.FirecrestAPIURL == "" {
		return fmt.Errorf("FirecrestAPIURL is not defined")
	}
	if _, err := url.Parse(cfg.FirecrestAPIURL); err != nil {
		return fmt.Errorf("FirecrestAPIURL is not valid: %w", err)
	}

	// if cfg.FirecrestAuthTokenURI == "" {
	// 	return fmt.Errorf("FirecrestAuthTokenURI is not defined")
	// }
	// if _, err := url.Parse(cfg.FirecrestAuthTokenURI); err != nil {
	// 	return fmt.Errorf("FirecrestAuthTokenURI is not valid: %w", err)
	// }
	// if cfg.FirecrestClientID == "" {
	// 	return fmt.Errorf("FirecrestClientID is not defined")
	// }
	// if cfg.FirecrestClientSecret == "" {
	// 	return fmt.Errorf("FirecrestClientSecret is not defined")
	// }

	return nil
}
