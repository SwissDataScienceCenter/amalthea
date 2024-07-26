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

package cmd

import (
	// "errors"
	"fmt"
	"os"
	// "reflect"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const ConfigFlag = "config"

var (
	rootCmd = &cobra.Command{
		Use:   "proxyauth",
		Short: "A small authentication proxy",
		Long: `authproxy is an reverse proxy that can be used
for token based authentication either through a cookie or
a header.`,
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	viper.SetEnvPrefix("authproxy") // will be uppercased automatically
	viper.AutomaticEnv()

	rootCmd.PersistentFlags().String(ConfigFlag, "", "config file (default is $HOME/.authproxy)")
	viper.BindPFlag(ConfigFlag, rootCmd.PersistentFlags().Lookup(ConfigFlag))

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	cfgFile := viper.GetString(ConfigFlag)

	viper.SetConfigType("yaml")

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".authproxy" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".authproxy")
	}

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		if !rootCmd.Flags().Changed("config") {
			if _, isNotFound := err.(viper.ConfigFileNotFoundError); isNotFound {
				return
			}
		}

		fmt.Println("Failed to read file:", err)
	}

	// Workaround as mandatory flag error triggers when loaded from configuration
	// file or environment variable.
	// https://github.com/spf13/viper/issues/397#issuecomment-1304749092
	serveCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if viper.IsSet(f.Name) {
			serveCmd.Flags().Set(f.Name, viper.GetString(f.Name))
		}
	})
}
