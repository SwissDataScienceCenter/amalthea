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
	renkuAccessTokenFlag  = "renku-access-token"
	renkuRefreshTokenFlag = "renku-refresh-token"
	renkuTokenURIFlag     = "renku-token-uri"
	renkuClientIDFlag     = "renku-client-id"
	renkuClientSecretFlag = "renku-client-secret"
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

	// The Renku access token (renku auth)
	RenkuAccessToken configUtils.RedactedString
	// The Renku refresh token (renku auth)
	RenkuRefreshToken configUtils.RedactedString
	// The URI used to issue new renku tokens (renku auth)
	RenkuTokenURI string
	// The Renku client ID (renku auth)
	RenkuClientID string
	// The Renku client secret (renku auth)
	RenkuClientSecret configUtils.RedactedString

	// The Runai client ID (client credentials auth)
	RunaiClientID string
	// The Runai client secret (client credentials auth)
	RunaiClientSecret configUtils.RedactedString
}

type RunaiAuthConfigKind string

const RunaiAuthConfigKindRenku = "renku"
const RunaiAuthConfigKindClientCredentials = "client_credentials"

// Validate checks that the authentication config is valid
func (cfg *RunaiAuthConfig) Validate() error {
	if cfg.Kind == "" {
		return fmt.Errorf("kind is not defined")
	}
	if cfg.Kind == RunaiAuthConfigKindRenku {
		return cfg.validateRenku()
	}
	if cfg.Kind == RunaiAuthConfigKindClientCredentials {
		return cfg.validateClientCredentials()
	}
	return fmt.Errorf("auth '%s' is not supported", cfg.Kind)
}

func (cfg *RunaiAuthConfig) validateRenku() error {
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

	cmd.Flags().String(authPrefix+"-"+renkuAccessTokenFlag, "", "the Renku access token (renku auth)")
	if err := viper.BindPFlag(authPrefix+"."+renkuAccessTokenFlag, cmd.Flags().Lookup(authPrefix+"-"+renkuAccessTokenFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+renkuAccessTokenFlag, configUtils.AsEnvVarFlag(authPrefix+"-"+renkuAccessTokenFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+renkuRefreshTokenFlag, "", "the Renku refresh token (renku auth)")
	if err := viper.BindPFlag(authPrefix+"."+renkuRefreshTokenFlag, cmd.Flags().Lookup(authPrefix+"-"+renkuRefreshTokenFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+renkuRefreshTokenFlag, configUtils.AsEnvVarFlag(authPrefix+"-"+renkuRefreshTokenFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+renkuTokenURIFlag, "", "the URI used to issue new renku tokens (renku auth)")
	if err := viper.BindPFlag(authPrefix+"."+renkuTokenURIFlag, cmd.Flags().Lookup(authPrefix+"-"+renkuTokenURIFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+renkuTokenURIFlag, configUtils.AsEnvVarFlag(authPrefix+"-"+renkuTokenURIFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+renkuClientIDFlag, "", "the Renku client ID (renku auth)")
	if err := viper.BindPFlag(authPrefix+"."+renkuClientIDFlag, cmd.Flags().Lookup(authPrefix+"-"+renkuClientIDFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+renkuClientIDFlag, configUtils.AsEnvVarFlag(authPrefix+"-"+renkuClientIDFlag)); err != nil {
		return err
	}

	cmd.Flags().String(authPrefix+"-"+renkuClientSecretFlag, "", "the Renku client secret (renku auth)")
	if err := viper.BindPFlag(authPrefix+"."+renkuClientSecretFlag, cmd.Flags().Lookup(authPrefix+"-"+renkuClientSecretFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(authPrefix+"."+renkuClientSecretFlag, configUtils.AsEnvVarFlag(authPrefix+"-"+renkuClientSecretFlag)); err != nil {
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

	cfg.RenkuAccessToken = configUtils.RedactedString(viper.GetString(authPrefix + "." + renkuAccessTokenFlag))
	cfg.RenkuRefreshToken = configUtils.RedactedString(viper.GetString(authPrefix + "." + renkuRefreshTokenFlag))
	cfg.RenkuTokenURI = viper.GetString(authPrefix + "." + renkuTokenURIFlag)
	cfg.RenkuClientID = viper.GetString(authPrefix + "." + renkuClientIDFlag)
	cfg.RenkuClientSecret = configUtils.RedactedString(viper.GetString(authPrefix + "." + renkuClientSecretFlag))
	cfg.RunaiClientID = viper.GetString(authPrefix + "." + runaiClientIDFlag)
	cfg.RunaiClientSecret = configUtils.RedactedString(viper.GetString(authPrefix + "." + runaiClientSecretFlag))

	return cfg, nil
}
