package runners

import (
	"context"
	"log/slog"
	"os"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/runners/auth"
)

type RunnersRemoteSessionController struct {
	rAuth auth.RunnersAuth
}

func NewRunnersRemoteSessionController(cfg config.RemoteSessionControllerConfig) (c *RunnersRemoteSessionController, err error) {
	runnersAuth, err := auth.NewRunnersAuth(cfg.Runners.AuthConfig)
	if err != nil {
		return nil, err
	}

	c = &RunnersRemoteSessionController{
		rAuth: runnersAuth,
	}

	return c, nil
}

// Status returns the status of the remote session
func (c *RunnersRemoteSessionController) Status(ctx context.Context) (state models.RemoteSessionState, err error) {
	// TODO
	return models.Running, nil
}

// Start: does nothing for now
func (c *RunnersRemoteSessionController) Start(ctx context.Context) error {
	// TODO

	// TODO: get wstunnel_secret as a config value
	wstunnel_secret := os.Getenv("RSC_WSTUNNEL_SECRET")
	if wstunnel_secret != "" {
		// err = c.uploadFile(startCtx, secretsPath, "wstunnel_secret", []byte(wstunnel_secret))
		// if err != nil {
		// 	return err
		// }
		slog.Warn("TODO: send WSTUNNEL_SECRET somehow")
	}

	return nil
}

// Stop: does nothing for now
func (c *RunnersRemoteSessionController) Stop(ctx context.Context) error {
	// TODO (?)
	return nil
}
