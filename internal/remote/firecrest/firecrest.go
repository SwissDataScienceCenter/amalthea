//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -generate types,client,spec -package firecrest -o firecrest_gen.go openapi_spec_downgraded.yaml

package firecrest

import (
	"context"
	"fmt"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/controller"
)

type FirecrestRemoteSessionController struct {
	client *ClientWithResponses

	jobID      string
	systemName string
}

func (c *FirecrestRemoteSessionController) Status(ctx context.Context) (state controller.RemoteSessionState, err error) {
	res, err := c.client.GetJobComputeSystemNameJobsJobIdGetWithResponse(ctx, c.systemName, c.jobID)
	if err != nil {
		return controller.Failed, err
	}
	if res.StatusCode() != 200 {
		message := ""
		if res.JSON4XX != nil {
			message = res.JSON4XX.Message
		} else if res.JSON5XX != nil {
			message = res.JSON5XX.Message
		}
		if message != "" {
			return controller.Failed, fmt.Errorf("could not get job: %s", message)
		}
		return controller.Failed, fmt.Errorf("could not get job: HTTP %d", res.StatusCode())
	}

	jobs, err := res.JSON200.Jobs.AsGetJobResponseJobs0()
	if err != nil {
		return controller.Failed, fmt.Errorf("could not parse job response: %w", err)
	}
	if len(jobs) < 1 {
		return controller.Failed, fmt.Errorf("empty job response")
	}
	state, err = GetRemoteSessionState(jobs[0].Status.State)
	if err != nil {
		return controller.Failed, err
	}
	return state, nil
}
