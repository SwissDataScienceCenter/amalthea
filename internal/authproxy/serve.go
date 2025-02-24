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
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

// The configuration options for the authentication proxy used for anonymous users.
// The fields below can be passed as arguments i.e. --token=some-very-complicated-random-value
// or as a yaml config file.
const remoteFlag = "remote"
const portFlag = "port"
const tokenFlag = "token"
const cookieKeyFlag = "cookie_key"
const verboseFlag = "verbose"
const configFlag = "config"

var remote string
var port int
var token string
var cookieKey string
var verbose bool
var config string

const prefix = "authproxy"

func Command() (*cobra.Command, error) {
	var serveCmd = &cobra.Command{
		Use:     "serve",
		Short:   "Run the proxy",
		Run:     serve,
		PreRunE: loadConfig,
	}

	serveCmd.PersistentFlags().StringVar(&remote, remoteFlag, "", "remote URL to proxy to")
	err := serveCmd.MarkPersistentFlagRequired(remoteFlag)
	if err != nil {
		return nil, err
	}
	err = viper.BindPFlag(prefix+"."+remoteFlag, serveCmd.PersistentFlags().Lookup(remoteFlag))
	if err != nil {
		return nil, err
	}
	err = viper.BindEnv(prefix+"."+remoteFlag, strings.ToUpper(prefix+"_"+remoteFlag))
	if err != nil {
		return nil, err
	}

	serveCmd.PersistentFlags().IntVar(&port, portFlag, 65535, "port on which the proxy will listen")
	err = viper.BindPFlag(prefix+"."+portFlag, serveCmd.PersistentFlags().Lookup(portFlag))
	if err != nil {
		return nil, err
	}
	err = viper.BindEnv(prefix+"."+portFlag, strings.ToUpper(prefix+"_"+portFlag))
	if err != nil {
		return nil, err
	}

	serveCmd.PersistentFlags().StringVar(&cookieKey, cookieKeyFlag, "renku-auth", "cookie key where to find the token")
	err = viper.BindPFlag(prefix+"."+cookieKeyFlag, serveCmd.PersistentFlags().Lookup(cookieKeyFlag))
	if err != nil {
		return nil, err
	}
	err = viper.BindEnv(prefix+"."+cookieKeyFlag, strings.ToUpper(prefix+"_"+cookieKeyFlag))
	if err != nil {
		return nil, err
	}

	serveCmd.PersistentFlags().StringVar(&token, tokenFlag, "", "secret token for authentication")
	err = serveCmd.MarkPersistentFlagRequired(tokenFlag)
	if err != nil {
		return nil, err
	}
	err = viper.BindPFlag(prefix+"."+tokenFlag, serveCmd.PersistentFlags().Lookup(tokenFlag))
	if err != nil {
		return nil, err
	}
	err = viper.BindEnv(prefix+"."+tokenFlag, strings.ToUpper(prefix+"_"+tokenFlag))
	if err != nil {
		return nil, err
	}

	serveCmd.PersistentFlags().BoolVar(&verbose, verboseFlag, false, "make the proxy verbose")
	err = viper.BindPFlag(prefix+"."+verboseFlag, serveCmd.PersistentFlags().Lookup(verboseFlag))
	if err != nil {
		return nil, err
	}
	err = viper.BindEnv(prefix+"."+verboseFlag, strings.ToUpper(prefix+"_"+verboseFlag))
	if err != nil {
		return nil, err
	}

	serveCmd.PersistentFlags().StringVar(&config, configFlag, "", "config file that can provide all the other config options, precedence is given to CLI args over values in the file")

	return serveCmd, nil
}

func serve(cmd *cobra.Command, args []string) {

	e := echo.New()

	e.Use(middleware.Recover())
	e.Logger.SetLevel(log.INFO)
	if verbose {
		e.Logger.SetLevel(log.DEBUG)
	}

	keyLookup := fmt.Sprintf("cookie:%v,header:Authorization", cookieKey)
	authnMW := middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		KeyLookup: keyLookup,
		Validator: func(key string, c echo.Context) (bool, error) {
			return key == token, nil
		},
	})

	remoteURL, err := url.Parse(remote)
	if err != nil {
		e.Logger.Fatal(err)
	}
	targets := []*middleware.ProxyTarget{
		{
			URL: remoteURL,
		},
	}
	// NOTE: You have to have "/*", if you just use "/" for the group path it will not route properly
	proxy := e.Group("/*")
	proxy.Use(middleware.Logger(), authnMW, middleware.Proxy(middleware.NewRoundRobinBalancer(targets)))

	// Healthcheck
	health := e.Group("/__amalthea__")
	health.GET("/health", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	e.Logger.Infof("Starting proxy for remote: %s, cookie key: %s, token of length %d", remoteURL.String(), cookieKey, len(token))
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", port)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// Start server
	go func() {
		if err := e.Start(fmt.Sprintf(":%d", port)); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}
