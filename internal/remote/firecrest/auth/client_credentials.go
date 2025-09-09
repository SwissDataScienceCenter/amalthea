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
)

// FirecrestClientCredentialsAuth implements the "Client Credentials Grant"
// authentication flow for the FirecREST API.
type FirecrestClientCredentialsAuth struct {
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

// Check that FirecrestClientCredentialsAuth satisfies the FirecrestAuth interface
var _ FirecrestAuth = (*FirecrestClientCredentialsAuth)(nil)

func newFirecrestClientCredentialsAuth(tokenURI, clientID, clientSecret string, options ...FirecrestClientCredentialsAuthOption) (auth *FirecrestClientCredentialsAuth, err error) {
	auth = &FirecrestClientCredentialsAuth{
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

// FirecrestClientCredentialsAuthOption allows setting options
type FirecrestClientCredentialsAuthOption func(*FirecrestClientCredentialsAuth) error

// func WithHttpClient(client *http.Client) FirecrestClientCredentialsAuthOption {
// 	return func(a *FirecrestClientCredentialsAuth) error {
// 		a.httpClient = client
// 		return nil
// 	}
// }

// RequestEditor returns a request editor which injects a valid access token
// for FirecREST API requests.
func (a *FirecrestClientCredentialsAuth) RequestEditor() RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		if req.Header.Get("Authorization") != "" {
			return nil
		}
		token, err := a.GetAccessToken()
		if err != nil {
			return err
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		return nil
	}
}

func (a *FirecrestClientCredentialsAuth) GetAccessToken() (token string, err error) {
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
	if err := a.refreshAccessToken(); err != nil {
		return token, err
	}
	a.accessTokenLock.RLock()
	defer a.accessTokenLock.RUnlock()
	return a.accessToken, nil
}

func (a *FirecrestClientCredentialsAuth) refreshAccessToken() error {
	a.accessTokenLock.Lock()
	defer a.accessTokenLock.Unlock()

	ctx, cancel := context.WithTimeoutCause(context.Background(), 30*time.Second, fmt.Errorf("authentication request timed out"))
	defer cancel()

	result, err := requestNewAccessToken(ctx, a.httpClient, a.tokenURI, "client_credentials", a.clientID, a.clientSecret, "")
	if err != nil {
		return err
	}

	a.accessToken = result.AccessToken
	a.accessTokenExpiresAt = result.ExpiresAt
	return nil
}
