package runners

import (
	"context"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
)

type RunnersRemoteSessionController struct{}

func NewRunnersRemoteSessionController(cfg config.RemoteSessionControllerConfig) (c *RunnersRemoteSessionController, err error) {
	c = &RunnersRemoteSessionController{}
	return c, nil
}

// Status returns the status of the remote session
func (c *RunnersRemoteSessionController) Status(ctx context.Context) (state models.RemoteSessionState, err error) {
	// TODO
	return models.Running, nil
}

// Start: does nothing for now
func (c *RunnersRemoteSessionController) Start(ctx context.Context) error {
	return nil
}

// Stop: does nothing for now
func (c *RunnersRemoteSessionController) Stop(ctx context.Context) error {
	return nil
}
