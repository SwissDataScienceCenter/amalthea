package utils

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	AuthKindFlag = "auth-kind"
	AuthPrefix   = "auth"
	TokenURIFlag = "token-uri"
)

func SetFlags(cmd *cobra.Command) error {

	cmd.Flags().String(AuthKindFlag, "", "the kind of authentication to use ('renku' or 'client_credentials')")
	if err := viper.BindPFlag(AuthKindFlag, cmd.Flags().Lookup(AuthKindFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(AuthKindFlag, AsEnvVarFlag(AuthKindFlag)); err != nil {
		return err
	}

	cmd.Flags().String(AuthPrefix+"-"+TokenURIFlag, "", "the URI used to issue new tokens to authenticate with the backend")
	if err := viper.BindPFlag(AuthPrefix+"."+TokenURIFlag, cmd.Flags().Lookup(AuthPrefix+"-"+TokenURIFlag)); err != nil {
		return err
	}
	if err := viper.BindEnv(AuthPrefix+"."+TokenURIFlag, AsEnvVarFlag(AuthPrefix+"-"+TokenURIFlag)); err != nil {
		return err
	}

	return nil
}
