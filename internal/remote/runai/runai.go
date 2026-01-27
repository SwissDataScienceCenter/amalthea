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

package runai

// TODO: Implement Run AI client and related types here

import (
	"net/http"
	"net/url"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/runai/auth"
)

type RunaiClient struct {
	RunaiApi
	auth       auth.RunaiAuth
	httpClient *http.Client
}

func NewRunaiClient(apiURL *url.URL, options ...RunaiClientOption) (rc *RunaiClient, err error) {
	rc = &RunaiClient{}
	for _, opt := range options {
		if err := opt(rc); err != nil {
			return nil, err
		}
	}
	// Create httpClient, if not already present
	if rc.httpClient == nil {
		rc.httpClient = http.DefaultClient
	}

	client, err := NewRunaiApi(apiURL.String(), rc.auth, rc.httpClient)
	if err != nil {
		return nil, err
	}
	rc.RunaiApi = *client
	return rc, nil
}

type RunaiClientOption func(*RunaiClient) error

func WithAuth(auth auth.RunaiAuth) RunaiClientOption {
	return func(rc *RunaiClient) error {
		rc.auth = auth
		return nil
	}
}

func WithHttpClient(httpClient *http.Client) RunaiClientOption {
	return func(rc *RunaiClient) error {
		rc.httpClient = httpClient
		return nil
	}
}
