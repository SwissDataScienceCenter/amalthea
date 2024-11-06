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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func loadConfig(cmd *cobra.Command, args []string) error {
	// If the config file is not set then we skip loading any files and just
	// expect all the parameters to be passed through the CLI or env vars
	if config != "" {
		viper.SetConfigType("yaml")
		viper.SetConfigFile(config)
		err := viper.ReadInConfig()
		fmt.Println(err)
		if err != nil {
			return err
		}
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}

	// Workaround as mandatory flag error triggers when loaded from configuration
	// file or environment variable.
	// https://github.com/spf13/viper/issues/397#issuecomment-1304749092
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		vname := prefix + "." + f.Name
		if viper.IsSet(vname) {
			err := cmd.Flags().Set(f.Name, viper.GetString(vname))
			cobra.CheckErr(err)
		}
	})
	return nil
}
