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
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/firecrest"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/firecrest/auth"
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

	firecrestAPIURL, err := url.Parse("https://api.cscs.ch/hpc/firecrest/v2/")
	if err != nil {
		return nil, err
	}
	clientID := os.Getenv("FIRECREST_CLIENT_ID")
	clientSecret := os.Getenv("FIRECREST_CLIENT_SECRET")
	firecrestAuth, err := auth.NewFirecrestClientCredentialsAuth("https://auth.cscs.ch/auth/realms/firecrest-clients/protocol/openid-connect/token", clientID, clientSecret)
	if err != nil {
		return nil, err
	}
	firecrestClient, err := firecrest.NewFirecrestClient(firecrestAPIURL, firecrest.WithAuth(firecrestAuth))
	if err != nil {
		return nil, err
	}
	controller, err := firecrest.NewFirecrestRemoteSessionController(firecrestClient, "eiger")
	if err != nil {
		return nil, err
	}

	fmt.Println("running system check...")
	err = controller.CheckSystemAccess(context.Background())
	if err != nil {
		return nil, err
	}

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
