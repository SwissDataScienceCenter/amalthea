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
	"os"
	"strconv"

	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

// The configuration options for the authentication proxy used for anonymous users.
// The fields below can be passed as arguments i.e. --token=some-very-complicated-random-value
// or as a yaml config file.
const TokenFlag = "token"
const CookieKeyFlag = "cookie_key"
const VerboseFlag = "verbose"

// The port on which the proxy will listen to.
const authProxyPortEnvKey = "PROXY_PORT"

// The URL where the proxy will proxy traffic to - i.e. the upstream URL.
const remoteURLEnvKey = "REMOTE_URL"

func init() {
	rootCmd.AddCommand(serveCmd)

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

	remoteURLStr := os.Getenv(remoteURLEnvKey)
	if remoteURLStr == "" {
		e.Logger.Fatalf("The '%s' environment variable is needed to start the authentication proxy", remoteURLEnvKey)
	}
	remoteURL, err := url.Parse(remoteURLStr)
	if err != nil {
		e.Logger.Fatal(err)
	}
	proxyPortStr := os.Getenv(authProxyPortEnvKey)
	if proxyPortStr == "" {
		proxyPortStr = "65535"
	}
	authProxyPort, err := strconv.Atoi(proxyPortStr)
	if err != nil {
		e.Logger.Fatalf("Cannot convert the proxy port environment variable to integer: %v", err)
	}
	targets := []*middleware.ProxyTarget{
		{
			URL: remoteURL,
		},
	}
	e.Use(middleware.Proxy(middleware.NewRoundRobinBalancer(targets)))

	e.Logger.Info(fmt.Sprintf("Starting proxy for %v", remoteURL))
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", authProxyPort)))
}
