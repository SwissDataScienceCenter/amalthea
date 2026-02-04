package shared

import (
	"context"
	"net/http"
)

// RemoteAuth can inject authentication credentials into an HTTP request
type RemoteAuth interface {
	// RequestEditor returns a request editor to be used by Remote clients
	RequestEditor() RequestEditorFn
}

// RequestEditorFn  is the function signature for the RequestEditor callback function
type RequestEditorFn func(ctx context.Context, req *http.Request) error
