package config

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	configUtils "github.com/SwissDataScienceCenter/amalthea/internal/remote/config/utils"
)

const (
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
	RenkuAccessToken configUtils.RedactedString
	// The Renku refresh token (renku auth)
	RenkuRefreshToken configUtils.RedactedString
	// The URI used to issue new renku tokens (renku auth)
	RenkuTokenURI string
	// The Renku client ID (renku auth)
	RenkuClientID string
	// The Renku client secret (renku auth)
	RenkuClientSecret configUtils.RedactedString

	// The FirecREST client ID (client credentials auth)
	FirecrestClientID string
	// The FirecREST client secret (client credentials auth)
	FirecrestClientSecret configUtils.RedactedString
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
	cmd.Flags().String(configUtils.AuthPrefix+"-"+renkuAccessTokenFlag, "", "the Renku access token (renku auth)")
	if err := viper.BindPFlag(configUtils.AuthPrefix+"."+renkuAccessTokenFlag, cmd.Flags().Lookup(configUtils.AuthPrefix+"-"+renkuAccessTokenFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(configUtils.AuthPrefix+"."+renkuAccessTokenFlag, configUtils.AsEnvVarFlag(configUtils.AuthPrefix+"-"+renkuAccessTokenFlag)); err != nil {
		return err
	}

	cmd.Flags().String(configUtils.AuthPrefix+"-"+renkuRefreshTokenFlag, "", "the Renku refresh token (renku auth)")
	if err := viper.BindPFlag(configUtils.AuthPrefix+"."+renkuRefreshTokenFlag, cmd.Flags().Lookup(configUtils.AuthPrefix+"-"+renkuRefreshTokenFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(configUtils.AuthPrefix+"."+renkuRefreshTokenFlag, configUtils.AsEnvVarFlag(configUtils.AuthPrefix+"-"+renkuRefreshTokenFlag)); err != nil {
		return err
	}

	cmd.Flags().String(configUtils.AuthPrefix+"-"+renkuTokenURIFlag, "", "the URI used to issue new renku tokens (renku auth)")
	if err := viper.BindPFlag(configUtils.AuthPrefix+"."+renkuTokenURIFlag, cmd.Flags().Lookup(configUtils.AuthPrefix+"-"+renkuTokenURIFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(configUtils.AuthPrefix+"."+renkuTokenURIFlag, configUtils.AsEnvVarFlag(configUtils.AuthPrefix+"-"+renkuTokenURIFlag)); err != nil {
		return err
	}

	cmd.Flags().String(configUtils.AuthPrefix+"-"+renkuClientIDFlag, "", "the Renku client ID (renku auth)")
	if err := viper.BindPFlag(configUtils.AuthPrefix+"."+renkuClientIDFlag, cmd.Flags().Lookup(configUtils.AuthPrefix+"-"+renkuClientIDFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(configUtils.AuthPrefix+"."+renkuClientIDFlag, configUtils.AsEnvVarFlag(configUtils.AuthPrefix+"-"+renkuClientIDFlag)); err != nil {
		return err
	}

	cmd.Flags().String(configUtils.AuthPrefix+"-"+renkuClientSecretFlag, "", "the Renku client secret (renku auth)")
	if err := viper.BindPFlag(configUtils.AuthPrefix+"."+renkuClientSecretFlag, cmd.Flags().Lookup(configUtils.AuthPrefix+"-"+renkuClientSecretFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(configUtils.AuthPrefix+"."+renkuClientSecretFlag, configUtils.AsEnvVarFlag(configUtils.AuthPrefix+"-"+renkuClientSecretFlag)); err != nil {
		return err
	}

	cmd.Flags().String(configUtils.AuthPrefix+"-"+firecrestClientIDFlag, "", "the FirecREST client ID (client credentials auth)")
	if err := viper.BindPFlag(configUtils.AuthPrefix+"."+firecrestClientIDFlag, cmd.Flags().Lookup(configUtils.AuthPrefix+"-"+firecrestClientIDFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(configUtils.AuthPrefix+"."+firecrestClientIDFlag, configUtils.AsEnvVarFlag(configUtils.AuthPrefix+"-"+firecrestClientIDFlag)); err != nil {
		return err
	}

	cmd.Flags().String(configUtils.AuthPrefix+"-"+firecrestClientSecretFlag, "", "the FirecREST client secret (client credentials auth)")
	if err := viper.BindPFlag(configUtils.AuthPrefix+"."+firecrestClientSecretFlag, cmd.Flags().Lookup(configUtils.AuthPrefix+"-"+firecrestClientSecretFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(configUtils.AuthPrefix+"."+firecrestClientSecretFlag, configUtils.AsEnvVarFlag(configUtils.AuthPrefix+"-"+firecrestClientSecretFlag)); err != nil {
		return err
	}

	return nil
}

func GetAuthConfig() (cfg FirecrestAuthConfig) {
	cfg.Kind = FirecrestAuthConfigKind(viper.GetString(configUtils.AuthKindFlag))
	cfg.TokenURI = viper.GetString(configUtils.AuthPrefix + "." + configUtils.TokenURIFlag)

	cfg.RenkuAccessToken = configUtils.RedactedString(viper.GetString(configUtils.AuthPrefix + "." + renkuAccessTokenFlag))
	cfg.RenkuRefreshToken = configUtils.RedactedString(viper.GetString(configUtils.AuthPrefix + "." + renkuRefreshTokenFlag))
	cfg.RenkuTokenURI = viper.GetString(configUtils.AuthPrefix + "." + renkuTokenURIFlag)
	cfg.RenkuClientID = viper.GetString(configUtils.AuthPrefix + "." + renkuClientIDFlag)
	cfg.RenkuClientSecret = configUtils.RedactedString(viper.GetString(configUtils.AuthPrefix + "." + renkuClientSecretFlag))
	cfg.FirecrestClientID = viper.GetString(configUtils.AuthPrefix + "." + firecrestClientIDFlag)
	cfg.FirecrestClientSecret = configUtils.RedactedString(viper.GetString(configUtils.AuthPrefix + "." + firecrestClientSecretFlag))

	return cfg
}
