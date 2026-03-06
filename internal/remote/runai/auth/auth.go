package auth

import (
	"fmt"
	"net/http"

	sharedAuth "github.com/SwissDataScienceCenter/amalthea/internal/remote/auth/shared"
	runaiConfig "github.com/SwissDataScienceCenter/amalthea/internal/remote/config/runai"
)

// RunaiAuth can inject authentication credentials into HTTP request to the Runai API
type RunaiAuth interface {
	sharedAuth.RemoteAuth
}

func NewRunaiAuth(cfg runaiConfig.RunaiAuthConfig, options ...RunaiAuthOption) (auth RunaiAuth, err error) {
	if cfg.Kind == runaiConfig.RunaiAuthConfigKindClientCredentials {
		opts := make([]RunaiClientCredentialsAuthOption, len(options))
		for i := range options {
			opts[i] = options[i].runaiClientCredentialsAuthOption
		}
		return newRunaiClientCredentialsAuth(
			cfg.TokenURI,
			cfg.RunaiClientID,
			string(cfg.RunaiClientSecret),
			opts...,
		)
	}
	return nil, fmt.Errorf("auth '%s' is not supported", cfg.Kind)
}

// RunaiAuthOption allows setting options
type RunaiAuthOption struct {
	runaiClientCredentialsAuthOption RunaiClientCredentialsAuthOption
}

func WithHttpClient(client *http.Client) RunaiAuthOption {
	return RunaiAuthOption{
		runaiClientCredentialsAuthOption: func(auth *RunaiClientCredentialsAuth) error {
			auth.httpClient = client
			return nil
		},
	}
}
