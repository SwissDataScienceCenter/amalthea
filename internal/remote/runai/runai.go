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
)

type RunaiClient struct {
	httpClient *http.Client
}

func NewRunaiClient(apiURL *url.URL, options ...RunaiClientOption) (fc *RunaiClient, err error) {
	fc = &RunaiClient{}
	for _, opt := range options {
		if err := opt(fc); err != nil {
			return nil, err
		}
	}
	// Create httpClient, if not already present
	if fc.httpClient == nil {
		fc.httpClient = http.DefaultClient
	}
	// Create client
	return fc, nil
}

type RunaiClientOption func(*RunaiClient) error

func WithHttpClient(httpClient *http.Client) RunaiClientOption {
	return func(fc *RunaiClient) error {
		fc.httpClient = httpClient
		return nil
	}
}
