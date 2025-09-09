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

// Package auth provides authentication methods for the FirecREST API
package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
)

// FirecrestAuth can inject authentication credentials into HTTP request to the FirecREST API
type FirecrestAuth interface {
	// RequestEditor returns a request editor to be used by FirecREST clients
	RequestEditor() RequestEditorFn
}

// RequestEditorFn  is the function signature for the RequestEditor callback function
type RequestEditorFn func(ctx context.Context, req *http.Request) error

func NewFirecrestAuth(cfg config.FirecrestAuthConfig, options ...FirecrestAuthOption) (auth FirecrestAuth, err error) {
	if cfg.Kind == config.FirecrestAuthConfigKindRenku {
		opts := make([]RenkuAuthOption, len(options))
		for i := range options {
			opts[i] = options[i].renkuAuthOption
		}
		return newRenkuAuth(
			cfg.TokenURI,
			string(cfg.RenkuAccessToken),
			string(cfg.RenkuRefreshToken),
			cfg.RenkuTokenURI,
			cfg.RenkuClientID,
			string(cfg.RenkuClientSecret),
			opts...,
		)
	}
	if cfg.Kind == config.FirecrestAuthConfigKindClientCredentials {
		opts := make([]FirecrestClientCredentialsAuthOption, len(options))
		for i := range options {
			opts[i] = options[i].firecrestClientCredentialsAuthOption
		}
		return newFirecrestClientCredentialsAuth(
			cfg.TokenURI,
			cfg.FirecrestClientID,
			string(cfg.FirecrestClientSecret),
			opts...,
		)
	}
	return nil, fmt.Errorf("auth '%s' is not supported", cfg.Kind)
}

// FirecrestAuthOption allows setting options
type FirecrestAuthOption struct {
	renkuAuthOption                      RenkuAuthOption
	firecrestClientCredentialsAuthOption FirecrestClientCredentialsAuthOption
}

func WithHttpClient(client *http.Client) FirecrestAuthOption {
	return FirecrestAuthOption{
		renkuAuthOption: func(auth *RenkuAuth) error {
			auth.httpClient = client
			return nil
		},
		firecrestClientCredentialsAuthOption: func(auth *FirecrestClientCredentialsAuth) error {
			auth.httpClient = client
			return nil
		},
	}
}
