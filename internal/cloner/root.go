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

package cloner

import (
	"github.com/spf13/cobra"
)

func Command() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "A small utility to clone repositories",
		Long:  `cloner is a helper to Amalthea to clone project related repositories`,
		Run:   clone,
	}
	preCloningStrategy := newEnum(PreCloningStrategies, NoStrategy)
	cmd.Flags().StringVar(&configPath, ConfigFlag, "", "Path to configuration file")

	cmd.Flags().StringVar(&remote, RemoteFlag, "", "remote URL to proxy to")
	err := cmd.MarkFlagRequired(RemoteFlag)
	if err != nil {
		return nil, err
	}

	cmd.Flags().StringVar(&revision, RevisionFlag, "", "remote revision (branch, tag, etc.)")

	cmd.Flags().StringVar(&path, PathFlag, "", "clone path")
	err = cmd.MarkFlagRequired(PathFlag)
	if err != nil {
		return nil, err
	}

	cmd.Flags().BoolVar(&verbose, VerboseFlag, false, "make the command verbose")

	cmd.Flags().VarP(preCloningStrategy, StrategyFlag, "", "the pre cloning strategy")
	return cmd, nil
}
