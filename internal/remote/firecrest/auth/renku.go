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
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	sharedAuth "github.com/SwissDataScienceCenter/amalthea/internal/remote/auth/shared"
)

// RenkuAuth implements authentication as used in Renku:
//  1. The client has a `renkuAccessToken` and a `renkuRefreshToken`
//     which can be used to authenticate against the Renku API.
//  2. The client is also configured with a `firecrestTokenURI`
//     which provides a valid token for the FirecREST API.
type RenkuAuth struct {
	// firecrestTokenURI the URI used to request new access tokens
	firecrestTokenURI string

	// renkuAccessToken the current renku access token
	renkuAccessToken string
	// renkuAccessTokenExpiresAt is when the renku access token expires
	renkuAccessTokenExpiresAt time.Time
	// renkuRefreshToken the current renku refresh token
	renkuRefreshToken string
	// renkuTokenURI the URI used for obtaining new renku tokens
	renkuTokenURI string
	// renkuClientID the client ID to which the access token and refresh tokens were issued to
	renkuClientID string
	// renkuClientSecret the client secret for the client ID
	renkuClientSecret string
	// renkuAccessTokenLock ensures that we do not try to refresh
	// the renkuAccessToken twice at the same time.
	renkuAccessTokenLock *sync.RWMutex
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

// Check that RenkuAuth satisfies the FirecrestAuth interface
var _ FirecrestAuth = (*RenkuAuth)(nil)

func newRenkuAuth(firecrestTokenURI, renkuAccessToken, renkuRefreshToken, renkuTokenURI, renkuClientID, renkuClientSecret string, options ...RenkuAuthOption) (auth *RenkuAuth, err error) {
	auth = &RenkuAuth{
		firecrestTokenURI:         firecrestTokenURI,
		renkuAccessToken:          renkuAccessToken,
		renkuAccessTokenExpiresAt: time.Time{},
		renkuRefreshToken:         renkuRefreshToken,
		renkuTokenURI:             renkuTokenURI,
		renkuClientID:             renkuClientID,
		renkuClientSecret:         renkuClientSecret,
		renkuAccessTokenLock:      &sync.RWMutex{},
		accessToken:               "",
		accessTokenExpiresAt:      time.Time{},
		accessTokenLock:           &sync.RWMutex{},
	}
	for _, opt := range options {
		if err := opt(auth); err != nil {
			return nil, err
		}
	}
	// Validate auth
	if firecrestTokenURI == "" {
		return nil, fmt.Errorf("firecrestTokenURI is not set")
	}
	if renkuRefreshToken == "" {
		return nil, fmt.Errorf("renkuRefreshToken is not set")
	}
	if renkuTokenURI == "" {
		return nil, fmt.Errorf("renkuTokenURI is not set")
	}
	if renkuClientID == "" {
		return nil, fmt.Errorf("renkuClientID is not set")
	}
	if renkuClientSecret == "" {
		return nil, fmt.Errorf("renkuClientSecret is not set")
	}
	// Create httpClient, if not already present
	if auth.httpClient == nil {
		auth.httpClient = http.DefaultClient
	}
	return auth, nil
}

// RenkuAuthOption allows setting options
type RenkuAuthOption func(*RenkuAuth) error

// RequestEditor returns a request editor which injects a valid access token
// for FirecREST API requests.
func (a *RenkuAuth) RequestEditor() sharedAuth.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		if req.Header.Get("Authorization") != "" {
			return nil
		}
		token, err := a.GetAccessToken(ctx)
		if err != nil {
			return err
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		return nil
	}
}

func (a *RenkuAuth) GetAccessToken(ctx context.Context) (token string, err error) {
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
	if err := a.refreshAccessToken(ctx); err != nil {
		return token, err
	}
	a.accessTokenLock.RLock()
	defer a.accessTokenLock.RUnlock()
	return a.accessToken, nil
}

func (a *RenkuAuth) refreshAccessToken(ctx context.Context) error {
	renkuAccessToken, err := a.getRenkuAccessToken(ctx)
	if err != nil {
		return err
	}

	a.accessTokenLock.Lock()
	defer a.accessTokenLock.Unlock()

	childCtx, cancel := context.WithTimeoutCause(ctx, 30*time.Second, fmt.Errorf("authentication request timed out"))
	defer cancel()

	result, err := requestNewAccessTokenFromRenku(childCtx, a.httpClient, a.firecrestTokenURI, renkuAccessToken)
	if err != nil {
		return err
	}

	a.accessToken = result.AccessToken
	a.accessTokenExpiresAt = result.ExpiresAt
	return nil
}

func requestNewAccessTokenFromRenku(ctx context.Context, httpClient *http.Client, tokenURI, renkuAccessToken string) (result tokenResult, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURI, nil)
	if err != nil {
		return tokenResult{}, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", renkuAccessToken))
	res, err := httpClient.Do(req)
	if err != nil {
		return tokenResult{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return tokenResult{}, fmt.Errorf("token request failed: %s", res.Status)
	}
	tokenResponse, err := parseTokenResponse(res)
	if err != nil {
		return tokenResult{}, err
	}
	expiresAt, err := getJWTExpiresAt(tokenResponse.AccessToken)
	if err != nil {
		expiresAt = time.Now().Add(time.Second * time.Duration(tokenResponse.ExpiresIn))
	}
	return tokenResult{
		AccessToken:  tokenResponse.AccessToken,
		ExpiresAt:    expiresAt,
		RefreshToken: tokenResponse.RefreshToken,
	}, nil
}

func (a *RenkuAuth) getRenkuAccessToken(ctx context.Context) (token string, err error) {
	a.renkuAccessTokenLock.RLock()
	token = a.renkuAccessToken
	expiresAt := a.renkuAccessTokenExpiresAt
	a.renkuAccessTokenLock.RUnlock()

	leeway := 10 * time.Second
	deadline := time.Now().Add(-leeway)

	// Return the current token if it is still valid
	if token != "" && (expiresAt.IsZero() || expiresAt.Before(deadline)) {
		return token, nil
	}

	// Refresh the token
	if err := a.refreshRenkuAccessToken(ctx); err != nil {
		return token, err
	}
	a.renkuAccessTokenLock.RLock()
	defer a.renkuAccessTokenLock.RUnlock()
	return a.renkuAccessToken, nil
}

func (a *RenkuAuth) refreshRenkuAccessToken(ctx context.Context) error {
	a.renkuAccessTokenLock.Lock()
	defer a.renkuAccessTokenLock.Unlock()

	// NOTE: we do not let the refresh request be cancelled by the caller
	refreshCtx, cancel := context.WithTimeoutCause(context.WithoutCancel(ctx), 30*time.Second, fmt.Errorf("authentication request timed out"))
	defer cancel()

	result, err := requestNewAccessToken(refreshCtx, a.httpClient, a.renkuTokenURI, "refresh_token", a.renkuClientID, a.renkuClientSecret, a.renkuRefreshToken)
	if err != nil {
		return err
	}

	a.renkuAccessToken = result.AccessToken
	a.renkuAccessTokenExpiresAt = result.ExpiresAt
	return nil
}
