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

package firecrest

import (
	"context"
	"fmt"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
)

type FirecrestRemoteSessionController struct {
	client *FirecrestClient

	jobID      string
	systemName string
}

func NewFirecrestRemoteSessionController(client *FirecrestClient, systemName string) (c *FirecrestRemoteSessionController, err error) {
	c = &FirecrestRemoteSessionController{
		client:     client,
		jobID:      "",
		systemName: systemName,
	}
	// Validate controller
	if c.client == nil {
		return nil, fmt.Errorf("client is not set")
	}
	if c.systemName == "" {
		return nil, fmt.Errorf("systemName is not set")
	}
	return c, nil
}

func (c *FirecrestRemoteSessionController) CheckSystemAccess(ctx context.Context) error {
	res, err := c.client.GetSystemsStatusSystemsGetWithResponse(ctx)
	if err != nil {
		return err
	}
	if res.JSON200 == nil {
		return fmt.Errorf("empty response")
	}
	for _, sys := range res.JSON200.Systems {
		if sys.Name == c.systemName {
			return nil
		}
	}
	return fmt.Errorf("system '%s' not found", c.systemName)
}

func (c *FirecrestRemoteSessionController) Status(ctx context.Context) (state models.RemoteSessionState, err error) {
	if c.jobID == "" {
		return models.NotReady, nil
	}

	res, err := c.client.GetJobComputeSystemNameJobsJobIdGetWithResponse(ctx, c.systemName, c.jobID)
	if err != nil {
		return models.Failed, err
	}
	if res.StatusCode() != 200 {
		message := ""
		if res.JSON4XX != nil {
			message = res.JSON4XX.Message
		} else if res.JSON5XX != nil {
			message = res.JSON5XX.Message
		}
		if message != "" {
			return models.Failed, fmt.Errorf("could not get job: %s", message)
		}
		return models.Failed, fmt.Errorf("could not get job: HTTP %d", res.StatusCode())
	}

	jobs, err := res.JSON200.Jobs.AsGetJobResponseJobs0()
	if err != nil {
		return models.Failed, fmt.Errorf("could not parse job response: %w", err)
	}
	if len(jobs) < 1 {
		return models.Failed, fmt.Errorf("empty job response")
	}
	state, err = GetRemoteSessionState(jobs[0].Status.State)
	if err != nil {
		return models.Failed, err
	}
	return state, nil
}
