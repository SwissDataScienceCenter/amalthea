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
	"sync"
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
const metaPortFlag = "meta_port"
const tokenFlag = "token"
const cookieKeyFlag = "cookie_key"
const verboseFlag = "verbose"
const configFlag = "config"
const stripPathPrefixFlag = "strip_path_prefix"

var remote string
var port int
var metaPort int
var token string
var cookieKey string
var verbose bool
var config string
var stripPathPrefix string

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

	serveCmd.PersistentFlags().StringVar(&stripPathPrefix, stripPathPrefixFlag, "", "the URL path prefix to strip from all requests")
	err = viper.BindPFlag(prefix+"."+stripPathPrefixFlag, serveCmd.PersistentFlags().Lookup(stripPathPrefixFlag))
	if err != nil {
		return nil, err
	}
	err = viper.BindEnv(prefix+"."+stripPathPrefixFlag, strings.ToUpper(prefix+"_"+stripPathPrefixFlag))
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
	serveCmd.PersistentFlags().IntVar(&metaPort, metaPortFlag, 65534, "port on which the proxy will expose metadata endpoints")
	err = viper.BindPFlag(prefix+"."+metaPortFlag, serveCmd.PersistentFlags().Lookup(metaPortFlag))
	if err != nil {
		return nil, err
	}
	err = viper.BindEnv(prefix+"."+metaPortFlag, strings.ToUpper(prefix+"_"+metaPortFlag))
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

	serveCmd.PersistentFlags().StringVar(&token, tokenFlag, "", "secret token for authentication, if not defined or left blank then there will be no authentication.")
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

type (
	RequestStats struct {
		LastRequest time.Time
		mutex       sync.RWMutex
	}
)

func NewStats() *RequestStats {
	return &RequestStats{
		LastRequest: time.Now(),
	}
}

func (l *RequestStats) Process(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {

		if err := next(c); err != nil {
			c.Error(err)
		}
		l.mutex.Lock()
		defer l.mutex.Unlock()
		l.LastRequest = time.Now()
		return nil
	}
}
func (l *RequestStats) Handle(c echo.Context) error {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return c.String(http.StatusOK, l.LastRequest.Format("2006-01-02 15:04:05"))
}

func serve(cmd *cobra.Command, args []string) {

	e := echo.New()

	e.Use(middleware.Recover())
	e.Logger.SetLevel(log.INFO)
	if verbose {
		e.Logger.SetLevel(log.DEBUG)
	}

	rs := NewStats()
	proxyMWs := []echo.MiddlewareFunc{middleware.Logger(), rs.Process}

	if len(token) > 0 {
		keyLookup := fmt.Sprintf("cookie:%v,header:Authorization", cookieKey)
		authnMW := middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
			KeyLookup: keyLookup,
			Validator: func(key string, c echo.Context) (bool, error) {
				return key == token, nil
			},
		})
		proxyMWs = append(proxyMWs, authnMW)
	} else {
		e.Logger.Info("Token is not defined, running without authentication.")
	}

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
	if len(stripPathPrefix) > 0 {
		if !strings.HasPrefix(stripPathPrefix, "/") {
			stripPathPrefix = "/" + stripPathPrefix
		}
		if !strings.HasSuffix(stripPathPrefix, "/") {
			stripPathPrefix = stripPathPrefix + "/"
		}
		rules := map[string]string{
			fmt.Sprintf("%s*", stripPathPrefix): "/$1",
		}
		e.Logger.Info("Will use path rewrite rules %+v", rules)
		proxyMWs = append(proxyMWs, middleware.Rewrite(rules))
	} else {
		e.Logger.Info("Running without path rewriting")
	}
	proxyMWs = append(proxyMWs, middleware.Proxy(middleware.NewRoundRobinBalancer(targets)))
	proxy.Use(proxyMWs...)

	// Healthcheck
	health := e.Group("/__amalthea__")
	health.GET("/health", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	e.Logger.Infof("Starting proxy for remote: %s, cookie key: %s, token of length %d", remoteURL.String(), cookieKey, len(token))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// Start server
	go func() {
		if err := e.Start(fmt.Sprintf(":%d", port)); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// set up metadata service
	meta := echo.New()
	meta.Use(middleware.Recover())
	meta.Logger.SetLevel(log.INFO)
	if verbose {
		meta.Logger.SetLevel(log.DEBUG)
	}
	meta.GET("/request_stats", rs.Handle)
	go func() {
		if err := meta.Start(fmt.Sprintf(":%d", metaPort)); err != nil && err != http.ErrServerClosed {
			meta.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
	if err := meta.Shutdown(ctx); err != nil {
		meta.Logger.Fatal(err)
	}
}
