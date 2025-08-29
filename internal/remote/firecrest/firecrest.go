//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -generate types,client,spec -package firecrest -o firecrest_gen.go openapi_spec_test.yaml

package firecrest

import (
	"net/http"
	"net/url"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/firecrest/auth"
)

type FirecrestClient struct {
	ClientWithResponses
	auth                auth.FirecrestAuth
	httpClient          *http.Client
	extraRequestEditors []RequestEditorFn
}

func NewFirecrestClient(apiURL *url.URL, options ...FirecrestClientOption) (fc *FirecrestClient, err error) {
	fc = &FirecrestClient{}
	for _, opt := range options {
		if err := opt(fc); err != nil {
			return nil, err
		}
	}
	// Create httpClient, if not already present
	if fc.httpClient == nil {
		fc.httpClient = http.DefaultClient
	}
	// Create client
	clientOpts := []ClientOption{WithHTTPClient(fc.httpClient)}
	for _, fn := range fc.extraRequestEditors {
		clientOpts = append(clientOpts, WithRequestEditorFn(fn))
	}
	if fc.auth != nil {
		clientOpts = append(clientOpts, WithRequestEditorFn(RequestEditorFn(fc.auth.RequestEditor())))
	}
	client, err := NewClientWithResponses(apiURL.String(), clientOpts...)
	if err != nil {
		return nil, err
	}
	fc.ClientInterface = client
	return fc, nil
}

type FirecrestClientOption func(*FirecrestClient) error

func WithAuth(auth auth.FirecrestAuth) FirecrestClientOption {
	return func(fc *FirecrestClient) error {
		fc.auth = auth
		return nil
	}
}

func WithHttpClient(httpClient *http.Client) FirecrestClientOption {
	return func(fc *FirecrestClient) error {
		fc.httpClient = httpClient
		return nil
	}
}

func WithExtraRequestEditors(editors ...RequestEditorFn) FirecrestClientOption {
	return func(fc *FirecrestClient) error {
		fc.extraRequestEditors = append(fc.extraRequestEditors, editors...)
		return nil
	}
}
