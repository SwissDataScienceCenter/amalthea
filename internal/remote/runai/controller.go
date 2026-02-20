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

package runai

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/runai/auth"
)

// The script submitted to start a new remote session.
//
//go:embed session_script.sh
var sessionScript string

var branchRegExp = regexp.MustCompile("[[]branch \"(.+)\"]")

var sessionScriptNoteRegExp = regexp.MustCompile("# NOTE FOR AMALTHEA MAINTAINERS(?s:.*)# END NOTE.*\n")

type RunaiRemoteSessionController struct {
	client *RunaiClient

	jobName string
	jobId   string
	project string

	// currentStatus the current session status
	currentStatus models.RemoteSessionState
	// currentStatusError the current session status error if any
	currentStatusError error
	// statusTicker a ticker which is used to update the session status in the background
	statusTicker *time.Ticker

	// fakeStart if true, do not start the remote session and print debug information
	fakeStart bool
}

func NewRunaiRemoteSessionController(cfg config.RemoteSessionControllerConfig) (c *RunaiRemoteSessionController, err error) {
	runaiAuth, err := auth.NewRunaiAuth(cfg.Runai.AuthConfig)
	if err != nil {
		return nil, err
	}
	runaiBaseURL, err := url.Parse(cfg.Runai.BaseURL)
	if err != nil {
		return nil, err
	}
	runaiClient, err := NewRunaiClient(runaiBaseURL, WithAuth(runaiAuth))
	if err != nil {
		return nil, err
	}
	c = &RunaiRemoteSessionController{
		client:        runaiClient,
		jobName:       "",
		project:       cfg.Runai.Project,
		currentStatus: models.NotReady,
		statusTicker:  time.NewTicker(time.Minute),
		fakeStart:     cfg.FakeStart,
	}
	// Validate controller
	if c.client == nil {
		return nil, fmt.Errorf("client is not set")
	}
	if c.project == "" {
		return nil, fmt.Errorf("project is not set")
	}
	return c, nil
}

// Status returns the status of the remote session
func (c *RunaiRemoteSessionController) Status(ctx context.Context) (state models.RemoteSessionState, err error) {
	return c.currentStatus, c.currentStatusError
}

// Start sets up and starts the remote session using the Runai API
//
//nolint:gocyclo // TODO: can we break down session start?
func (c *RunaiRemoteSessionController) Start(ctx context.Context) error {
	// Start a go routine to update the session status
	go c.periodicSessionStatus(ctx)

	if err := c.recoverJobInfo(); err != nil {
		return err
	}
	// We recovered an existing job id, do nothing
	if c.jobName != "" {
		// return fmt.Errorf("a remote job is already running: %s", c.jobName)
		slog.Info("a remote job is already running, skipping session start", "jobName", c.jobName)
		return nil
	}

	c.jobName = fmt.Sprintf("amalthea-%s-%s", c.project, time.Now().Format("20060102-15-04-05"))
	slog.Info("starting remote session", "project", c.project, "jobName", c.jobName, "env", os.Environ())

	// do not do anything if `fakeStart` is true
	if c.fakeStart {
		slog.Info("fake start", "jobName", c.jobName, "env", os.Environ())
		return nil
	}

	// TODO: should the 15-minute timeout be configurable?
	startCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	fmt.Printf("startCtx: %v\n", startCtx)
	defer cancel()

	project, err := c.getProject(startCtx, c.project)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}
	if project == nil {
		return fmt.Errorf("project %s not found", c.project)
	}

	renkuProjectPath := strings.TrimSuffix(os.Getenv("RENKU_PROJECT_PATH"), "/")
	if renkuProjectPath == "" {
		renkuProjectPath = "dev-project"
		slog.Warn("RENKU_PROJECT_PATH is not defined", "defaultValue", renkuProjectPath)
	}
	renkuBaseURLPath := strings.TrimSuffix(os.Getenv("RENKU_BASE_URL_PATH"), "/")
	if renkuBaseURLPath == "" {
		renkuBaseURLPath = "/"
		slog.Warn("RENKU_BASE_URL_PATH is not defined", "defaultValue", renkuBaseURLPath)
	}

	// TODO: construct the spec dynamically
	workspaceResp, err := c.client.CreateWorkspace(ctx, *project, c.jobName, WorkspaceSpec{
		Image:      "ghcr.io/ciyer/opendata_zh_renku:sha-8fbba0d",
		WorkingDir: "/home/jovyan",
		EnvironmentVariables: []WorkspaceSpecEnvVar{
			{
				Name:  "RENKU_PROJECT_PATH",
				Value: renkuProjectPath,
			},
			{
				Name:  "RENKU_BASE_URL_PATH",
				Value: renkuBaseURLPath,
			},
			{
				Name:  "HOME",
				Value: "/home/jovyan",
			},
			{
				Name:  "RENKU_SESSION_PORT",
				Value: "8888",
			},
			{
				Name:  "RENKU_MOUNT_DIR",
				Value: "/home/renku",
			},
			{
				Name:  "RENKU_WORKING_DIR",
				Value: "/home/renku",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}
	slog.Info("starting remote session", "project", c.project, "renkuProjectPath", renkuProjectPath, "renkuBaseURLPath", renkuBaseURLPath, "workspace", workspaceResp)
	c.jobId = workspaceResp.WorkloadId
	if err := c.saveJobInfo(); err != nil {
		slog.Error("failed to save job info", "error", err)
		return err
	}

	return nil
}

// Stop stops the remote session using the Runai API.
//
// The caller needs to make sure Stop is not called before Start has returned.
func (c *RunaiRemoteSessionController) Stop(ctx context.Context) error {
	// The remote job was never submitted, nothing to do
	if c.jobId == "" {
		slog.Info("no job to cancel")
		return nil
	}

	// Remove the saved state: if the session gets restarted later, we need to submit a fresh job
	if err := c.deleteSavedState(); err != nil {
		slog.Error("could not delete saved state before stopping", "error", err)
	}

	slog.Info("cancelling job", "jobName", c.jobName, "jobId", c.jobId)
	if err := c.client.DeleteWorkspace(ctx, c.jobId); err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}
	slog.Info("job cancelled successfully", "jobName", c.jobName, "jobId", c.jobId)
	return nil
}

func (c *RunaiRemoteSessionController) collectGitRepositories(ctx context.Context, workDir string) (gitRepositories map[string]gitRepository, err error) {
	gitRepositories = map[string]gitRepository{}

	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(workDir, entry.Name())
		gitConfigPath := filepath.Join(fullPath, ".git", "config")
		gitConfigFile, err := os.Open(gitConfigPath)
		if err != nil {
			continue
		}
		gitRepository := gitRepository{
			ConfigPath: gitConfigPath,
		}
		scanner := bufio.NewScanner(gitConfigFile)
		gitBranch := ""
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimSpace(line)
			res := branchRegExp.FindStringSubmatch(line)
			if len(res) > 1 {
				gitBranch = res[1]
			}
			if gitBranch != "" {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			slog.Warn("error when reading a file", "file", gitConfigPath, "error", err)
		}
		if gitBranch != "" {
			gitRepository.Branch = gitBranch
		}
		gitRepositories[entry.Name()] = gitRepository
		if err := gitConfigFile.Close(); err != nil {
			slog.Warn("error when closing a file", "file", gitConfigPath, "error", err)
		}
	}
	return gitRepositories, nil
}

