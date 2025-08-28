package firecrest

import (
	"fmt"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/models"
)

// GetRemoteSessionState translates the state returned by the FirecREST API into a RemoteSessionState
//
// Reference: https://slurm.schedmd.com/job_state_codes.html
//
// We are assuming that the base states is what the FirecREST API returns.
func GetRemoteSessionState(status string) (state models.RemoteSessionState, err error) {
	state, ok := statusMap[status]
	if ok {
		return state, nil
	}
	return models.Failed, fmt.Errorf("status not recognized: %s", status)
}

var statusMap map[string]models.RemoteSessionState = map[string]models.RemoteSessionState{
	"BOOT_FAIL":     models.Failed,
	"CANCELLED":     models.Failed,
	"COMPLETED":     models.Completed,
	"DEADLINE":      models.Failed,
	"FAILED":        models.Failed,
	"NODE_FAIL":     models.Failed,
	"OUT_OF_MEMORY": models.Failed,
	"PENDING":       models.NotReady,
	"PREEMPTED":     models.Failed,
	"RUNNING":       models.Running,
	"SUSPENDED":     models.Failed,
	"TIMEOUT":       models.Failed,
}
