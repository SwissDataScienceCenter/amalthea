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
	"errors"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"gopkg.in/yaml.v3"
)

const ConfigFlag string = "config"
const RemoteFlag string = "remote"
const RevisionFlag string = "revision"
const PathFlag string = "path"
const VerboseFlag string = "verbose"
const StrategyFlag string = "strategy"

var (
	configPath       string
	remote           string
	revision         string
	path             string
	verbose          bool
	CloningStratgies []string = []string{"notifexist", "overwrite", "default"}
	strategy                  = newEnum(CloningStratgies, "default")
)

type CloneFonfig struct {
	Username   *string `yaml:"username,omitempty"`
	PrivateKey *string `yaml:"privateKey,omitempty"`
	Password   string  `yaml:"password"`
}

func init() {
	rootCmd.AddCommand(cloneCmd)

	cloneCmd.Flags().StringVar(&configPath, ConfigFlag, "", "Path to configuration file")

	cloneCmd.Flags().StringVar(&remote, RemoteFlag, "", "remote URL to proxy to")
	cloneCmd.MarkFlagRequired(RemoteFlag)

	cloneCmd.Flags().StringVar(&revision, RevisionFlag, "", "remote revision (branch, tag, etc.)")

	cloneCmd.Flags().StringVar(&path, PathFlag, "", "clone path")
	cloneCmd.MarkFlagRequired(PathFlag)

	cloneCmd.Flags().BoolVar(&verbose, VerboseFlag, false, "make the command verbose")

	cloneCmd.Flags().VarP(strategy, StrategyFlag, "", "the cloning strategy")
}

var cloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone the repository",
	Run:   clone,
}

func clone(cmd *cobra.Command, args []string) {

	endpoint, err := transport.NewEndpoint(remote)
	if err != nil {
		log.Fatal("failed to parse remote", err)
	}

	splittedRepo := strings.FieldsFunc(endpoint.Path, func(c rune) bool { return c == '/' }) // FieldsFunc handles repeated and beginning/ending separator characters more sanely than Split
	if len(splittedRepo) < 2 {
		log.Fatal("expecting <user>/<repo> in url path, received: ", endpoint.Path)
	}
	projectName := splittedRepo[len(splittedRepo)-1]

	clonePath := projectName
	if path != "" {
		clonePath = path + "/" + projectName
	}

	if !strategy.Equal("default") {
		_, err := os.Stat(clonePath)

		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Fatal("unexpected error", err)
		}

		if err == nil {
			if strategy.Equal("notifexist") {
				log.Print(clonePath, " already exist, doing nothing")
				os.Exit(0)
			}

			if strategy.Equal("overwrite") {
				log.Print("deleting clone")
				err = os.RemoveAll(clonePath + "/")
				if err != nil {
					log.Fatal("failed to remove existing clone", err)
				}
			}
		}
	}

	// Clone the given repository to the given directory
	log.Print("git clone ", remote, " to ", clonePath)

	cloneOptions := git.CloneOptions{
		URL:               remote,
		SingleBranch:      true,
		ReferenceName:     plumbing.ReferenceName(revision),
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Progress:          os.Stdout,
	}

	if configPath != "" {
		buf, err := os.ReadFile(configPath)
		if err != nil {
			log.Fatal("failed to read configuration file", err)
		}

		cloneFonfig := &CloneFonfig{}
		err = yaml.Unmarshal(buf, cloneFonfig)
		if err != nil {
			log.Fatal("failed to parse configuration:", err)
		}

		if cloneFonfig.PrivateKey == nil && cloneFonfig.Username == nil {
			log.Fatal("Invalid authentication configuration one username or privateKey must be set")
		}

		if cloneFonfig.PrivateKey != nil {
			publicKeys, err := ssh.NewPublicKeys("git", []byte(*cloneFonfig.PrivateKey), cloneFonfig.Password)
			if err != nil {
				log.Fatal("generate publickeys failed:", err)
			}
			cloneOptions.Auth = publicKeys
		} else if cloneFonfig.Username != nil {
			cloneOptions.Auth = &http.BasicAuth{
				Username: *cloneFonfig.Username,
				Password: cloneFonfig.Password,
			}
		}

	}

	_, err = git.PlainClone(clonePath, false, &cloneOptions)

	if err != nil {
		log.Fatal("clone failed:", err)
	}
}
