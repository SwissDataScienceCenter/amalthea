package config

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	configUtils "github.com/SwissDataScienceCenter/amalthea/internal/remote/config/utils"
)

const (
	authPrefix            = "auth"
	authKindFlag          = "auth-kind"
	tokenURIFlag          = "token-uri"
	runaiClientIDFlag     = "runai-client-id"
	runaiClientSecretFlag = "runai-client-secret"
)

// RunaiAuthConfig defines the configuration of the authentication scheme
// used to access the Runai API
type RunaiAuthConfig struct {
	// The kind of authentication scheme to use
	Kind RunaiAuthConfigKind
	// The URI used to issue new tokens
	TokenURI string
	// The Runai client ID (client credentials auth)
	RunaiClientID string
	// The Runai client secret (client credentials auth)
	RunaiClientSecret configUtils.RedactedString
}

type RunaiAuthConfigKind string

// Only "client_credentials" is currently supported for Runai API
const RunaiAuthConfigKindClientCredentials = "client_credentials"

// Validate checks that the authentication config is valid
func (cfg *RunaiAuthConfig) Validate() error {
	if cfg.Kind == "" {
		return fmt.Errorf("kind is not defined")
	}
	if cfg.Kind == RunaiAuthConfigKindClientCredentials {
		return cfg.validateClientCredentials()
	}
	return fmt.Errorf("auth '%s' is not supported", cfg.Kind)
}

func (cfg *RunaiAuthConfig) validateClientCredentials() error {
	if cfg.TokenURI == "" {
		return fmt.Errorf("tokenURI is not defined")
	}
	if _, err := url.Parse(cfg.TokenURI); err != nil {
		return fmt.Errorf("tokenURI is not valid: %w", err)
	}
	if cfg.RunaiClientID == "" {
		return fmt.Errorf("RunaiClientID is not defined")
	}
	if cfg.RunaiClientSecret == "" {
		return fmt.Errorf("RunaiClientSecret is not defined")
	}
	return nil
}

func SetAuthFlags(cmd *cobra.Command) error {
	cmd.Flags().String(authKindFlag, "", "the kind of authentication to use ('renku' or 'client_credentials')")
	if err := viper.BindPFlag(authKindFlag, cmd.Flags().Lookup(authKindFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authKindFlag, configUtils.AsEnvVarFlag(authKindFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+tokenURIFlag, "", "the URI used to issue new tokens to authenticate with Runai")
	if err := viper.BindPFlag(authPrefix+"."+tokenURIFlag, cmd.Flags().Lookup(authPrefix+"-"+tokenURIFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+tokenURIFlag, configUtils.AsEnvVarFlag(authPrefix+"-"+tokenURIFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+runaiClientIDFlag, "", "the Runai client ID (client credentials auth)")
	if err := viper.BindPFlag(authPrefix+"."+runaiClientIDFlag, cmd.Flags().Lookup(authPrefix+"-"+runaiClientIDFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+runaiClientIDFlag, configUtils.AsEnvVarFlag(authPrefix+"-"+runaiClientIDFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+runaiClientSecretFlag, "", "the Runai client secret (client credentials auth)")
	if err := viper.BindPFlag(authPrefix+"."+runaiClientSecretFlag, cmd.Flags().Lookup(authPrefix+"-"+runaiClientSecretFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+runaiClientSecretFlag, configUtils.AsEnvVarFlag(authPrefix+"-"+runaiClientSecretFlag)); err != nil {
		return err
	}

	return nil
}

func GetAuthConfig() (cfg RunaiAuthConfig, err error) {
	cfg.Kind = RunaiAuthConfigKind(viper.GetString(authKindFlag))
	cfg.TokenURI = viper.GetString(authPrefix + "." + tokenURIFlag)
	cfg.RunaiClientID = viper.GetString(authPrefix + "." + runaiClientIDFlag)
	cfg.RunaiClientSecret = configUtils.RedactedString(viper.GetString(authPrefix + "." + runaiClientSecretFlag))

	return cfg, nil
}
