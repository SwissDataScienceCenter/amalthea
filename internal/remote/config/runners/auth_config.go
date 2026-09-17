package runnersConfig

import (
	"fmt"
	"net/url"

	configUtils "github.com/SwissDataScienceCenter/amalthea/internal/remote/config/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	accessTokenFlag  = "renku-access-token"
	refreshTokenFlag = "renku-refresh-token"
)

type RunnersAuthConfig struct {
	// The kind of authentication scheme to use
	Kind RunnersAuthConfigKind
	// The URI used to issue new tokens
	TokenURI string

	// The Renku access token (renku auth)
	AccessToken configUtils.RedactedString
	// The Renku refresh token (renku auth)
	RefreshToken configUtils.RedactedString
}

type RunnersAuthConfigKind string

// Only "renku_v2" is currently supported for Runners
const RunnersAuthConfigKindRenkuV2 RunnersAuthConfigKind = "renku_v2"

// Validate checks that the authentication config is valid
func (cfg *RunnersAuthConfig) Validate() error {
	if cfg.Kind == "" {
		return fmt.Errorf("kind is not defined")
	}
	if cfg.Kind == RunnersAuthConfigKindRenkuV2 {
		return cfg.validateRenkuV2()
	}
	return fmt.Errorf("auth '%s' is not supported", cfg.Kind)
}

func (cfg *RunnersAuthConfig) validateRenkuV2() error {
	if cfg.TokenURI == "" {
		return fmt.Errorf("tokenURI is not defined")
	}
	if _, err := url.Parse(cfg.TokenURI); err != nil {
		return fmt.Errorf("tokenURI is not valid: %w", err)
	}
	if cfg.AccessToken == "" {
		return fmt.Errorf("renkuAccessToken is not defined")
	}
	if cfg.RefreshToken == "" {
		return fmt.Errorf("renkuRefreshToken is not defined")
	}
	return nil
}

func SetAuthFlags(cmd *cobra.Command) error {
	cmd.Flags().String(configUtils.AuthPrefix+"-"+accessTokenFlag, "", "the Renku access token (renku auth)")
	if err := viper.BindPFlag(configUtils.AuthPrefix+"."+accessTokenFlag, cmd.Flags().Lookup(configUtils.AuthPrefix+"-"+accessTokenFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(configUtils.AuthPrefix+"."+accessTokenFlag, configUtils.AsEnvVarFlag(configUtils.AuthPrefix+"-"+accessTokenFlag)); err != nil {
		return err
	}

	cmd.Flags().String(configUtils.AuthPrefix+"-"+refreshTokenFlag, "", "the Renku refresh token (renku auth)")
	if err := viper.BindPFlag(configUtils.AuthPrefix+"."+refreshTokenFlag, cmd.Flags().Lookup(configUtils.AuthPrefix+"-"+refreshTokenFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(configUtils.AuthPrefix+"."+refreshTokenFlag, configUtils.AsEnvVarFlag(configUtils.AuthPrefix+"-"+refreshTokenFlag)); err != nil {
		return err
	}

	return nil
}

func GetAuthConfig() (cfg RunnersAuthConfig) {
	cfg.Kind = RunnersAuthConfigKind(viper.GetString(configUtils.AuthKindFlag))
	cfg.TokenURI = viper.GetString(configUtils.AuthPrefix + "." + configUtils.TokenURIFlag)

	cfg.AccessToken = configUtils.RedactedString(viper.GetString(configUtils.AuthPrefix + "." + accessTokenFlag))
	cfg.RefreshToken = configUtils.RedactedString(viper.GetString(configUtils.AuthPrefix + "." + refreshTokenFlag))

	return cfg
}
