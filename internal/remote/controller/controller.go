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

package controller

import (
	"net/url"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/firecrest"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/firecrest/auth"
)

// type RemoteSessionController struct {
// }

// type RemoteSessionControllerInterface interface {
// 	Status(ctx context.Context)
// 	Start(ctx context.Context)
// 	Stop(ctx context.Context)
// }

// TODO: support different types of remote session controller
func NewRemoteSessionController(cfg config.RemoteSessionControllerConfig) (c *firecrest.FirecrestRemoteSessionController, err error) {
	firecrestAuth, err := auth.NewFirecrestClientCredentialsAuth(cfg.FirecrestAuthTokenURI, string(cfg.FirecrestClientID), string(cfg.FirecrestClientSecret))
	if err != nil {
		return nil, err
	}
	firecrestAPIURL, err := url.Parse(cfg.FirecrestAPIURL)
	if err != nil {
		return nil, err
	}
	firecrestClient, err := firecrest.NewFirecrestClient(firecrestAPIURL, firecrest.WithAuth(firecrestAuth))
	if err != nil {
		return nil, err
	}
	controller, err := firecrest.NewFirecrestRemoteSessionController(firecrestClient, "eiger")
	if err != nil {
		return nil, err
	}
	return controller, nil
}
