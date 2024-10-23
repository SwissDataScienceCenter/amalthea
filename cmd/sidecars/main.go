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
package main

import (
	"runtime/debug"

	"github.com/SwissDataScienceCenter/amalthea/internal/authproxy"
	"github.com/SwissDataScienceCenter/amalthea/internal/cloner"
	"github.com/spf13/cobra"
)

func buildCommands() *cobra.Command {
	var rootCmd = &cobra.Command{
		Short: "Amalthea sidecar utilities",
		Long:  "Amalthea sidecar utilities",
	}
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of the sidecar executable",
		Run: func(cmd *cobra.Command, args []string) {
			version := "(devel)"
			info, ok := debug.ReadBuildInfo()
			if ok && len(info.Main.Version) > 0 {
				version = info.Main.Version
			}
			cmd.Println("sidecars", version)
		},
	}
	proxyRoot := &cobra.Command{
		Use:   "proxy serve",
		Short: "Authentication proxy",
	}
	clonerRoot := &cobra.Command{
		Use:   "cloner clone",
		Short: "Cloning utilities",
	}
	rootCmd.AddCommand(versionCmd)
	authCmd, err := authproxy.Command()
	cobra.CheckErr(err)
	clonerCmd, err := cloner.Command()
	cobra.CheckErr(err)
	proxyRoot.AddCommand(authCmd)
	clonerRoot.AddCommand(clonerCmd)
	rootCmd.AddCommand(proxyRoot)
	rootCmd.AddCommand(clonerRoot)
	return rootCmd
}

func main() {
	cmd := buildCommands()
	cobra.CheckErr(cmd.Execute())
}
