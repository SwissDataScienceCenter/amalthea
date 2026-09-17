package runnersConfig

import (
	"fmt"
	"net/url"

	configUtils "github.com/SwissDataScienceCenter/amalthea/internal/remote/config/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	renkuAPIURLFlag = "renku-api-url"
)

type RunnersConfig struct {
	// The URL of the Renku API
	APIURL string
	// The configuration used to authenticate with Renku
	AuthConfig RunnersAuthConfig
}

func SetFlags(cmd *cobra.Command) error {
	cmd.Flags().String(renkuAPIURLFlag, "", "URL of the Renku API")
	if err := viper.BindPFlag(renkuAPIURLFlag, cmd.Flags().Lookup(renkuAPIURLFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(renkuAPIURLFlag, configUtils.AsEnvVarFlag(renkuAPIURLFlag)); err != nil {
		return err
	}

	// Set up auth flags
	if err := SetAuthFlags(cmd); err != nil {
		return err
	}

	return nil
}

func GetConfig() (cfg RunnersConfig) {
	cfg = RunnersConfig{}

	runnersAuthConfig := GetAuthConfig()
	cfg.AuthConfig = runnersAuthConfig

	return cfg
}

func (cfg *RunnersConfig) Validate() error {
	if cfg.APIURL == "" {
		return fmt.Errorf("runners.APIURL is not defined")
	}
	if _, err := url.Parse(cfg.APIURL); err != nil {
		return fmt.Errorf("runners.APIURL is not valid: %w", err)
	}
	if err := cfg.AuthConfig.Validate(); err != nil {
		return err
	}
	return nil
}
