package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenAuthProxyRequiresAuthn(t *testing.T) {
	session := AmaltheaSession{
		Spec: AmaltheaSessionSpec{
			Authentication: &Authentication{
				Enabled:   true,
				Type:      Token,
				SecretRef: SessionSecretRef{Name: "auth-secret", Key: "config"},
			},
		},
	}
	output, err := session.auth()
	require.NoError(t, err)
	require.Len(t, output.Containers, 1)
	assert.Contains(t, output.Containers[0].Args, "--enable_authn")
}
