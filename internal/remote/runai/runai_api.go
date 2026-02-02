package runai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// TokenRequest represents the request payload for authentication
type TokenRequest struct {
	GrantType    string `json:"grantType"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// TokenResponse represents the response from the token endpoint
type TokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
}

// WorkloadsResponse represents the response from the workloads endpoint
type WorkloadsResponse struct {
	Workloads []map[string]interface{} `json:"workloads"`
}

// RunaiApi handles authentication and API calls to Run:AI
type RunaiApi struct {
	ClientID       string
	ClientSecret   string
	BaseURL        string
	Token          string
	TokenExpiresAt time.Time
	HTTPClient     *http.Client
}

// NewRunaiApi creates a new RunAI client instance
func NewRunaiApi(baseURL, clientID, clientSecret string) *RunaiApi {

	client := &RunaiApi{}

	if baseURL == "" {
		baseURL = "https://api.run.ai"
	}

	// create httpClient, if not already present
	if client.HTTPClient == nil {
		client.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &RunaiApi{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		BaseURL:      baseURL,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Authenticate obtains a new access token from the Run:AI API
func (c *RunaiApi) Authenticate() error {
	tokenReq := TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
	}

	jsonData, err := json.Marshal(tokenReq)
	if err != nil {
		return fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequest("POST", tokenUrl(c.BaseURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	c.Token = tokenResp.AccessToken
	c.TokenExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second) // refresh 1 min early
	slog.Info("Obtained new token")

	return nil
}

// EnsureToken ensures the access token is valid, refreshing if necessary
func (c *RunaiApi) EnsureToken() error {
	if c.Token == "" || time.Now().After(c.TokenExpiresAt) {
		slog.Info("Token expired or missing, authenticating...")
		return c.Authenticate()
	}
	return nil
}

// GetWorkloads fetches workloads from the Run:AI API
func (c *RunaiApi) GetWorkloads() ([]map[string]interface{}, error) {
	if err := c.EnsureToken(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", workloadsUrl(c.BaseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create workloads request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send workloads request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		slog.Warn("Token expired or invalid, re-authenticating...")
		if err := c.Authenticate(); err != nil {
			return nil, err
		}
		// Retry with new token
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
		resp, err = c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to retry workloads request: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("workloads request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var workloadsResp WorkloadsResponse
	if err := json.NewDecoder(resp.Body).Decode(&workloadsResp); err != nil {
		return nil, fmt.Errorf("failed to decode workloads response: %w", err)
	}

	slog.Info("Successfully fetched workloads")
	return workloadsResp.Workloads, nil
}

func tokenUrl(baseURL string) string {
	return fmt.Sprintf("%s/api/v1/token", baseURL)
}

func workloadsUrl(baseURL string) string {
	return fmt.Sprintf("%s/api/v1/workloads", baseURL)
}
