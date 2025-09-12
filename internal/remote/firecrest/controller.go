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

package firecrest

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
)

// The script submitted to start a new remote session.
//
//go:embed session_script.sh
var sessionScript string

var branchRegExp = regexp.MustCompile("^[[]branch \"(.)+\"]")

type FirecrestRemoteSessionController struct {
	client *FirecrestClient

	jobID      string
	systemName string

	// currentStatus the current session status
	currentStatus models.RemoteSessionState
	// currentStatusError the current session status error if any
	currentStatusError error
	// statusTicker a ticker which is used to update the session status in the background
	statusTicker *time.Ticker
}

func NewFirecrestRemoteSessionController(client *FirecrestClient, systemName string) (c *FirecrestRemoteSessionController, err error) {
	c = &FirecrestRemoteSessionController{
		client:        client,
		jobID:         "",
		systemName:    systemName,
		currentStatus: models.NotReady,
		statusTicker:  time.NewTicker(time.Minute),
	}
	// Validate controller
	if c.client == nil {
		return nil, fmt.Errorf("client is not set")
	}
	if c.systemName == "" {
		return nil, fmt.Errorf("systemName is not set")
	}
	return c, nil
}

func (c *FirecrestRemoteSessionController) GetCurrentSystem(ctx context.Context) (system HPCCluster, err error) {
	res, err := c.client.GetSystemsStatusSystemsGetWithResponse(ctx)
	if err != nil {
		return HPCCluster{}, err
	}
	if res.JSON200 == nil {
		message := getErrorMessage(res.JSON4XX, res.JSON5XX)
		if message != "" {
			return HPCCluster{}, fmt.Errorf("could not get job: %s", message)
		}
		return HPCCluster{}, fmt.Errorf("could not get job: HTTP %d", res.StatusCode())
	}
	for _, sys := range res.JSON200.Systems {
		if sys.Name == c.systemName {
			return sys, nil
		}
	}
	return HPCCluster{}, fmt.Errorf("system '%s' not found", c.systemName)
}

// Status returns the status of the remote session
func (c *FirecrestRemoteSessionController) Status(ctx context.Context) (state models.RemoteSessionState, err error) {
	return c.currentStatus, c.currentStatusError
}

// Start sets up and starts the remote session using the FirecREST API
func (c *FirecrestRemoteSessionController) Start(ctx context.Context) error {
	// TODO: handle start when the pod was deleted:
	// TODO: 1. we should save the job ID on disk, on the session PVC
	// TODO: 2. try to load the currently running job ID from disk

	// Start a go routine to update the session status
	go c.periodicSessionStatus()

	if c.jobID != "" {
		return fmt.Errorf("a remote job is already running: %s", c.jobID)
	}

	// start by checking whether we can access the requested system
	system, err := c.GetCurrentSystem(ctx)
	if err != nil {
		return err
	}

	userInfo, err := c.getUserInfo(ctx)
	if err != nil {
		return err
	}
	userName := userInfo.User.Name
	if userName == "" {
		return fmt.Errorf("could not get user name")
	}
	slog.Info("got username", "username", userName)

	var scratch *FileSystem
	if system.FileSystems != nil {
		for _, fs := range *system.FileSystems {
			if fs.DataType == Scratch {
				scratch = &fs
			}
		}
	}
	if scratch == nil {
		return fmt.Errorf("could not find scratch file system on '%s'", c.systemName)
	}

	renkuBaseURLPath := strings.TrimSuffix(os.Getenv("RENKU_BASE_URL_PATH"), "/")
	if renkuBaseURLPath == "" {
		renkuBaseURLPath = "dev-session"
		slog.Warn("RENKU_BASE_URL_PATH is not defined", "defaultValue", renkuBaseURLPath)
	}

	scratchPathRenku := path.Join(scratch.Path, userName, "renku")
	sessionPath := path.Join(scratchPathRenku, renkuBaseURLPath)

	slog.Info("determined session path", "sessionPath", sessionPath)

	// Setup secrets
	secretsPath := path.Join(sessionPath, "secrets")
	err = c.mkdir(ctx, secretsPath, true /* createParents */)
	if err != nil {
		return err
	}
	// Makes sure that only the session owner can read session files
	err = c.chmod(ctx, sessionPath, "700")
	if err != nil {
		return err
	}
	wstunnel_secret := os.Getenv("WSTUNNEL_SECRET")
	if wstunnel_secret != "" {
		err = c.uploadFile(ctx, secretsPath, "wstunnel_secret", []byte(wstunnel_secret))
		if err != nil {
			return err
		}
	}
	// TODO: upload user secrets into secretsPath

	// Setup git repositories
	renkuWorkDir := os.Getenv("RENKU_WORKING_DIR")
	gitRepositories, err := c.collectGitRepositories(ctx, renkuWorkDir)
	if err != nil {
		return err
	}
	fmt.Printf("gitRepositories: %+v\n", gitRepositories)
	for repo := range gitRepositories {
		repoGitDirPath := path.Join(sessionPath, "work", repo, ".git")
		err = c.mkdir(ctx, repoGitDirPath, true /* createParents */)
		if err != nil {
			return err
		}
		gitConfigContents, err := os.ReadFile(gitRepositories[repo].ConfigPath)
		if err != nil {
			return err
		}
		err = c.uploadFile(ctx, repoGitDirPath, "config", gitConfigContents)
		if err != nil {
			return err
		}
	}

	env := map[string]string{}
	// Copy the REMOTE_SESSION environment variables
	for _, environ := range os.Environ() {
		key, val, _ := strings.Cut(environ, "=")
		if strings.HasPrefix(key, "REMOTE_SESSION") {
			env[key] = val
		}
	}
	// Copy RENKU environment variables
	for _, environ := range os.Environ() {
		key, val, _ := strings.Cut(environ, "=")
		if strings.HasPrefix(key, "RENKU") {
			env[key] = val
		}
	}
	// Setup WSTUNNEL environment variables
	renkuBaseURLStr := os.Getenv("RENKU_BASE_URL")
	if renkuBaseURLStr != "" {
		renkuBaseURL, err := url.Parse(renkuBaseURLStr)
		if err != nil {
			return err
		}
		env["WSTUNNEL_SERVICE_ADDRESS"] = renkuBaseURL.Hostname()
		env["WSTUNNEL_SERVICE_PORT"] = fmt.Sprintf("%d", 443)      // wss port (same as https)
		env["WSTUNNEL_PATH_PREFIX"] = renkuBaseURLPath + "/tunnel" // session path with tunnel
	}
	// Setup environment variables for git repositories
	repos := []string{}
	for repo := range gitRepositories {
		repos = append(repos, fmt.Sprintf("%s\t%s", repo, gitRepositories[repo].Branch))
	}
	env["GIT_REPOSITORIES"] = strings.Join(repos, "\n")

	// TODO: GIT_PROXY_PORT

	// TODO: upload session script
	// TODO: maybe the session script should be a template: pass account, partition, log files, etc.

	jobEnv := JobDescriptionModel_Env{}
	err = jobEnv.FromJobDescriptionModelEnv0(env)
	if err != nil {
		return err
	}
	job := JobDescriptionModel{
		Env:              &jobEnv,
		Script:           &sessionScript,
		WorkingDirectory: sessionPath,
	}
	jobID, err := c.submitJob(ctx, job)
	if err != nil {
		return err
	}
	c.jobID = jobID

	slog.Info("submitted job", "jobID", c.jobID)

	return nil
}

