/*
Copyright 2024.

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

package authproxy

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const ConfigFlag = "config"

func initConfig(cmd *cobra.Command, args []string) {
	aViper := viper.New()

	aViper.SetEnvPrefix("authproxy") // will be uppercased automatically
	aViper.AutomaticEnv()

	cmd.PersistentFlags().String(ConfigFlag, "", "config file (default is $HOME/.authproxy)")
	err := viper.BindPFlag(ConfigFlag, cmd.PersistentFlags().Lookup(ConfigFlag))
	cobra.CheckErr(err)

	cfgFile := aViper.GetString(ConfigFlag)

	aViper.SetConfigType("yaml")

	if cfgFile != "" {
		// Use config file from the flag.
		aViper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".authproxy" (without extension).
		aViper.AddConfigPath(home)
		aViper.SetConfigName(".authproxy")
	}

	if err := aViper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", aViper.ConfigFileUsed())
	} else {
		if !cmd.Flags().Changed("config") {
			if _, isNotFound := err.(viper.ConfigFileNotFoundError); isNotFound {
				return
			}
		}

		fmt.Println("Failed to read file:", err)
	}

	// Workaround as mandatory flag error triggers when loaded from configuration
	// file or environment variable.
	// https://github.com/spf13/viper/issues/397#issuecomment-1304749092
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if aViper.IsSet(f.Name) {
			err := cmd.Flags().Set(f.Name, aViper.GetString(f.Name))
			if err != nil {
				cobra.CheckErr(err)
			}
		}
	})
}
