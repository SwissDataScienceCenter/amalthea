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
	"fmt"

	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

const RemoteFlag = "remote"
const PortFlag = "port"
const TokenFlag = "token"
const CookieKeyFlag = "cookie_key"
const VerboseFlag = "verbose"

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.PersistentFlags().String(RemoteFlag, "", "remote URL to proxy to")
	serveCmd.MarkPersistentFlagRequired(RemoteFlag)
	viper.BindPFlag(RemoteFlag, serveCmd.PersistentFlags().Lookup(RemoteFlag))
	viper.BindEnv(RemoteFlag)

	serveCmd.PersistentFlags().Int(PortFlag, 4180, "port on which the proxy will listen")
	viper.BindPFlag(PortFlag, serveCmd.PersistentFlags().Lookup(PortFlag))
	viper.BindEnv(PortFlag)

	serveCmd.PersistentFlags().String(CookieKeyFlag, "renku-auth", "cookie key where to find the token")
	viper.BindPFlag(CookieKeyFlag, serveCmd.PersistentFlags().Lookup(CookieKeyFlag))
	viper.BindEnv(CookieKeyFlag)

	serveCmd.PersistentFlags().String(TokenFlag, "", "secret token for authentication")
	serveCmd.MarkPersistentFlagRequired(TokenFlag)
	viper.BindPFlag(TokenFlag, serveCmd.PersistentFlags().Lookup(TokenFlag))
	viper.BindEnv(TokenFlag)

	serveCmd.PersistentFlags().Bool(VerboseFlag, false, "make the proxy verbose")
	viper.BindPFlag(VerboseFlag, serveCmd.PersistentFlags().Lookup(VerboseFlag))
	viper.BindEnv(VerboseFlag)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the proxy",
	Run:   serve,
}

func serve(cmd *cobra.Command, args []string) {

	e := echo.New()

	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	verbose := viper.GetBool(VerboseFlag)
	if verbose {
		e.Logger.SetLevel(log.INFO)
	}

	remote := viper.GetString(RemoteFlag)
	if remote == "" {
		e.Logger.Fatal("Invalid remote URL")
	}

	port := viper.GetInt(PortFlag)
	if port == 0 {
		e.Logger.Warn("Using random port")
	}

	cookieKey := viper.GetString(CookieKeyFlag)
	if cookieKey == "" {
		e.Logger.Fatal("Invalid cookie key")
	}

	token := viper.GetString(TokenFlag)
	if token == "" {
		e.Logger.Fatal("Invalid token")
	}

	keyLookup := fmt.Sprintf("cookie:%v,header:Authorization", cookieKey)
	e.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		KeyLookup: keyLookup,
		Validator: func(key string, c echo.Context) (bool, error) {
			return key == token, nil
		},
	}))

	remoteURL, err := url.Parse(remote)
	if err != nil {
		e.Logger.Fatal(err)
	}
	targets := []*middleware.ProxyTarget{
		{
			URL: remoteURL,
		},
	}
	e.Use(middleware.Proxy(middleware.NewRoundRobinBalancer(targets)))

	e.Logger.Info(fmt.Sprintf("Starting proxy for %v", remoteURL))
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", port)))
}