// Stop stops the remote session using the FirecREST API
func (c *FirecrestRemoteSessionController) Stop(ctx context.Context) error {
	// The remote job was never submitted, nothing to do
	if c.jobID == "" {
		slog.Info("no job to cancel")
		return nil
	}

	slog.Info("cancelling job", "jobID", c.jobID)
	res, err := c.client.DeleteJobCancelComputeSystemNameJobsJobIdDeleteWithResponse(ctx, c.systemName, c.jobID)
	if err != nil {
		return err
	}
	if res.StatusCode() < 200 || res.StatusCode() >= 300 {
		message := getErrorMessage(res.JSON4XX, res.JSON5XX)
		if message != "" {
			return fmt.Errorf("could not cancel job: %s", message)
		}
		return fmt.Errorf("could not cancel job: HTTP %d", res.StatusCode())
	}

	return nil
}

func (c *FirecrestRemoteSessionController) collectGitRepositories(ctx context.Context, workDir string) (gitRepositories map[string]*gitRepository, err error) {
	gitRepositories = map[string]*gitRepository{}

	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(workDir, entry.Name())
		gitConfigPath := filepath.Join(fullPath, ".git", "config")
		gitConfigFile, err := os.Open(gitConfigPath)
		if err != nil {
			continue
		}
		gitRepositories[entry.Name()] = &gitRepository{
			ConfigPath: gitConfigPath,
		}
		scanner := bufio.NewScanner(gitConfigFile)
		gitBranch := ""
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimSpace(line)
			gitBranch = branchRegExp.FindString(line)
			if gitBranch != "" {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("error while reading %s: %s\n", gitConfigPath, err.Error())
			continue
		}
		if gitBranch != "" {
			gitRepositories[entry.Name()].Branch = gitBranch
		}
		gitConfigFile.Close()
	}
	return gitRepositories, nil
}

type gitRepository struct {
	Branch     string
	ConfigPath string
}

func (c *FirecrestRemoteSessionController) getUserInfo(ctx context.Context) (userInfo UserInfoResponse, err error) {
	res, err := c.client.GetUserinfoStatusSystemNameUserinfoGetWithResponse(ctx, c.systemName)
	if err != nil {
		return UserInfoResponse{}, err
	}
	if res.JSON200 == nil {
		message := getErrorMessage(res.JSON4XX, res.JSON5XX)
		if message != "" {
			return UserInfoResponse{}, fmt.Errorf("could not get user info: %s", message)
		}
		return UserInfoResponse{}, fmt.Errorf("could not get user info: HTTP %d", res.StatusCode())
	}
	return *res.JSON200, nil
}

