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
	"context"
	"fmt"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/firecrest"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/runai"
)

type RemoteSessionController interface {
	Status(ctx context.Context) (models.RemoteSessionState, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Check that the backend-specific session controllers satisfy the RemoteSessionController interface
var _ RemoteSessionController = (*firecrest.FirecrestRemoteSessionController)(nil)
var _ RemoteSessionController = (*runai.RunaiRemoteSessionController)(nil)

// TODO: support different types of remote session controller
func NewRemoteSessionController(cfg config.RemoteSessionControllerConfig) (c RemoteSessionController, err error) {
	if cfg.RemoteKind == config.RemoteKindFirecrest {
		controller, err := firecrest.NewFirecrestRemoteSessionController(cfg)
		if err != nil {
			return nil, err
		}
		return controller, nil
	}
	if cfg.RemoteKind == config.RemoteKindRunai {
		controller, err := runai.NewRunaiRemoteSessionController(cfg)
		if err != nil {
			return nil, err
		}
		return controller, nil
	}

	return nil, fmt.Errorf("remote kind: '%s' is not supported", cfg.RemoteKind)
}
