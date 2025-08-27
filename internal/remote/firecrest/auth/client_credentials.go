package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/SwissDataScienceCenter/amalthea/internal/remote/firecrest"
)

// FirecrestClientCredentialsAuth implements the "Client Credentials Grant"
// authentication flow for the FirecREST API.
type FirecrestClientCredentialsAuth struct {
	clientID     string
	clientSecret string
	tokenURI     string

	accessToken string

	// accessTokenLock ensures that we do not try to refresh
	// the accessToken twice at the same time.
	accessTokenLock *sync.RWMutex

	httpClient *http.Client

	// gitAccessTokensLock *sync.RWMutex
}

func NewFirecrestClientCredentialsAuth() (auth *FirecrestClientCredentialsAuth, err error) {
	auth = &FirecrestClientCredentialsAuth{
		httpClient: http.DefaultClient,
	}

	return

}

// RequestEditor returns a request editor which injects a valid access token
// for FirecREST API requests.
func (a *FirecrestClientCredentialsAuth) RequestEditor() firecrest.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		// TODO
		return nil
	}
}

func (a *FirecrestClientCredentialsAuth) GetAccessToken() {

}

func (a *FirecrestClientCredentialsAuth) refreshAccessToken() (token string, err error) {
	a.accessTokenLock.Lock()
	defer a.accessTokenLock.Unlock()

	ctx, cancel := context.WithTimeoutCause(context.Background(), 30*time.Second, fmt.Errorf("authentication request timed out"))
	defer cancel()

	postData := url.Values{}
	postData.Set("grant_type", "client_credentials")
	postData.Set("client_id", a.clientID)
	postData.Set("client_secret", a.clientSecret)
	body := strings.NewReader(postData.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURI, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, err = a.httpClient.Do(req)

	// TODO
	return "", nil
}

// Response keys
// "access_token" string
// "expires_in" int
// "refresh_expires_in" // will not use
// "token_type" // will not use
// "not-before-policy" // will not use
// "scope" // will not use