type gitRepository struct {
	Branch     string
	ConfigPath string
}

// periodicSessionStatus sets up periodic refresh of the session status
func (c *RunaiRemoteSessionController) periodicSessionStatus(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.statusTicker.C:
			func() {
				childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				state, err := c.getCurrentStatus(childCtx)
				c.currentStatus = state
				c.currentStatusError = err
				if err == nil {
					slog.Info("current session status", "status", state)
				} else {
					slog.Error("getCurrentStatus failed", "status", state, "error", err)
				}
			}()
		}
	}
}

// getCurrentStatus updates the status of the remote session
func (c *RunaiRemoteSessionController) getCurrentStatus(ctx context.Context) (state models.RemoteSessionState, err error) {
	// TODO: also implement checking the http interface of the remote session through the tunnel
	slog.Info("Getting current status...")
	workloads, err := c.client.GetWorkloads(ctx)
	if err != nil {
		slog.Error("failed to get current status", "error", err)
		return models.NotReady, err
	}

	slog.Info("fetched current status", "workloads", workloads)

	if c.jobName == "" {
		return models.NotReady, nil
	}

	return state, nil
}

func (c *RunaiRemoteSessionController) getProject(ctx context.Context, projectName string) (*ProjectResponse, error) {
	projects, err := c.client.GetProjectsByName(ctx, projectName)
	if err != nil {
		slog.Error("failed to get projects", "error", err)
		return nil, err
	}

	if len(projects) == 0 {
		return nil, nil
	}

	return &projects[0], nil
}

func (c *RunaiRemoteSessionController) deleteSavedState() error {
	savePath := c.getSavePath()
	return os.Remove(savePath)
}

func (c *RunaiRemoteSessionController) recoverJobInfo() error {
	contents, err := os.ReadFile(c.getSavePath())
	if err != nil {
		return nil
	}

	var state savedState
	if err := json.Unmarshal(contents, &state); err != nil {
		return err
	}

	if state.JobName != "" {
		c.jobName = state.JobName
		slog.Info("recovered job name", "jobName", c.jobName)
	}
	return nil
}

func (c *RunaiRemoteSessionController) saveJobInfo() error {
	if c.jobId == "" {
		return fmt.Errorf("cannot save, job id is not defined")
	}
	saveDirPath := c.getSaveDirPath()
	if err := os.MkdirAll(saveDirPath, 0755); err != nil {
		return err
	}
	savePath := c.getSavePath()

	state := savedState{
		JobId:   c.jobId,
		JobName: c.jobName,
	}
	contents, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(savePath, contents, 0644)
}

type savedState struct {
	JobName string `json:"job_name"`
	JobId   string `json:"job_id"`
}

func (c *RunaiRemoteSessionController) getSaveDirPath() string {
	renkuMountDir := os.Getenv("RENKU_MOUNT_DIR")
	return path.Join(renkuMountDir, ".rsc") // NOTE: "rsc" stands for "Remote Session Controller"
}

func (c *RunaiRemoteSessionController) getSavePath() string {

	return path.Join(c.getSaveDirPath(), "state.json")
}
