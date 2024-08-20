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

const ConfigFlag = "config"
const RemoteFlag = "remote"
const RevisionFlag = "revision"
const PathFlag = "path"
const VerboseFlag = "verbose"

var (
	configPath string
	remote     string
	revision   string
	path       string
	verbose    bool
)

type AuthConfig struct {
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

		authConfig := &AuthConfig{}
		err = yaml.Unmarshal(buf, authConfig)
		if err != nil {
			log.Fatal("failed to parse configuration:", err)
		}

		if authConfig.PrivateKey == nil && authConfig.Username == nil {
			log.Fatal("Invalid authentication configuration one username or privateKey must be set")
		}

		if authConfig.PrivateKey != nil {
			publicKeys, err := ssh.NewPublicKeys("git", []byte(*authConfig.PrivateKey), authConfig.Password)
			if err != nil {
				log.Fatal("generate publickeys failed:", err)
			}
			cloneOptions.Auth = publicKeys
		} else if authConfig.Username != nil {
			cloneOptions.Auth = &http.BasicAuth{
				Username: *authConfig.Username,
				Password: authConfig.Password,
			}
		}

	}

	_, err = git.PlainClone(clonePath, false, &cloneOptions)

	if err != nil {
		log.Fatal("clone failed:", err)
	}
}
