package auth

import (
	"fmt"

	sharedAuth "github.com/SwissDataScienceCenter/amalthea/internal/remote/auth/shared"
	runnersConfig "github.com/SwissDataScienceCenter/amalthea/internal/remote/config/runners"
)

// RunnersAuth can inject authentication credentials into HTTP request to the Renku API
type RunnersAuth interface {
	sharedAuth.RemoteAuth
}

func NewRunnersAuth(cfg runnersConfig.RunnersAuthConfig) (auth RunnersAuth, err error) {
	if cfg.Kind == runnersConfig.RunnersAuthConfigKindRenkuV2 {
		return newRenkuAuth(string(cfg.AccessToken), string(cfg.RefreshToken), cfg.TokenURI)
	}
	return nil, fmt.Errorf("auth '%s' is not supported", cfg.Kind)
}