func (c *FirecrestRemoteSessionController) uploadFile(ctx context.Context, directory, filename string, contents []byte) error {
	params := PostUploadFilesystemSystemNameOpsUploadPostParams{
		Path: directory,
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, bytes.NewReader(contents))
	if err != nil {
		return err
	}
	err = writer.Close()
	if err != nil {
		return err
	}
	res, err := c.client.PostUploadFilesystemSystemNameOpsUploadPostWithBodyWithResponse(ctx, c.systemName, &params, writer.FormDataContentType(), body)
	if err != nil {
		return err
	}
	if res.StatusCode() != 204 {
		message := ""
		if res.JSON4XX != nil {
			message = res.JSON4XX.Message
		} else if res.JSON5XX != nil {
			message = res.JSON5XX.Message
		}
		if message != "" {
			return fmt.Errorf("could run uploadFile: %s", message)
		}
		return fmt.Errorf("could run uploadFile: HTTP %d", res.StatusCode())
	}
	return nil
}

func (c *FirecrestRemoteSessionController) mkdir(ctx context.Context, path string, createParents bool) error {
	body := PostMakeDirRequest{
		Parent:     &createParents,
		SourcePath: &path,
	}
	res, err := c.client.PostMkdirFilesystemSystemNameOpsMkdirPostWithResponse(ctx, c.systemName, body)
	if err != nil {
		return err
	}
	if res.JSON201 == nil {
		message := getErrorMessage(res.JSON4XX, res.JSON5XX)
		if message != "" {
			return fmt.Errorf("could run mkdir: %s", message)
		}
		return fmt.Errorf("could run mkdir: HTTP %d", res.StatusCode())
	}
	return nil
}

func (c *FirecrestRemoteSessionController) chmod(ctx context.Context, path string, mode string) error {
	body := PutFileChmodRequest{
		Mode:       mode,
		SourcePath: &path,
	}
	res, err := c.client.PutChmodFilesystemSystemNameOpsChmodPutWithResponse(ctx, c.systemName, body)
	if err != nil {
		return err
	}
	if res.JSON200 == nil {
		message := getErrorMessage(res.JSON4XX, res.JSON5XX)
		if message != "" {
			return fmt.Errorf("could run chmod: %s", message)
		}
		return fmt.Errorf("could run chmod: HTTP %d", res.StatusCode())
	}
	return nil
}

func (c *FirecrestRemoteSessionController) submitJob(ctx context.Context, job JobDescriptionModel) (jobId string, err error) {
	body := PostJobSubmitRequest{
		Job: job,
	}
	res, err := c.client.PostJobSubmitComputeSystemNameJobsPostWithResponse(ctx, c.systemName, body)
	if err != nil {
		return "", err
	}
	if res.JSON201 == nil {
		message := getErrorMessage(res.JSON4XX, res.JSON5XX)
		if message != "" {
			return "", fmt.Errorf("could run submitJob: %s", message)
		}
		return "", fmt.Errorf("could run submitJob: HTTP %d", res.StatusCode())
	}
	if res.JSON201.JobId == nil {
		return "", fmt.Errorf("invalid job submission response")
	}
	return fmt.Sprintf("%d", *res.JSON201.JobId), nil
}

func getErrorMessage(json4XX, json5XX *ApiResponseError) (message string) {
	message = ""
	if json4XX != nil {
		message = json4XX.Message
	} else if json5XX != nil {
		message = json5XX.Message
	}
	return message
}

// periodicSessionStatus sets up periodic refresh of the session status
func (c *FirecrestRemoteSessionController) periodicSessionStatus() {
	for {
		<-c.statusTicker.C
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		state, err := c.getCurrentStatus(ctx)
		c.currentStatus = state
		c.currentStatusError = err
		if err == nil {
			slog.Info("current session status", "status", state)
		} else {
			slog.Error("getCurrentStatus failed", "status", state, "error", err)
		}
		cancel()
	}
}

// getCurrentStatus updates the status of the remote session
func (c *FirecrestRemoteSessionController) getCurrentStatus(ctx context.Context) (state models.RemoteSessionState, err error) {
	// TODO: also implement checking the http interface of the remote session through the tunnel

	if c.jobID == "" {
		return models.NotReady, nil
	}

	res, err := c.client.GetJobComputeSystemNameJobsJobIdGetWithResponse(ctx, c.systemName, c.jobID)
	if err != nil {
		return models.Failed, err
	}
	if res.JSON200 == nil {
		message := getErrorMessage(res.JSON4XX, res.JSON5XX)
		if message != "" {
			return models.Failed, fmt.Errorf("could not get job: %s", message)
		}
		return models.Failed, fmt.Errorf("could not get job: HTTP %d", res.StatusCode())
	}
	if res.JSON200.Jobs == nil {
		return models.Failed, fmt.Errorf("invalid job status response")
	}
	jobs := *res.JSON200.Jobs
	if len(jobs) < 1 {
		return models.Failed, fmt.Errorf("empty job response")
	}
	state, err = GetRemoteSessionState(jobs[0].Status.State)
	if err != nil {
		return models.Failed, err
	}
	return state, nil
}
