package api

import (
	"context"
	"net/http"
	"net/url"

	sessionRunners "github.com/SwissDataScienceCenter/amalthea/internal/remote/runners/api/session_runners"
	"github.com/SwissDataScienceCenter/amalthea/internal/remote/runners/auth"
)

type RenkuClient struct {
	sessionRunnersClient sessionRunners.ClientWithResponsesInterface

	auth                auth.RunnersAuth
	httpClient          *http.Client
	extraRequestEditors []RequestEditorFn
}

func NewRenkuClient(apiURL *url.URL, options ...RenkuClientOption) (rc *RenkuClient, err error) {
	rc = &RenkuClient{}

	apiURLStr := apiURL.ResolveReference(&url.URL{Path: "/api/data"}).String()

	for _, opt := range options {
		if err := opt(rc); err != nil {
			return nil, err
		}
	}
	// Create httpClient, if not already present
	if rc.httpClient == nil {
		rc.httpClient = http.DefaultClient
	}
	// Create clients
	sessionRunnersOpts := []sessionRunners.ClientOption{sessionRunners.WithHTTPClient(rc.httpClient)}
	for _, fn := range rc.extraRequestEditors {
		sessionRunnersOpts = append(sessionRunnersOpts, sessionRunners.WithRequestEditorFn(sessionRunners.RequestEditorFn(fn)))
	}
	if rc.auth != nil {
		sessionRunnersOpts = append(sessionRunnersOpts, sessionRunners.WithRequestEditorFn(sessionRunners.RequestEditorFn(rc.auth.RequestEditor())))
	}
	sessionRunnersClient, err := sessionRunners.NewClientWithResponses(apiURLStr, sessionRunnersOpts...)
	if err != nil {
		return nil, err
	}
	rc.sessionRunnersClient = sessionRunnersClient
	return rc, nil
}

func (rc *RenkuClient) SessionRunners() sessionRunners.ClientWithResponsesInterface {
	return rc.sessionRunnersClient
}

type RenkuClientOption func(*RenkuClient) error

type RequestEditorFn func(ctx context.Context, req *http.Request) error

func WithAuth(rcAuth auth.RunnersAuth) RenkuClientOption {
	return func(rc *RenkuClient) error {
		rc.auth = rcAuth
		return nil
	}
}

func WithHttpClient(httpClient *http.Client) RenkuClientOption {
	return func(rc *RenkuClient) error {
		rc.httpClient = httpClient
		return nil
	}
}

func WithExtraRequestEditors(editors ...RequestEditorFn) RenkuClientOption {
	return func(rc *RenkuClient) error {
		rc.extraRequestEditors = append(rc.extraRequestEditors, editors...)
		return nil
	}
}
