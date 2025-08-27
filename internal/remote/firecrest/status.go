package firecrest

import (
	"fmt"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/controller"
)

// GetRemoteSessionState translates the state returned by the FirecREST API into a RemoteSessionState
//
// Reference: https://slurm.schedmd.com/job_state_codes.html
//
// We are assuming that the base states is what the FirecREST API returns.
func GetRemoteSessionState(status string) (state controller.RemoteSessionState, err error) {
	state, ok := statusMap[status]
	if ok {
		return state, nil
	}
	return controller.Failed, fmt.Errorf("status not recognized: %s", status)
}

var statusMap map[string]controller.RemoteSessionState = map[string]controller.RemoteSessionState{
	"BOOT_FAIL":     controller.Failed,
	"CANCELLED":     controller.Failed,
	"COMPLETED":     controller.Completed,
	"DEADLINE":      controller.Failed,
	"FAILED":        controller.Failed,
	"NODE_FAIL":     controller.Failed,
	"OUT_OF_MEMORY": controller.Failed,
	"PENDING":       controller.NotReady,
	"PREEMPTED":     controller.Failed,
	"RUNNING":       controller.Running,
	"SUSPENDED":     controller.Failed,
	"TIMEOUT":       controller.Failed,
}
