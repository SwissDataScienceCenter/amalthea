package runai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	sharedAuth "github.com/SwissDataScienceCenter/amalthea/internal/remote/auth/shared"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/runai/auth"
)

type ProjectsResponse struct {
	Projects []ProjectResponse `json:"projects"`
}

type ProjectResponse struct {
	Id                    string                 `json:"id"`
	Name                  string                 `json:"name"`
	Description           string                 `json:"description"`
	SchedulingRules       map[string]interface{} `json:"schedulingRules"`
	DefaultNodePools      []string               `json:"defaultNodePools"`
	NodeTypes             map[string]interface{} `json:"nodeTypes"`
	ClusterId             string                 `json:"clusterId"`
	ParentId              string                 `json:"parentId"`
	RequestedNamespace    string                 `json:"requestedNamespace"`
	EnforceRunaiScheduler bool                   `json:"enforceRunaiScheduler"`
}

// WorkloadsResponse represents the response from the workloads endpoint
type WorkloadsResponse struct {
	Workloads []WorkloadResponse `json:"workloads"`
}

type WorkloadResponse map[string]interface{}

type WorkspacePostBody struct {
	Name      string        `json:"name"`
	ProjectId string        `json:"projectId"`
	ClusterId string        `json:"clusterId"`
	Spec      WorkspaceSpec `json:"spec"`
}

type WorkspaceResponse struct {
	Name         string     `json:"name"`
	WorkloadId   string     `json:"workloadId"`
	ProjectId    string     `json:"projectId"`
	DepartmentId string     `json:"departmentId"`
	ClusterId    string     `json:"clusterId"`
	CreatedAt    time.Time  `json:"createdAt"`
	DeletedAt    *time.Time `json:"deletedAt,omitempty"`
	DesiredPhase string     `json:"desiredPhase"`
	ActualPhase  string     `json:"actualPhase"`
}

type WorkspaceSpec struct {
	Image                string                `json:"image"`
	WorkingDir           string                `json:"workingDir"`
	EnvironmentVariables []WorkspaceSpecEnvVar `json:"environmentVariables"`
}

type WorkspaceSpecEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type RunaiApiOption func(*RunaiApi) error

// RunaiApi handles authentication and API calls to Run:AI
type RunaiApi struct {
	BaseURL        string
	TokenExpiresAt time.Time
	Auth           auth.RunaiAuth
	HTTPClient     *http.Client

	// A list of callbacks for modifying requests which are generated before sending over
	// the network.
	RequestEditors []sharedAuth.RequestEditorFn
}

// NewRunaiApi creates a new RunAI client instance
func NewRunaiApi(baseURL string, auth auth.RunaiAuth, httpClient *http.Client) (*RunaiApi, error) {

	client := &RunaiApi{BaseURL: baseURL, Auth: auth, HTTPClient: httpClient}

	if client.BaseURL == "" {
		client.BaseURL = "https://api.run.ai"
	}

	// create httpClient, if not already present
	if client.HTTPClient == nil {
		client.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}

	client.RequestEditors = append(client.RequestEditors, sharedAuth.RequestEditorFn(client.Auth.RequestEditor()))

	return client, nil
}

