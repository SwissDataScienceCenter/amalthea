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
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// requestNewAccessToken implements requesting an access token from an OAuth 2.0 authorization server
func requestNewAccessToken(ctx context.Context, httpClient *http.Client, tokenURI, grantType, clientID, clientSecret, refreshToken string) (result tokenResult, err error) {
	postData := url.Values{}
	postData.Set("grant_type", grantType)
	postData.Set("client_id", clientID)
	postData.Set("client_secret", clientSecret)
	if refreshToken != "" {
		postData.Set("refresh_token", refreshToken)
	}
	body := strings.NewReader(postData.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, body)
	if err != nil {
		return tokenResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

type tokenResult struct {
	AccessToken  string
	ExpiresAt    time.Time
	RefreshToken string
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func parseTokenResponse(res *http.Response) (result tokenResponse, err error) {
	bodyBytes, err := io.ReadAll(res.Body)
	defer func() {
		err := res.Body.Close()
		if err != nil {
			slog.Warn("error while closing request body", "error", err)
		}
	}()
	if err != nil {
		return tokenResponse{}, err
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return tokenResponse{}, err
	}
	return result, nil
}

func getJWTExpiresAt(token string) (expiresAt time.Time, err error) {
	parser := jwt.NewParser()
	claims := jwt.RegisteredClaims{}
	if _, _, err := parser.ParseUnverified(token, &claims); err != nil {
		slog.Warn("cannot parse token claims", "error", err)
		return time.Now(), err
	}
	return claims.ExpiresAt.Time, nil
}
