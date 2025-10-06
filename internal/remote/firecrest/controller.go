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
	"encoding/json"
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

	"github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
	"k8s.io/utils/ptr"
)

// The script submitted to start a new remote session.
//
//go:embed session_script.sh
var sessionScript string

var branchRegExp = regexp.MustCompile("[[]branch \"(.+)\"]")

var sessionScriptNoteRegExp = regexp.MustCompile("# NOTE FOR AMALTHEA MAINTAINERS(?s:.*)# END NOTE.*\n")

type FirecrestRemoteSessionController struct {
	client *FirecrestClient

	jobID      string
	systemName string
	partition  string

	// currentStatus the current session status
	currentStatus models.RemoteSessionState
	// currentStatusError the current session status error if any
	currentStatusError error
	// statusTicker a ticker which is used to update the session status in the background
	statusTicker *time.Ticker

	// fakeStart if true, do not start the remote session and print debug information
	fakeStart bool
}

func NewFirecrestRemoteSessionController(client *FirecrestClient, systemName, partition string, fakeStart bool) (c *FirecrestRemoteSessionController, err error) {
	c = &FirecrestRemoteSessionController{
		client:        client,
		jobID:         "",
		systemName:    systemName,
		partition:     partition,
		currentStatus: models.NotReady,
		statusTicker:  time.NewTicker(time.Minute),
		fakeStart:     fakeStart,
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
//
//nolint:gocyclo // TODO: can we break down session start?
func (c *FirecrestRemoteSessionController) Start(ctx context.Context) error {
	// Start a go routine to update the session status
	go c.periodicSessionStatus(ctx)

	if err := c.recoverJobID(); err != nil {
		return err
	}
	// We recovered an existing job ID, do nothing
	if c.jobID != "" {
		return nil
	}

	// do not do anything if `fakeStart` is true
	if c.fakeStart {
		c.jobID = "fake-job-id"
		slog.Info("fake start", "jobID", c.jobID, "env", os.Environ())
		return nil
	}

	// TODO: should the 15-minute timeout be configurable?
	startCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	if c.jobID != "" {
		return fmt.Errorf("a remote job is already running: %s", c.jobID)
	}

	// start by checking whether we can access the requested system
	system, err := c.GetCurrentSystem(startCtx)
	if err != nil {
		return err
	}

	userInfo, err := c.getUserInfo(startCtx)
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

	renkuProjectPath := strings.TrimSuffix(os.Getenv("RENKU_PROJECT_PATH"), "/")
	if renkuProjectPath == "" {
		renkuProjectPath = "dev-project"
		slog.Warn("RENKU_PROJECT_PATH is not defined", "defaultValue", renkuProjectPath)
	}
	renkuBaseURLPath := strings.TrimSuffix(os.Getenv("RENKU_BASE_URL_PATH"), "/")
	if renkuBaseURLPath == "" {
		renkuBaseURLPath = "dev-session"
		slog.Warn("RENKU_BASE_URL_PATH is not defined", "defaultValue", renkuBaseURLPath)
	}

	scratchPathRenku := path.Join(scratch.Path, userName, "renku")
	sessionPath := path.Join(scratchPathRenku, "sessions", renkuProjectPath, strings.TrimPrefix(renkuBaseURLPath, "/sessions"))

	slog.Info("determined session path", "sessionPath", sessionPath)

	// Setup secrets
	secretsPath := path.Join(sessionPath, "secrets")
	err = c.mkdir(startCtx, secretsPath, true /* createParents */)
	if err != nil {
		return err
	}
	// Makes sure that only the session owner can read session files
	err = c.chmod(startCtx, sessionPath, "700")
	if err != nil {
		return err
	}
	// TODO: get wstunnel_secret as a config value
	wstunnel_secret := os.Getenv("RSC_WSTUNNEL_SECRET")
	if wstunnel_secret != "" {
		err = c.uploadFile(startCtx, secretsPath, "wstunnel_secret", []byte(wstunnel_secret))
		if err != nil {
			return err
		}
	}
	// TODO: upload user secrets into secretsPath

	// Setup git repositories
	renkuWorkDir := os.Getenv("RENKU_WORKING_DIR")
	gitRepositories, err := c.collectGitRepositories(startCtx, renkuWorkDir)
	if err != nil {
		return err
	}
	slog.Info("collected git repositories", "gitRepositories", gitRepositories)
	for repo := range gitRepositories {
		repoGitDirPath := path.Join(sessionPath, "work", repo, ".git")
		err = c.mkdir(startCtx, repoGitDirPath, true /* createParents */)
		if err != nil {
			return err
		}
		gitConfigContents, err := os.ReadFile(gitRepositories[repo].ConfigPath)
		if err != nil {
			return err
		}
		err = c.uploadFile(startCtx, repoGitDirPath, "config", gitConfigContents)
		if err != nil {
			return err
		}
	}

	env := map[string]string{}
	// Copy the environment variables defined by the user
	for _, environ := range os.Environ() {
		key, val, _ := strings.Cut(environ, "=")
		if newKey, isRenkuEnv := strings.CutPrefix(key, "USER_ENV_"); isRenkuEnv {
			env[newKey] = val
		}
	}
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
		env["WSTUNNEL_SERVICE_PORT"] = fmt.Sprintf("%d", 443)                                   // wss port (same as https)
		env["WSTUNNEL_PATH_PREFIX"] = renkuBaseURLPath + "/" + v1alpha1.TunnelIngressPathSuffix // session path with tunnel
	}
	// Setup environment variables for git repositories
	repos := []string{}
	for repo := range gitRepositories {
		repos = append(repos, fmt.Sprintf("%s\t%s", repo, gitRepositories[repo].Branch))
	}
	env["GIT_REPOSITORIES"] = strings.Join(repos, "\n")
	// NOTE: we assume that the git proxy port is 65480 (default from the renku helm chart)
	env["GIT_PROXY_PORT"] = fmt.Sprintf("%d", 65480)        // git proxy port
	env["GIT_PROXY_HEALTH_PORT"] = fmt.Sprintf("%d", 65481) // git proxy port

	// Upload the session script
	sessionScriptFinal := c.renderSessionScript(sessionScript, system.FileSystems, secretsPath)
	err = c.uploadFile(ctx, sessionPath, "session_script.sh", []byte(sessionScriptFinal))
	if err != nil {
		return err
	}

	jobEnv := JobDescriptionModel_Env{}
	err = jobEnv.FromJobDescriptionModelEnv0(env)
	if err != nil {
		return err
	}
	job := JobDescriptionModel{
		Env:              &jobEnv,
		ScriptPath:       ptr.To(path.Join(sessionPath, "session_script.sh")),
		WorkingDirectory: sessionPath,
	}
	// The slurm account can be set by the user as an environment variable
	slurmAccount := os.Getenv("USER_ENV_SLURM_ACCOUNT")
	if slurmAccount != "" {
		job.Account = &slurmAccount
	}
	jobID, err := c.submitJob(startCtx, job)
	if err != nil {
		return err
	}
	c.jobID = jobID

	slog.Info("submitted job", "jobID", c.jobID)

	// Save the job ID for recovery
	if err := c.saveJobID(); err != nil {
		return err
	}

	return nil
}

// Stop stops the remote session using the FirecREST API.
//
// The caller needs to make sure Stop is not called before Start has returned.
func (c *FirecrestRemoteSessionController) Stop(ctx context.Context) error {
	// The remote job was never submitted, nothing to do
	if c.jobID == "" {
		slog.Info("no job to cancel")
		return nil
	}

	// Remove the saved state: if the session gets restarted later, we need to submit a fresh job
	if err := c.deleteSavedState(); err != nil {
		slog.Error("could not delete saved state before stopping", "error", err)
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

func (c *FirecrestRemoteSessionController) collectGitRepositories(ctx context.Context, workDir string) (gitRepositories map[string]gitRepository, err error) {
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
			return fmt.Errorf("could not run uploadFile: %s", message)
		}
		return fmt.Errorf("could not run uploadFile: HTTP %d", res.StatusCode())
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
			return fmt.Errorf("could not run mkdir: %s", message)
		}
		return fmt.Errorf("could not run mkdir: HTTP %d", res.StatusCode())
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
			return fmt.Errorf("could not run chmod: %s", message)
		}
		return fmt.Errorf("could not run chmod: HTTP %d", res.StatusCode())
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
			return "", fmt.Errorf("could not submit job: %s", message)
		}
		return "", fmt.Errorf("could not submit job: HTTP %d", res.StatusCode())
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
func (c *FirecrestRemoteSessionController) periodicSessionStatus(ctx context.Context) {
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

func (c *FirecrestRemoteSessionController) renderSessionScript(sessionScript string, fileSystems *[]FileSystem, secretsPath string) string {
	return renderSessionScriptStatic(sessionScript, c.partition, fileSystems, secretsPath)
}

func renderSessionScriptStatic(sessionScript, partition string, fileSystems *[]FileSystem, secretsPath string) string {
	sessionScriptFinal := removeMaintainersNotesFromScript(sessionScript)
	sessionScriptFinal = addSbatchDirectivesToScript(sessionScriptFinal, partition)
	sessionScriptFinal = addSessionMountsToScript(sessionScriptFinal, fileSystems, secretsPath)
	return sessionScriptFinal
}

func removeMaintainersNotesFromScript(sessionScript string) string {
	return sessionScriptNoteRegExp.ReplaceAllString(sessionScript, "")
}

func addSbatchDirectivesToScript(sessionScript, partition string) string {
	directives := []string{
		"#SBATCH --nodes=1",
		"#SBATCH --ntasks-per-node=1",
	}
	if partition != "" {
		directives = append(directives, fmt.Sprintf("#SBATCH --partition=%s", partition))
	}
	// The slurm account can be set by the user as an environment variable
	slurmAccount := os.Getenv("USER_ENV_SLURM_ACCOUNT")
	if slurmAccount != "" {
		directives = append(directives, fmt.Sprintf("#SBATCH --account=%s", slurmAccount))
	}
	directivesStr := strings.Join(directives, "\n")
	return strings.Replace(sessionScript, "#{{SBATCH_DIRECTIVES}}", directivesStr, 1)
}

func addSessionMountsToScript(sessionScript string, fileSystems *[]FileSystem, secretsPath string) string {
	if fileSystems == nil {
		return strings.Replace(sessionScript, "#{{SBATCH_DIRECTIVES}}", "", 1)
	}
	// Collect file systems we want to mount
	var scratch, project, home *FileSystem
	for _, fs := range *fileSystems {
		switch fs.DataType {
		case Scratch:
			scratch = &fs
		case Store:
			project = &fs
		case Users:
			home = &fs
		}
	}

	mounts := []string{}
	if scratch != nil {
		mounts = append(mounts, scratch.Path)
	}
	if project != nil {
		mounts = append(mounts, project.Path)
	}
	// TODO: Try to mount home at its location (need to handle ~/.bashrc)
	// TODO: Alternatively, copy the contents in the container
	if home != nil {
		mounts = append(mounts, fmt.Sprintf("%s:/home%s:ro", home.Path, home.Path))
	}

	// Add the secrets mount
	mounts = append(mounts, fmt.Sprintf("%s:/secrets:ro", secretsPath))
	// Format mount list
	for i := range mounts {
		mounts[i] = fmt.Sprintf("    \"%s\",", mounts[i])
	}

	mountsStr := fmt.Sprintf("mounts = [\n%s\n]", strings.Join(mounts, "\n"))
	return strings.Replace(sessionScript, "#{{SESSION_MOUNTS}}", mountsStr, 1)
}

func (c *FirecrestRemoteSessionController) saveJobID() error {
	if c.jobID == "" {
		return fmt.Errorf("cannot save, job ID is not defined")
	}
	saveDirPath := c.getSaveDirPath()
	if err := os.MkdirAll(saveDirPath, 0755); err != nil {
		return err
	}
	savePath := c.getSavePath()

	state := savedState{
		JobID: c.jobID,
	}
	contents, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(savePath, contents, 0644)
}

func (c *FirecrestRemoteSessionController) deleteSavedState() error {
	savePath := c.getSavePath()
	return os.Remove(savePath)
}

func (c *FirecrestRemoteSessionController) recoverJobID() error {
	contents, err := os.ReadFile(c.getSavePath())
	if err != nil {
		return nil
	}

	var state savedState
	if err := json.Unmarshal(contents, &state); err != nil {
		return err
	}

	if state.JobID != "" {
		c.jobID = state.JobID
		slog.Info("recovered job ID", "jobID", c.jobID)
	}
	return nil
}

type savedState struct {
	JobID string `json:"job_id"`
}

func (c *FirecrestRemoteSessionController) getSaveDirPath() string {
	renkuMountDir := os.Getenv("RENKU_MOUNT_DIR")
	return path.Join(renkuMountDir, ".rsc") // NOTE: "rsc" stands for "Remote Session Controller"
}

func (c *FirecrestRemoteSessionController) getSavePath() string {

	return path.Join(c.getSaveDirPath(), "state.json")
}
