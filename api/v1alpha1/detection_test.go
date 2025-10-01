package v1alpha1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestDetectPlatformBasedOnAvailableAPIGroups(t *testing.T) {
	for _, tt := range []struct {
		apiGroupList *metav1.APIGroupList
		expected     ClusterType
	}{
		{
			&metav1.APIGroupList{},
			Kubernetes,
		},
		{
			&metav1.APIGroupList{
				Groups: []metav1.APIGroup{
					{
						Name: "project.openshift.io",
					},
				},
			},
			OpenShift,
		},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			output, err := json.Marshal(tt.apiGroupList)
			assert.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(output)
			assert.NoError(t, err)
		}))
		defer server.Close()

		clusterType, err := DetectClusterType(&rest.Config{Host: server.URL})
		require.NoError(t, err)

		assert.Equal(t, tt.expected, clusterType)
	}
}
