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
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func Start() {
	// Logging setup
	slog.SetDefault(jsonLogger)

	cfg, err := config.GetConfig()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}
	slog.Info("loaded configuration", "config", cfg)
	err = cfg.Validate()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	server, err := newServer()
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	address := fmt.Sprintf(":%d", cfg.ServerPort)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// Start server
	go func() {
		if err := server.Start(address); err != nil && err != http.ErrServerClosed {
			slog.Error("shutting down the server gracefully failed", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info(fmt.Sprintf("http server started on %s", address))

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 60 seconds.
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// TODO: Other cleanup actions here
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("shutting down the server gracefully failed", "error", err)
		os.Exit(1)
	}
}

var logLevel *slog.LevelVar = new(slog.LevelVar)
var jsonLogger *slog.Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

func newServer() (server *echo.Echo, err error) {
	e := echo.New()

	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())

	// firecrestAPIURL, err := url.Parse("https://api.cscs.ch/hpc/firecrest/v2/")
	// if err != nil {
	// 	return nil, err
	// }
	// clientID := os.Getenv("FIRECREST_CLIENT_ID")
	// clientSecret := os.Getenv("FIRECREST_CLIENT_SECRET")
	// firecrestAuth, err := auth.NewFirecrestClientCredentialsAuth("https://auth.cscs.ch/auth/realms/firecrest-clients/protocol/openid-connect/token", clientID, clientSecret)
	// if err != nil {
	// 	return nil, err
	// }
	// firecrestClient, err := firecrest.NewFirecrestClient(firecrestAPIURL, firecrest.WithAuth(firecrestAuth))
	// if err != nil {
	// 	return nil, err
	// }
	// controller, err := firecrest.NewFirecrestRemoteSessionController(firecrestClient, "eiger")
	// if err != nil {
	// 	return nil, err
	// }

	// fmt.Println("running system check...")
	// err = controller.CheckSystemAccess(context.Background())
	// if err != nil {
	// 	return nil, err
	// }

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
