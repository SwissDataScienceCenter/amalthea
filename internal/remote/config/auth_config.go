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

package config

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	authPrefix                = "auth"
	authKindFlag              = "auth-kind"
	tokenURIFlag              = "token-uri"
	renkuAccessTokenFlag      = "renku-access-token"
	renkuRefreshTokenFlag     = "renku-refresh-token"
	renkuTokenURIFlag         = "renku-token-uri"
	renkuClientIDFlag         = "renku-client-id"
	renkuClientSecretFlag     = "renku-client-secret"
	firecrestClientIDFlag     = "firecrest-client-id"
	firecrestClientSecretFlag = "firecrest-client-secret"
)

// FirecrestAuthConfig defines the configuration of the authentication scheme
// used to access the FirecREST API
type FirecrestAuthConfig struct {
	// The kind of authentication scheme to use
	Kind FirecrestAuthConfigKind
	// The URI used to issue new tokens
	TokenURI string

	// The Renku access token (renku auth)
	RenkuAccessToken RedactedString
	// The Renku refresh token (renku auth)
	RenkuRefreshToken RedactedString
	// The URI used to issue new renku tokens (renku auth)
	RenkuTokenURI string
	// The Renku client ID (renku auth)
	RenkuClientID string
	// The Renku client secret (renku auth)
	RenkuClientSecret RedactedString

	// The FirecREST client ID (client credentials auth)
	FirecrestClientID string
	// The FirecREST client secret (client credentials auth)
	FirecrestClientSecret RedactedString
}

type FirecrestAuthConfigKind string

const FirecrestAuthConfigKindRenku = "renku"
const FirecrestAuthConfigKindClientCredentials = "client_credentials"

// Validate checks that the authentication config is valid
func (cfg *FirecrestAuthConfig) Validate() error {
	if cfg.Kind == "" {
		return fmt.Errorf("kind is not defined")
	}
	if cfg.Kind == FirecrestAuthConfigKindRenku {
		return cfg.validateRenku()
	}
	if cfg.Kind == FirecrestAuthConfigKindClientCredentials {
		return cfg.validateClientCredentials()
	}
	return fmt.Errorf("auth '%s' is not supported", cfg.Kind)
}

func (cfg *FirecrestAuthConfig) validateRenku() error {
	if cfg.TokenURI == "" {
		return fmt.Errorf("tokenURI is not defined")
	}
	if _, err := url.Parse(cfg.TokenURI); err != nil {
		return fmt.Errorf("tokenURI is not valid: %w", err)
	}
	if cfg.RenkuRefreshToken == "" {
		return fmt.Errorf("renkuRefreshToken is not defined")
	}
	if cfg.RenkuTokenURI == "" {
		return fmt.Errorf("renkuTokenURI is not defined")
	}
	if _, err := url.Parse(cfg.RenkuTokenURI); err != nil {
		return fmt.Errorf("renkuTokenURI is not valid: %w", err)
	}
	if cfg.RenkuClientID == "" {
		return fmt.Errorf("renkuClientID is not defined")
	}
	if cfg.RenkuClientSecret == "" {
		return fmt.Errorf("renkuClientSecret is not defined")
	}
	return nil
}

func (cfg *FirecrestAuthConfig) validateClientCredentials() error {
	if cfg.TokenURI == "" {
		return fmt.Errorf("tokenURI is not defined")
	}
	if _, err := url.Parse(cfg.TokenURI); err != nil {
		return fmt.Errorf("tokenURI is not valid: %w", err)
	}
	if cfg.FirecrestClientID == "" {
		return fmt.Errorf("firecrestClientID is not defined")
	}
	if cfg.FirecrestClientSecret == "" {
		return fmt.Errorf("firecrestClientSecret is not defined")
	}
	return nil
}

