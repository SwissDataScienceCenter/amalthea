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

package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	sharedAuth "github.com/SwissDataScienceCenter/amalthea/internal/remote/auth/shared"
)

// RunaiClientCredentialsAuth implements the "Client Credentials Grant"
// authentication flow for the Runai API.
type RunaiClientCredentialsAuth struct {
	// tokenURI the URI used to request new access tokens
	tokenURI string
	// clientID the client ID used for the "Client Credentials Grant" authentication flow
	clientID string
	// clientSecret the client secret used for the "Client Credentials Grant" authentication flow
	clientSecret string

	// accessToken is the current access token
	accessToken string
	// accessTokenExpiresAt is when the access token expires
	accessTokenExpiresAt time.Time
	// accessTokenLock ensures that we do not try to refresh
	// the accessToken twice at the same time.
	accessTokenLock *sync.RWMutex

	// httpClient is the HTTP client used to obtain access tokens
	httpClient *http.Client
}

// Check that RunaiClientCredentialsAuth satisfies the RunaiAuth interface
var _ RunaiAuth = (*RunaiClientCredentialsAuth)(nil)

func newRunaiClientCredentialsAuth(tokenURI, clientID, clientSecret string, options ...RunaiClientCredentialsAuthOption) (auth *RunaiClientCredentialsAuth, err error) {
	auth = &RunaiClientCredentialsAuth{
		tokenURI:             tokenURI,
		clientID:             clientID,
		clientSecret:         clientSecret,
		accessToken:          "",
		accessTokenExpiresAt: time.Time{},
		accessTokenLock:      &sync.RWMutex{},
	}
	for _, opt := range options {
		if err := opt(auth); err != nil {
			return nil, err
		}
	}
	// Validate auth
	if tokenURI == "" {
		return nil, fmt.Errorf("tokenURI is not set")
	}
	if clientID == "" {
		return nil, fmt.Errorf("clientID is not set")
	}
	if clientSecret == "" {
		return nil, fmt.Errorf("clientSecret is not set")
	}
	// Create httpClient, if not already present
	if auth.httpClient == nil {
		auth.httpClient = http.DefaultClient
	}
	return auth, nil
}

// RunaiClientCredentialsAuthOption allows setting options
type RunaiClientCredentialsAuthOption func(*RunaiClientCredentialsAuth) error

// RequestEditor returns a request editor which injects a valid access token
// for Runai API requests.
func (a *RunaiClientCredentialsAuth) RequestEditor() sharedAuth.RequestEditorFn {
	return sharedAuth.RequestEditorInjectAccessToken(a)
}

func (a *RunaiClientCredentialsAuth) GetAccessToken(ctx context.Context) (token string, err error) {
	a.accessTokenLock.RLock()
	token = a.accessToken
	expiresAt := a.accessTokenExpiresAt
	a.accessTokenLock.RUnlock()

	leeway := 10 * time.Second
	deadline := time.Now().Add(-leeway)

	// Return the current token if it is still valid
	if token != "" && (expiresAt.IsZero() || expiresAt.Before(deadline)) {
		return token, nil
	}

	// Refresh the token
	token, err = a.refreshAccessToken(ctx)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (a *RunaiClientCredentialsAuth) refreshAccessToken(ctx context.Context) (token string, err error) {
	a.accessTokenLock.Lock()
	defer a.accessTokenLock.Unlock()

	// Re-check if another goroutine has already refreshed the token
	token = a.accessToken
	expiresAt := a.accessTokenExpiresAt
	leeway := 10 * time.Second
	deadline := time.Now().Add(-leeway)

	// Return the current token if it is still valid
	if token != "" && (expiresAt.IsZero() || expiresAt.Before(deadline)) {
		return token, nil
	}

	// NOTE: we do not let the refresh request be cancelled by the caller
	refreshCtx, cancel := context.WithTimeoutCause(context.WithoutCancel(ctx), 30*time.Second, fmt.Errorf("authentication request timed out"))
	defer cancel()

	result, err := requestNewAccessToken(refreshCtx, a.httpClient, a.tokenURI, "client_credentials", a.clientID, a.clientSecret)
	if err != nil {
		return "", err
	}

	a.accessToken = result.AccessToken
	a.accessTokenExpiresAt = result.ExpiresAt
	return a.accessToken, nil
}

type tokenRequest struct {
	GrantType    string `json:"grantType"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type tokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
}

type tokenResult struct {
	AccessToken string
	ExpiresAt   time.Time
}

// requestNewAccessToken implements requesting an access token from a Runai API token endpoint
func requestNewAccessToken(ctx context.Context, httpClient *http.Client, tokenURI, grantType, clientID, clientSecret string) (result tokenResult, err error) {
	tokenReq := tokenRequest{
		GrantType:    grantType,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
	jsonData, err := json.Marshal(tokenReq)
	if err != nil {
		return tokenResult{}, err
	}

	body := bytes.NewBuffer(jsonData)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, body)
	if err != nil {
		return tokenResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := httpClient.Do(req)
	if err != nil {
		return tokenResult{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return tokenResult{}, fmt.Errorf("token request failed: %s", res.Status)
	}
	tokenResponse, err := parseTokenResponse(res)
	if err != nil {
		return tokenResult{}, err
	}
	expiresAt := time.Now().Add(time.Second * time.Duration(tokenResponse.ExpiresIn))
	return tokenResult{
		AccessToken: tokenResponse.AccessToken,
		ExpiresAt:   expiresAt,
	}, nil
}

func parseTokenResponse(res *http.Response) (result tokenResponse, err error) {
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return tokenResponse{}, err
	}
	return result, nil
}
