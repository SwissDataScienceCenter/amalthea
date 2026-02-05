package shared

import (
	"context"
	"fmt"
	"net/http"
)

// RemoteAuth can inject authentication credentials into an HTTP request
type RemoteAuth interface {
	// RequestEditor returns a request editor to be used by Remote clients
	RequestEditor() RequestEditorFn
	// GetAccessToken retrieves an access token for authentication
	GetAccessToken(ctx context.Context) (string, error)
}

type RequestEditorFn func(ctx context.Context, req *http.Request) error

func RequestEditorInjectAccessToken(a RemoteAuth) RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		if req.Header.Get("Authorization") != "" {
			return nil
		}
		token, err := a.GetAccessToken(ctx)
		if err != nil {
			return err
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		return nil
	}
}