func SetAuthFlags(cmd *cobra.Command) error {
	cmd.Flags().String(authKindFlag, "", "the kind of authentication to use ('renku' or 'client_credentials')")
	if err := viper.BindPFlag(authKindFlag, cmd.Flags().Lookup(authKindFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authKindFlag, AsEnvVarFlag(authKindFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+tokenURIFlag, "", "the URI used to issue new tokens to authenticate with FirecREST")
	if err := viper.BindPFlag(authPrefix+"."+tokenURIFlag, cmd.Flags().Lookup(authPrefix+"-"+tokenURIFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+tokenURIFlag, AsEnvVarFlag(authPrefix+"-"+tokenURIFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+renkuAccessTokenFlag, "", "the Renku access token (renku auth)")
	if err := viper.BindPFlag(authPrefix+"."+renkuAccessTokenFlag, cmd.Flags().Lookup(authPrefix+"-"+renkuAccessTokenFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+renkuAccessTokenFlag, AsEnvVarFlag(authPrefix+"-"+renkuAccessTokenFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+renkuRefreshTokenFlag, "", "the Renku refresh token (renku auth)")
	if err := viper.BindPFlag(authPrefix+"."+renkuRefreshTokenFlag, cmd.Flags().Lookup(authPrefix+"-"+renkuRefreshTokenFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+renkuRefreshTokenFlag, AsEnvVarFlag(authPrefix+"-"+renkuRefreshTokenFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+renkuTokenURIFlag, "", "the URI used to issue new renku tokens (renku auth)")
	if err := viper.BindPFlag(authPrefix+"."+renkuTokenURIFlag, cmd.Flags().Lookup(authPrefix+"-"+renkuTokenURIFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+renkuTokenURIFlag, AsEnvVarFlag(authPrefix+"-"+renkuTokenURIFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+renkuClientIDFlag, "", "the Renku client ID (renku auth)")
	if err := viper.BindPFlag(authPrefix+"."+renkuClientIDFlag, cmd.Flags().Lookup(authPrefix+"-"+renkuClientIDFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+renkuClientIDFlag, AsEnvVarFlag(authPrefix+"-"+renkuClientIDFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+renkuClientSecretFlag, "", "the Renku client secret (renku auth)")
	if err := viper.BindPFlag(authPrefix+"."+renkuClientSecretFlag, cmd.Flags().Lookup(authPrefix+"-"+renkuClientSecretFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+renkuClientSecretFlag, AsEnvVarFlag(authPrefix+"-"+renkuClientSecretFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+firecrestClientIDFlag, "", "the FirecREST client ID (client credentials auth)")
	if err := viper.BindPFlag(authPrefix+"."+firecrestClientIDFlag, cmd.Flags().Lookup(authPrefix+"-"+firecrestClientIDFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+firecrestClientIDFlag, AsEnvVarFlag(authPrefix+"-"+firecrestClientIDFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+firecrestClientSecretFlag, "", "the FirecREST client secret (client credentials auth)")
	if err := viper.BindPFlag(authPrefix+"."+firecrestClientSecretFlag, cmd.Flags().Lookup(authPrefix+"-"+firecrestClientSecretFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+firecrestClientSecretFlag, AsEnvVarFlag(authPrefix+"-"+firecrestClientSecretFlag)); err != nil {
		return err
	}

	return nil
}

func GetAuthConfig() (cfg FirecrestAuthConfig, err error) {
	cfg.Kind = FirecrestAuthConfigKind(viper.GetString(authKindFlag))
	cfg.TokenURI = viper.GetString(authPrefix + "." + tokenURIFlag)

	cfg.RenkuAccessToken = RedactedString(viper.GetString(authPrefix + "." + renkuAccessTokenFlag))
	cfg.RenkuRefreshToken = RedactedString(viper.GetString(authPrefix + "." + renkuRefreshTokenFlag))
	cfg.RenkuTokenURI = viper.GetString(authPrefix + "." + renkuTokenURIFlag)
	cfg.RenkuClientID = viper.GetString(authPrefix + "." + renkuClientIDFlag)
	cfg.RenkuClientSecret = RedactedString(viper.GetString(authPrefix + "." + renkuClientSecretFlag))

	cfg.FirecrestClientID = viper.GetString(authPrefix + "." + firecrestClientIDFlag)
	cfg.FirecrestClientSecret = RedactedString(viper.GetString(authPrefix + "." + firecrestClientSecretFlag))

	return cfg, nil
}
