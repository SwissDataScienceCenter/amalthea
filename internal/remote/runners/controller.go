package runners

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/runners/api"
	sessionRunners "github.com/SwissDataScienceCenter/amalthea/internal/remote/runners/api/session_runners"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/runners/auth"
)

type RunnersRemoteSessionController struct {
	client *api.RenkuClient
}

func NewRunnersRemoteSessionController(cfg config.RemoteSessionControllerConfig) (c *RunnersRemoteSessionController, err error) {
	runnersAuth, err := auth.NewRunnersAuth(cfg.Runners.AuthConfig)
	if err != nil {
		return nil, err
	}
	renkuAPIURL, err := url.Parse(cfg.Runners.APIURL)
	if err != nil {
		return nil, err
	}
	renkuClient, err := api.NewRenkuClient(renkuAPIURL, api.WithAuth(runnersAuth))
	if err != nil {
		return nil, err
	}

	c = &RunnersRemoteSessionController{
		client: renkuClient,
	}
	// Validate controller
	if c.client == nil {
		return nil, fmt.Errorf("client is not set")
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

	// Determine session ID
	sessionPath := os.Getenv("RENKU_BASE_URL_PATH")
	sessionID := ""
	for _, split := range strings.Split(sessionPath, "/") {
		if split != "" {
			sessionID = split
		}
	}

	if sessionID == "" {
		return fmt.Errorf("could not determine session ID")
	}

	// Wait for runner
	session, err := c.waitUntilRunnerIsAssigned(ctx, sessionID)
	if err != nil {
		return err
	}
	runnerID := ""
	if session.RunnerId != nil {
		runnerID = *session.RunnerId
	}
	if runnerID == "" {
		return fmt.Errorf("could not determine session runner ID")
	}

	// TODO: get wstunnel_secret as a config value
	wstunnelSecret := os.Getenv("RSC_WSTUNNEL_SECRET")
	if wstunnelSecret != "" {
		secrets := []sessionRunners.AssignedSessionSecret{{Name: "RENKU_WSTUNNEL_SECRET", Value: wstunnelSecret}}
		err := c.patchSessionSecrets(ctx, runnerID, sessionID, secrets)
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop: does nothing for now
func (c *RunnersRemoteSessionController) Stop(ctx context.Context) error {
	// TODO (?)
	return nil
}

func (c *RunnersRemoteSessionController) waitUntilRunnerIsAssigned(ctx context.Context, sessionID string) (session sessionRunners.AssignedSession, err error) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		session, err := c.getSession(ctx, sessionID)
		if err != nil {
			slog.Error("waiting for runner", "err", err)
		} else if session.RunnerId != nil && *session.RunnerId != "" {
			return session, nil
		}
	}
}

func (c *RunnersRemoteSessionController) getSession(ctx context.Context, sessionID string) (session sessionRunners.AssignedSession, err error) {
	res, err := c.client.SessionRunners().GetSessionRunnersSessionsSessionIdWithResponse(ctx, sessionID)
	if err != nil {
		return session, err
	}
	resJSON := res.GetJSON200()
	if resJSON == nil {
		message := ""
		resJSONDefault := res.GetJSONDefault()
		if resJSONDefault != nil {
			message = resJSONDefault.Error.Message
			if resJSONDefault.Error.Detail != nil {
				message += fmt.Sprintf(", detail: %s", *resJSONDefault.Error.Detail)
			}
		} else {
			message = res.HTTPResponse.Status
		}
		return session, fmt.Errorf("failed to get session from Renku: %s", message)
	}
	slog.Info("Received from GET /session_runners/sessions/{session_id}", "session", session)
	return *resJSON, nil
}

func (c *RunnersRemoteSessionController) patchSessionSecrets(ctx context.Context, runnerID, sessionID string, secrets []sessionRunners.AssignedSessionSecret) error {
	slog.Info("Sending to PATCH /session_runners/{session_runner_id}/sessions/{session_id}/secrets", "runnerID", runnerID, "sessionID", sessionID)
	res, err := c.client.SessionRunners().PatchSessionRunnersSessionRunnerIdSessionsSessionIdSecretsWithResponse(ctx, runnerID, sessionID, secrets)
	if err != nil {
		return err
	}
	resJSON := res.GetJSON200()
	if resJSON == nil {
		message := ""
		resJSONDefault := res.GetJSONDefault()
		if resJSONDefault != nil {
			message = resJSONDefault.Error.Message
			if resJSONDefault.Error.Detail != nil {
				message += fmt.Sprintf(", detail: %s", *resJSONDefault.Error.Detail)
			}
		} else {
			message = res.HTTPResponse.Status
		}
		return fmt.Errorf("failed to update session secrets: %s", message)
	}
	return nil

}
