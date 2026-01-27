package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	configUtils "github.com/SwissDataScienceCenter/amalthea/internal/remote/config/utils"
)

const (
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
	if cfg.RunaiClientID == "" {
		return fmt.Errorf("RunaiClientID is not defined")
	}
	if cfg.RunaiClientSecret == "" {
		return fmt.Errorf("RunaiClientSecret is not defined")
	}
	return nil
}

func SetAuthFlags(cmd *cobra.Command) error {

	cmd.Flags().String(configUtils.AuthPrefix+"-"+runaiClientIDFlag, "", "the Runai client ID (client credentials auth)")
	if err := viper.BindPFlag(configUtils.AuthPrefix+"."+runaiClientIDFlag, cmd.Flags().Lookup(configUtils.AuthPrefix+"-"+runaiClientIDFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(configUtils.AuthPrefix+"."+runaiClientIDFlag, configUtils.AsEnvVarFlag(configUtils.AuthPrefix+"-"+runaiClientIDFlag)); err != nil {
		return err
	}

	cmd.Flags().String(configUtils.AuthPrefix+"-"+runaiClientSecretFlag, "", "the Runai client secret (client credentials auth)")
	if err := viper.BindPFlag(configUtils.AuthPrefix+"."+runaiClientSecretFlag, cmd.Flags().Lookup(configUtils.AuthPrefix+"-"+runaiClientSecretFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(configUtils.AuthPrefix+"."+runaiClientSecretFlag, configUtils.AsEnvVarFlag(configUtils.AuthPrefix+"-"+runaiClientSecretFlag)); err != nil {
		return err
	}

	return nil
}

func GetAuthConfig(apiUrl string) (cfg RunaiAuthConfig) {
	cfg.Kind = RunaiAuthConfigKind(viper.GetString(configUtils.AuthKindFlag))
	cfg.TokenURI = fmt.Sprintf("%s/api/v1/token", apiUrl)
	cfg.RunaiClientID = viper.GetString(configUtils.AuthPrefix + "." + runaiClientIDFlag)
	cfg.RunaiClientSecret = configUtils.RedactedString(viper.GetString(configUtils.AuthPrefix + "." + runaiClientSecretFlag))

	return cfg
}
