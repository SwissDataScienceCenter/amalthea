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

// package remote implements the remote session controller for Amalthea sessions
package remote

import (
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Command() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs the remote session controller",
		Run:   run,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
	}
	err := config.SetFlags(cmd)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

func run(cmd *cobra.Command, args []string) {
	server.Start()
}
