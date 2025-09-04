package firecrest

import (
	"os"
	"testing"
)

func TestFirecrestClient(t *testing.T) {
	firecrestURLStr := os.Getenv("FIRECREST_API_URL")
	if firecrestURLStr == "" {
		t.Skip("FIRECREST_API_URL needs to be set for this test")
	}
	authURLStr := os.Getenv("FIRECREST_AUTH_URL")
	if authURLStr == "" {
		t.Skip("FIRECREST_AUTH_URL needs to be set for this test")
	}
	firecrestClientID := os.Getenv("FIRECREST_CLIENT_ID")
	if firecrestClientID == "" {
		t.Skip("FIRECREST_CLIENT_ID needs to be set for this test")
	}
	firecrestClientSecret := os.Getenv("FIRECREST_CLIENT_SECRET")
	if firecrestClientSecret == "" {
		t.Skip("FIRECREST_CLIENT_SECRET needs to be set for this test")
	}

}
