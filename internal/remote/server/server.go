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

package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	glog "github.com/labstack/gommon/log"
)

func Start() {
	server, err := newServer()
	if err != nil {
		log.Fatalln("failed to create server: ", err)
	}

	// TODO: configure
	var port = 8080
	address := fmt.Sprintf(":%d", port)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// Start server
	go func() {
		if err := server.Start(address); err != nil && err != http.ErrServerClosed {
			server.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 60 seconds.
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// TODO: Other cleanup actions here
	if err := server.Shutdown(ctx); err != nil {
		server.Logger.Fatal(err)
	}
}

func newServer() (server *echo.Echo, err error) {
	e := echo.New()

	e.Use(middleware.Recover())
	e.Logger.SetLevel(glog.DEBUG)

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Remote session controller: OK")
	})

	// Liveness endpoint
	e.GET("/live", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	// Readiness endpoint
	e.GET("/ready", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	return e, nil
}
