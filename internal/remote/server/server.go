/*
Copyright 2026.

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
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/SwissDataScienceCenter/amalthea/internal/common"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/controller"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
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
	slog.Info("using remote kind", "kind", cfg.RemoteKind)

	controller, err := controller.NewRemoteSessionController(cfg)
	if err != nil {
		slog.Error("failed to create controller", "error", err)
		os.Exit(1)
	}

	server := newServer(controller, cfg)

	address := fmt.Sprintf(":%d", cfg.ServerPort)

	ctx, stop := signal.NotifyContext(context.Background(), common.InterruptSignals...)
	defer stop()
	// Start server
	go func() {
		if err := server.Start(address); err != nil && err != http.ErrServerClosed {
			slog.Error("shutting down the server gracefully failed", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info(fmt.Sprintf("http server started on %s", address))

	// Start the remote session
	err = controller.Start(ctx)
	if err != nil {
		slog.Error("could not start session", "error", err)
		os.Exit(1)
	}

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 60 seconds.
	<-ctx.Done()
	slog.Info("shutting down the server", "reason", ctx.Err())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := controller.Stop(ctx); err != nil {
		slog.Error("cancelling the remote job failed", "error", err)
	}
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("shutting down the server gracefully failed", "error", err)
		os.Exit(1)
	}
}

var logLevel *slog.LevelVar = new(slog.LevelVar)
var jsonLogger *slog.Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

func newServer(controller controller.RemoteSessionController, cfg config.RemoteSessionControllerConfig) (server *echo.Echo) {
	e := echo.New()

	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Remote session controller: OK")
	})

	// Liveness endpoint
	e.GET("/live", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	// Readiness endpoint
	e.GET("/ready", func(c echo.Context) error {
		switch cfg.ReadinessProbeType {
		case string(amaltheadevv1alpha1.None):
			return c.NoContent(http.StatusOK)
		case string(amaltheadevv1alpha1.TCP):
			if cfg.SessionPort == 0 {
				return c.NoContent(http.StatusOK)
			}
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.SessionPort), 5*time.Second)
			if err != nil {
				return c.NoContent(http.StatusServiceUnavailable)
			}
			if err := conn.Close(); err != nil {
				slog.Error("failed to close readiness probe connection", "error", err)
			}
			return c.NoContent(http.StatusOK)
		case string(amaltheadevv1alpha1.HTTP):
			if cfg.SessionPort == 0 {
				return c.NoContent(http.StatusOK)
			}
			client := &http.Client{
				Timeout: 5 * time.Second,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			url := fmt.Sprintf("http://127.0.0.1:%d%s", cfg.SessionPort, cfg.SessionURLPath)
			resp, err := client.Get(url)
			if err != nil {
				return c.NoContent(http.StatusServiceUnavailable)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					slog.Error("failed to close readiness probe response body", "error", err)
				}
			}()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return c.NoContent(http.StatusOK)
			}
			return c.NoContent(http.StatusServiceUnavailable)
		default:
			// Unconfigured / unknown: preserve old behavior for backward compatibility
			return c.NoContent(http.StatusOK)
		}
	})

	// Status endpoint
	e.GET("/status", func(c echo.Context) error {
		status, err := controller.Status(c.Request().Context())
		if err == nil && status == models.Running {
			return c.JSON(http.StatusOK, statusResponse{
				Status: status,
			})
		}
		return c.JSON(http.StatusServiceUnavailable, statusResponse{
			Status: status,
			Error:  err,
		})
	})

	return e
}

type statusResponse struct {
	Status models.RemoteSessionState `json:"status"`
	Error  error                     `json:"error,omitempty"`
}
