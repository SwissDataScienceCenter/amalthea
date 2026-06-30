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
	"github.com/SwissDataScienceCenter/amalthea/internal/common"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/firecrest/auth"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
	"github.com/SwissDataScienceCenter/amalthea/internal/utils"
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

	// stdoutPath and stderrPath are the paths to the Slurm stdout/stderr files on the cluster filesystem.
	stdoutPath string
	stderrPath string
	// stdoutOffset and stderrOffset track how many bytes have been consumed from each log file.
	stdoutOffset int
	stderrOffset int
	// stdoutBuf and stderrBuf hold partial lines between fetches.
	stdoutBuf bytes.Buffer
	stderrBuf bytes.Buffer
}

func NewFirecrestRemoteSessionController(cfg config.RemoteSessionControllerConfig) (c *FirecrestRemoteSessionController, err error) {
	firecrestAuth, err := auth.NewFirecrestAuth(cfg.Firecrest.AuthConfig)
	if err != nil {
		return nil, err
	}
	firecrestAPIURL, err := url.Parse(cfg.Firecrest.APIURL)
	if err != nil {
		return nil, err
	}
	firecrestClient, err := NewFirecrestClient(firecrestAPIURL, WithAuth(firecrestAuth))
	if err != nil {
		return nil, err
	}
	c = &FirecrestRemoteSessionController{
		client:        firecrestClient,
		jobID:         "",
		systemName:    cfg.Firecrest.SystemName,
		partition:     cfg.Firecrest.Partition,
		currentStatus: models.NotReady,
		statusTicker:  time.NewTicker(time.Minute),
		fakeStart:     cfg.FakeStart,
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

func walkIfExists(root string, filter func(dir os.DirEntry) bool, process func(dir os.DirEntry) error, onceBefore ...func() error) error {
	var err error

	dirEntries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if onceBefore != nil && onceBefore[0] != nil {
		if err = onceBefore[0](); err != nil {
			return err
		}
	}

	for _, dirEntry := range dirEntries {
		if filter(dirEntry) {
			if err = process(dirEntry); err != nil {
				return err
			}
		}
	}

	return nil
}

func ensurePrivateFolder(c *FirecrestRemoteSessionController, ctx context.Context, remotePath string) error {
	// Ensure the remote folder exists
	err := c.mkdir(ctx, remotePath, true)
	if err != nil {
		return err
	}

	err = c.chmod(ctx, remotePath, "700")
	if err != nil {
		return err
	}
	return err
}

func (c *FirecrestRemoteSessionController) uploadSecretFromBuffer(ctx context.Context, remotePath, filename string, content []byte) error {
	var err error
	// ignore errors, we want this just to make sure we can write to it if the files exists
	_ = c.chmod(ctx, path.Join(remotePath, filename), "700")

	if err = c.uploadFile(ctx, remotePath, filename, content); err != nil {
		return err
	}

	if err = c.chmod(ctx, path.Join(remotePath, filename), "400"); err != nil {
		return err
	}

	return err
}

func (c *FirecrestRemoteSessionController) uploadSecret(ctx context.Context, localPath, remotePath, filename string) error {
	var err error
	var content []byte

	if content, err = os.ReadFile(path.Join(localPath, filename)); err != nil {
		return err
	}

	return c.uploadSecretFromBuffer(ctx, remotePath, filename, content)
}

func (c *FirecrestRemoteSessionController) uploadOptionalSecretFromBuffer(ctx context.Context, remotePath, filename string, content []byte) error {
	if len(content) > 0 {
		return c.uploadSecretFromBuffer(ctx, remotePath, filename, content)
	}
	return nil
}

func (c *FirecrestRemoteSessionController) uploadDataSource(ctx context.Context, remotePath string, dataSource *common.DataConnector) error {
	var err error

	if err = c.uploadSecretFromBuffer(ctx, remotePath, "remote", []byte(dataSource.Remote)); err != nil {
		return err
	}

	if err = c.uploadSecretFromBuffer(ctx, remotePath, "remotePath", []byte(dataSource.RemotePath)); err != nil {
		return err
	}

	if err = c.uploadOptionalSecretFromBuffer(ctx, remotePath, "mountOpt", []byte(dataSource.MountOpt)); err != nil {
		return err
	}

	if err = c.uploadOptionalSecretFromBuffer(ctx, remotePath, "vfsOpt", []byte(dataSource.VfsOpt)); err != nil {
		return err
	}

	var configData *string
	if configData, err = dataSource.ConfigData(ctx); err != nil {
		return err
	}

	if err = c.uploadSecretFromBuffer(ctx, remotePath, "configData", []byte(*configData)); err != nil {
		return err
	}

	// TODO: Write read-only extra flag from "DATA_SOURCES"
	return nil
}

func isFile(dir os.DirEntry) bool {
	return !dir.IsDir()
}

func isDir(dir os.DirEntry) bool {
	return dir.IsDir()
}

func (c *FirecrestRemoteSessionController) uploadSecrets(ctx context.Context, localPath, remotePath string) error {
	return walkIfExists(
		localPath,
		func(dir os.DirEntry) bool {
			return isFile(dir) && !strings.HasPrefix(dir.Name(), "..")
		},
		func(a os.DirEntry) error {
			filename := a.Name()
			return c.uploadSecret(ctx, localPath, remotePath, filename)
		},
		func() error {
			return ensurePrivateFolder(c, ctx, remotePath)
		},
	)
}

func (c *FirecrestRemoteSessionController) uploadDataConnectorSecrets(ctx context.Context, localPath, remotePath string) error {
	return walkIfExists(
		localPath,
		isDir,
		func(dir os.DirEntry) error {
			ds, err := common.LoadDataSource(localPath, dir.Name())
			if err != nil {
				return err
			}
			return c.uploadDataSource(ctx, remotePath, ds)
		},
		func() error {
			return ensurePrivateFolder(c, ctx, remotePath)
		},
	)
}

// Start sets up and starts the remote session using the FirecREST API
//
//nolint:gocyclo // TODO: can we break down session start?
func (c *FirecrestRemoteSessionController) Start(ctx context.Context) error {
	// Start a go routine to update the session status
	go c.periodicSessionStatus(ctx)

	if err := c.recoverState(); err != nil {
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

	scratch := getPreferredScratch(system.FileSystems)
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

	srunRenkuPath := path.Join(scratch.Path, userName, "renku")
	srunSessionPath := path.Join(srunRenkuPath, "sessions", renkuProjectPath, strings.TrimPrefix(renkuBaseURLPath, "/sessions"))

	slog.Info("determined session path", "sessionPath", srunSessionPath)

	// Setup secrets
	proxyContainerSecretsPath, exists := os.LookupEnv("RENKU_SECRETS_PATH")
	if !exists {
		proxyContainerSecretsPath = "/secrets"
	}

	// Makes sure that only the session owner can read session files
	if err = ensurePrivateFolder(c, startCtx, srunSessionPath); err != nil {
		return err
	}

	srunSecretsPath := path.Join(srunSessionPath, "secrets")
	if err = ensurePrivateFolder(c, startCtx, srunSecretsPath); err != nil {
		return err
	}

	err = c.uploadSecret(startCtx, common.UserSecretProxyFolder, srunSecretsPath, "wstunnel_secret")
	if err != nil {
		return err
	}

	// Upload user secrets
	srunUserSecretsPath := path.Join(srunSecretsPath, "user")
	proxyContainerUserSecretsPath := proxyContainerSecretsPath // the secrets are stored directly, as is

	var dirEntries []os.DirEntry
	if dirEntries, err = os.ReadDir(proxyContainerUserSecretsPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(dirEntries) == 0 {
		// There are no secrets to mount
		srunUserSecretsPath = ""
	} else {
		if err = c.uploadSecrets(startCtx, proxyContainerUserSecretsPath, srunUserSecretsPath); err != nil {
			return err
		}
	}

	srunDataConnectorsPath := path.Join(srunSecretsPath, "data_connectors")
	// Can't put them under /secrets as we can't create a subfolder in it as the volume is RO, so we put them at the root
	proxyContainerDataConnectorsPath := common.DataConnectorSecretsProxyFolder
	if dirEntries, err = os.ReadDir(proxyContainerDataConnectorsPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(dirEntries) > 0 {
		// Upload data source secrets
		if err = c.uploadDataConnectorSecrets(startCtx, proxyContainerDataConnectorsPath, srunDataConnectorsPath); err != nil {
			return err
		}
	}

	// Setup git repositories
	renkuWorkDir := os.Getenv("RENKU_WORKING_DIR")
	gitRepositories, err := c.collectGitRepositories(startCtx, renkuWorkDir)
	if err != nil {
		return err
	}
	slog.Info("collected git repositories", "gitRepositories", gitRepositories)
	for repo := range gitRepositories {
		repoGitDirPath := path.Join(srunSessionPath, "work", repo, ".git")
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

	// Format the REMOTE_SESSION_IMAGE environment variable for enroot
	enrootImage, err := utils.EnrootImageFormat(env["REMOTE_SESSION_IMAGE"])
	if err == nil {
		env["REMOTE_SESSION_IMAGE"] = enrootImage
	} else {
		// TODO: Is this the best way to report this?
		slog.Warn("could not format REMOTE_SESSION_IMAGE for enroot, using the original value", "REMOTE_SESSION_IMAGE", env["REMOTE_SESSION_IMAGE"], "error", err)
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
	// We mirror the RENKU_SECRETS_PATH in the proxy and container final container, as it contains the user secrets, at
	// the user secrets lcoaltion (configurable by end-user)
	sessionScriptFinal := c.renderSessionScript(sessionScript, system.FileSystems, srunUserSecretsPath, proxyContainerSecretsPath)
	err = c.uploadFile(startCtx, srunSessionPath, "session_script.sh", []byte(sessionScriptFinal))
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
		ScriptPath:       ptr.To(path.Join(srunSessionPath, "session_script.sh")),
		WorkingDirectory: srunSessionPath,
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

	// After submission, construct the log file paths that Slurm will create on the cluster.
	c.stdoutPath = path.Join(srunSessionPath, fmt.Sprintf("slurm-%s.out", c.jobID))
	c.stderrPath = path.Join(srunSessionPath, fmt.Sprintf("slurm-%s.err", c.jobID))

	metaRes, err := c.client.GetJobMetadataComputeSystemNameJobsJobIdMetadataGetWithResponse(startCtx, c.systemName, c.jobID)
	if err != nil {
		slog.Warn("could not get job metadata, falling back to default log paths", "error", err)
	} else if metaRes.JSON200 == nil || metaRes.JSON200.Jobs == nil || len(*metaRes.JSON200.Jobs) == 0 {
		slog.Warn("job metadata response empty, falling back to default log paths")
	} else {
		meta := (*metaRes.JSON200.Jobs)[0]
		if meta.StandardOutput != nil && *meta.StandardOutput != "" {
			p := *meta.StandardOutput
			if !path.IsAbs(p) {
				p = path.Join(srunSessionPath, p)
			}
			c.stdoutPath = p
		}
		if meta.StandardError != nil && *meta.StandardError != "" {
			p := *meta.StandardError
			if !path.IsAbs(p) {
				p = path.Join(srunSessionPath, p)
			}
			c.stderrPath = p
		} else {
			// Slurm writes stderr to stdout when StandardError is not explicitly set.
			c.stderrPath = c.stdoutPath
		}
	}

	// Save the state for recovery
	if err := c.saveState(); err != nil {
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
	return *res.JSON201.JobId, nil
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
				// Fetch any new session logs from the cluster filesystem.
				c.fetchSessionLogs(childCtx)
				// Persist offsets so we can resume after a restart.
				if c.jobID != "" {
					if err := c.saveState(); err != nil {
						slog.Warn("failed to save controller state", "error", err)
					}
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

// fetchSessionLogs retrieves any new lines from the remote Slurm stdout/stderr files
// via FirecREST and writes them to this container's stdout so they appear in kubectl logs.
func (c *FirecrestRemoteSessionController) fetchSessionLogs(ctx context.Context) {
	if c.jobID == "" || c.stdoutPath == "" {
		return
	}

	for _, stream := range c.streamsToFetch() {
		c.fetchLogStream(ctx, stream)
	}
}

func (c *FirecrestRemoteSessionController) streamsToFetch() []string {
	streams := []string{"stdout"}
	if c.stderrPath != "" && c.stderrPath != c.stdoutPath {
		streams = append(streams, "stderr")
	}
	return streams
}

func (c *FirecrestRemoteSessionController) fetchLogStream(ctx context.Context, stream string) {
	var filePath string
	var offset *int
	var buf *bytes.Buffer

	switch stream {
	case "stdout":
		filePath = c.stdoutPath
		offset = &c.stdoutOffset
		buf = &c.stdoutBuf
	case "stderr":
		filePath = c.stderrPath
		offset = &c.stderrOffset
		buf = &c.stderrBuf
	default:
		slog.Warn("unknown stream type", "stream", stream)
		return
	}

	size := 524288 // 512KB
	apiOffset := *offset + buf.Len()
	params := GetViewFilesystemSystemNameOpsViewGetParams{
		Path:   filePath,
		Offset: &apiOffset,
		Size:   &size,
	}

	res, err := c.client.GetViewFilesystemSystemNameOpsViewGetWithResponse(ctx, c.systemName, &params)
	if err != nil {
		slog.Warn("failed to fetch session log", "stream", stream, "error", err)
		return
	}
	if res.JSON200 == nil || res.JSON200.Output == nil {
		// File not available yet or empty — this is expected early in the job lifecycle.
		return
	}

	content := *res.JSON200.Output
	if content == "" {
		return
	}

	// Append new content and flush any complete lines.
	if _, err := buf.WriteString(content); err != nil {
		slog.Warn("failed to buffer session log content", "stream", stream, "error", err)
		return
	}
	for {
		data := buf.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx == -1 {
			break
		}
		line := string(data[:idx])
		if _, err := fmt.Fprintf(os.Stdout, "[session/%s] %s\n", stream, line); err != nil {
			slog.Warn("failed to write session log", "stream", stream, "error", err)
		}
		buf.Next(idx + 1)
		*offset += idx + 1
	}
}

func (c *FirecrestRemoteSessionController) renderSessionScript(sessionScript string, fileSystems *[]FileSystem, nodeSecretsPath, containerSecretsPath string) string {
	return renderSessionScriptStatic(sessionScript, c.partition, fileSystems, nodeSecretsPath, containerSecretsPath)
}

func renderSessionScriptStatic(sessionScript, partition string, fileSystems *[]FileSystem, nodeSecretsPath, containerSecretsPath string) string {
	sessionScriptFinal := removeMaintainersNotesFromScript(sessionScript)
	sessionScriptFinal = addSbatchDirectivesToScript(sessionScriptFinal, partition)
	sessionScriptFinal = addSessionMountsToScript(sessionScriptFinal, fileSystems, nodeSecretsPath, containerSecretsPath)
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
	return strings.Replace(sessionScript, "#{{SBATCH_DIRECTIVES_PLACEHOLDER}}", directivesStr, 1)
}

func addSessionMountsToScript(sessionScript string, fileSystems *[]FileSystem, nodeSecretPath, containerSecretPath string) string {
	if fileSystems == nil {
		return strings.Replace(sessionScript, "#{{SESSION_MOUNTS_PLACEHOLDER}}", "", 1)
	}

	// Collect file systems we want to mount as "SRC:DST[:FLAG]" strings
	var mounts []string
	for _, fs := range *fileSystems {
		switch fs.DataType {
		case Users:
			// TODO: Try to mount home at its location (need to handle ~/.bashrc)
			// TODO: Alternatively, copy the contents in the container
			mounts = append(mounts, fmt.Sprintf("%s:/home%s:ro", fs.Path, fs.Path))
		default:
			// Identity mapping of the host mounts
			mounts = append(mounts, fmt.Sprintf("%s:%s", fs.Path, fs.Path))
		}
	}

	// Add the secrets mount
	if nodeSecretPath != "" && containerSecretPath != "" {
		mounts = append(mounts, fmt.Sprintf("%s:%s:ro", nodeSecretPath, containerSecretPath))
	}

	// Format mount list
	for i := range mounts {
		mounts[i] = fmt.Sprintf("\"%s\"", mounts[i])
	}
	mountsStr := fmt.Sprintf("--container-mounts=%s", strings.Join(mounts, ","))
	return strings.Replace(sessionScript, "#{{SESSION_MOUNTS_PLACEHOLDER}}", mountsStr, 1)
}

func (c *FirecrestRemoteSessionController) saveState() error {
	if c.jobID == "" {
		return fmt.Errorf("cannot save, job ID is not defined")
	}
	saveDirPath := c.getSaveDirPath()
	if err := os.MkdirAll(saveDirPath, 0755); err != nil {
		return err
	}
	savePath := c.getSavePath()

	state := savedState{
		JobID:        c.jobID,
		StdoutPath:   c.stdoutPath,
		StderrPath:   c.stderrPath,
		StdoutOffset: c.stdoutOffset,
		StderrOffset: c.stderrOffset,
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

func (c *FirecrestRemoteSessionController) recoverState() error {
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
	c.stdoutPath = state.StdoutPath
	c.stderrPath = state.StderrPath
	c.stdoutOffset = state.StdoutOffset
	c.stderrOffset = state.StderrOffset
	return nil
}

type savedState struct {
	JobID        string `json:"job_id"`
	StdoutPath   string `json:"stdout_path,omitempty"`
	StderrPath   string `json:"stderr_path,omitempty"`
	StdoutOffset int    `json:"stdout_offset,omitempty"`
	StderrOffset int    `json:"stderr_offset,omitempty"`
}

func (c *FirecrestRemoteSessionController) getSaveDirPath() string {
	renkuMountDir := os.Getenv("RENKU_MOUNT_DIR")
	return path.Join(renkuMountDir, ".rsc") // NOTE: "rsc" stands for "Remote Session Controller"
}

func (c *FirecrestRemoteSessionController) getSavePath() string {

	return path.Join(c.getSaveDirPath(), "state.json")
}

func getPreferredScratch(fileSystems *[]FileSystem) *FileSystem {
	var scratch *FileSystem
	if fileSystems == nil {
		return scratch
	}
	// Find the default work dir if it exists
	for _, fs := range *fileSystems {
		if fs.DataType == Scratch && fs.DefaultWorkDir != nil && *fs.DefaultWorkDir {
			scratch = &fs
		}
	}
	if scratch != nil {
		return scratch
	}
	// Get the first scratch file system otherwise
	for _, fs := range *fileSystems {
		if fs.DataType == Scratch {
			scratch = &fs
		}
	}
	return scratch
}