func (c *RunaiApi) GetProjects(ctx context.Context) ([]ProjectResponse, error) {
	req, err := http.NewRequest("GET", projectsUrl(c.BaseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make get projects request: %w", err)
	}

	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req); err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send projects request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		resp, err = c.retryUnauthorizedRequest(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to retry projects request: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("projects request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var projectsResp ProjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&projectsResp); err != nil {
		return nil, fmt.Errorf("failed to decode projects response: %w", err)
	}

	slog.Info("Successfully fetched projects")
	return projectsResp.Projects, nil
}

func (c *RunaiApi) GetProjectsByName(ctx context.Context, name string) ([]ProjectResponse, error) {
	req, err := http.NewRequest("GET", projectsUrl(c.BaseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make get projects by name request: %w", err)
	}
	q := req.URL.Query()
	q.Add("filterBy", fmt.Sprintf("name==%s", name))
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req); err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send get projects by name request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		resp, err = c.retryUnauthorizedRequest(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to retry get projects by name request: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get projects by name request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var projectsResp ProjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&projectsResp); err != nil {
		return nil, fmt.Errorf("failed to decode projects response: %w", err)
	}

	slog.Info("Successfully fetched projects")
	return projectsResp.Projects, nil
}

func (c *RunaiApi) GetWorkloads(ctx context.Context) ([]WorkloadResponse, error) {
	req, err := http.NewRequest("GET", workloadsUrl(c.BaseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make get workloads request: %w", err)
	}

	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req); err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send get workloads request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		resp, err = c.retryUnauthorizedRequest(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to retry get workloads request: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get workloads request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var workloadsResp WorkloadsResponse
	if err := json.NewDecoder(resp.Body).Decode(&workloadsResp); err != nil {
		return nil, fmt.Errorf("failed to decode workloads response: %w", err)
	}

	slog.Info("Successfully fetched workloads")
	return workloadsResp.Workloads, nil
}

func (c *RunaiApi) CreateWorkspace(ctx context.Context, project ProjectResponse, jobName string, spec WorkspaceSpec) (*WorkspaceResponse, error) {
	req, err := http.NewRequest("POST", workspacesUrl(c.BaseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make create workspace request: %w", err)
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req); err != nil {
		return nil, err
	}

	// construct request body
	body := WorkspacePostBody{
		Name:      jobName,
		ProjectId: project.Id,
		ClusterId: project.ClusterId,
		Spec:      spec,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workspace request body: %w", err)
	}
	req.Body = io.NopCloser(io.Reader(bytes.NewReader(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send create workspace request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		resp, err = c.retryUnauthorizedRequest(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to retry create workspace request: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create workspace request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var workspacesResp WorkspaceResponse
	if err := json.NewDecoder(resp.Body).Decode(&workspacesResp); err != nil {
		return nil, fmt.Errorf("failed to decode workspace response: %w", err)
	}

	slog.Info("Successfully created workspace")
	return &workspacesResp, nil
}

func (c *RunaiApi) DeleteWorkspace(ctx context.Context, id string) error {
	workspaceUrl := fmt.Sprintf("%s/%s", workspacesUrl(c.BaseURL), id)
	req, err := http.NewRequest("DELETE", workspaceUrl, nil)
	if err != nil {
		return fmt.Errorf("failed to make delete workspace request: %w", err)
	}

	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req); err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete workspace request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		resp, err = c.retryUnauthorizedRequest(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to retry delete workspace request: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete workspace request failed with status %d: %s", resp.StatusCode, string(body))
	}

	slog.Info("Successfully deleted workspace", "workspaceId", id)
	return nil
}

func (c *RunaiApi) applyEditors(ctx context.Context, req *http.Request) error {
	for _, r := range c.RequestEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

func (c *RunaiApi) retryUnauthorizedRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	slog.Warn("Token expired or invalid, re-authenticating...")
	_, err := c.Auth.GetAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	// Retry with new token
	if err := c.applyEditors(ctx, req); err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	return resp, err
}

func projectsUrl(baseURL string) string {
	return fmt.Sprintf("%s/api/v1/org-unit/projects", baseURL)
}

func workloadsUrl(baseURL string) string {
	return fmt.Sprintf("%s/api/v1/workloads", baseURL)
}

func workspacesUrl(baseURL string) string {
	// https://run-ai-docs.nvidia.com/self-hosted/workloads-in-nvidia-run-ai/using-workspaces/quick-starts/jupyter-quickstart#api-1
	// https://run-ai-docs.nvidia.com/api/workloads/workspaces
	return fmt.Sprintf("%s/workspaces", workloadsUrl(baseURL))
}
