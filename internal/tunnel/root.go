/*
Copyright 2025.

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

package tunnel

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Command() (*cobra.Command, error) {
	var tunnelCmd = &cobra.Command{
		Use:   "listen",
		Short: "Runs the tunnel server",
		Long:  `tunnel is a helper to open inbound tunnels for Amalthea's remote sessions`,
		RunE:  listen,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
	}

	tunnelCmd.PersistentFlags().String(wstunnelSecretFlag, "", "secret for wstunnel authentication")
	err := viper.BindPFlag(wstunnelPrefix+"."+wstunnelSecretFlag, tunnelCmd.PersistentFlags().Lookup(wstunnelSecretFlag))
	if err != nil {
		return nil, err
	}
	err = viper.BindEnv(wstunnelPrefix+"."+wstunnelSecretFlag, strings.ToUpper(wstunnelPrefix+"_"+wstunnelSecretFlag))
	if err != nil {
		return nil, err
	}

	tunnelCmd.PersistentFlags().String(wstunnelPortFlag, "5050", "port on which wstunnel will listen")
	err = viper.BindPFlag(wstunnelPrefix+"."+wstunnelPortFlag, tunnelCmd.PersistentFlags().Lookup(wstunnelPortFlag))
	if err != nil {
		return nil, err
	}
	err = viper.BindEnv(wstunnelPrefix+"."+wstunnelPortFlag, strings.ToUpper(wstunnelPrefix+"_"+wstunnelPortFlag))
	if err != nil {
		return nil, err
	}

	tunnelCmd.PersistentFlags().String(wstunnelLogLevelFlag, "INFO", "log level for wstunnel")
	err = viper.BindPFlag(wstunnelPrefix+"."+wstunnelLogLevelFlag, tunnelCmd.PersistentFlags().Lookup(wstunnelLogLevelFlag))
	if err != nil {
		return nil, err
	}
	err = viper.BindEnv(wstunnelPrefix+"."+wstunnelLogLevelFlag, strings.ToUpper(wstunnelPrefix+"_"+wstunnelLogLevelFlag))
	if err != nil {
		return nil, err
	}

	return tunnelCmd, nil
}
