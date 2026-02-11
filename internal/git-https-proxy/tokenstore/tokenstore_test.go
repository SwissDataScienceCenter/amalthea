package tokenstore

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	configLib "github.com/SwissDataScienceCenter/amalthea/internal/git-https-proxy/config"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

func getTestConfig(renkuURL string, renkuAccessToken string, renkuRefreshToken string) configLib.GitProxyConfig {
	parsedRenkuURL, err := url.Parse(renkuURL)
	if err != nil {
		log.Fatalln(err)
	}

	providers := []configLib.GitProvider{
		{
			Id:             "example",
			AccessTokenUrl: parsedRenkuURL.JoinPath("/api/oauth2/token").String(),
		},
	}

	return configLib.GitProxyConfig{
		ProxyPort:                 8080,
		HealthPort:                8081,
		AnonymousSession:          false,
		RenkuAccessToken:          renkuAccessToken,
		RenkuRefreshToken:         renkuRefreshToken,
		RenkuURL:                  parsedRenkuURL,
		RenkuRealm:                "Renku",
		RenkuClientID:             "RenkuClientID",
		RenkuClientSecret:         "RenkuClientSecret",
		Repositories:              []configLib.GitRepository{},
		Providers:                 providers,
		RefreshCheckPeriodSeconds: 600,
	}
}

func getTestTokenStore(renkuURL string, renkuAccessToken string, renkuRefreshToken string) *TokenStore {
	config := getTestConfig(renkuURL, renkuAccessToken, renkuRefreshToken)
	return New(&config)
}

func setUpTestServer(handler http.Handler) (*url.URL, func()) {
	ts := httptest.NewServer(handler)
	tsURL, err := url.Parse(ts.URL)
	if err != nil {
		log.Fatalln(err)
	}
	return tsURL, ts.Close
}

func setUpDummyRefreshEndpoints(gitRefreshResponse *gitTokenRefreshResponse, renkuRefreshResponse *renkuTokenRefreshResponse) (*url.URL, func()) {
	handler := http.NewServeMux()
	gitHandlerFunc := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Handling git token refresh request at %s", r.URL.String())
		if gitRefreshResponse == nil {
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("Cannot refresh git token"))
			if err != nil {
				log.Fatalln(err)
			}
		}
		err := json.NewEncoder(w).Encode(gitRefreshResponse)
		if err != nil {
			log.Fatalln(err)
		}
	}
	renkuHandlerFunc := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Handling renku token refresh request at %s", r.URL.String())
		if renkuRefreshResponse == nil {
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("Cannot refresh renku token"))
			if err != nil {
				log.Fatalln(err)
			}
		}
		err := json.NewEncoder(w).Encode(renkuRefreshResponse)
		if err != nil {
			log.Fatalln(err)
		}
	}
	handler.HandleFunc("/api/oauth2/token", gitHandlerFunc)
	handler.HandleFunc("/auth/realms/Renku/protocol/openid-connect/token", renkuHandlerFunc)
	return setUpTestServer(handler)
}

type DummySigningMethod struct{}

func (d DummySigningMethod) Verify(signingString, signature string, key interface{}) error {
	return nil
}

func (d DummySigningMethod) Sign(signingString string, key interface{}) (string, error) {
	return base64.URLEncoding.EncodeToString([]byte(signingString)), nil
}

func (d DummySigningMethod) Alg() string { return "none" }

func getDummyAccessToken(expiresAt int64) (token string, err error) {
	t := jwt.New(DummySigningMethod{})
	t.Claims = &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Unix(expiresAt, 0)),
	}
	return t.SignedString(nil)
}

func TestSuccessfulRefresh(t *testing.T) {
	newGitToken := "newGitToken"
	newRenkuToken, err := getDummyAccessToken(time.Now().Add(time.Hour).Unix())
	assert.Nil(t, err)
	oldRenkuAccessToken, err := getDummyAccessToken(time.Now().Add(-time.Hour).Unix())
	assert.Nil(t, err)
	oldRenkuRefreshToken, err := getDummyAccessToken(time.Now().Add(2 * time.Hour).Unix())
	assert.Nil(t, err)
	gitRefreshResponse := &gitTokenRefreshResponse{
		AccessToken: newGitToken,
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}
	renkuRefreshResponse := &renkuTokenRefreshResponse{
		AccessToken:  newRenkuToken,
		RefreshToken: oldRenkuRefreshToken,
	}
	authServerURL, authServerClose := setUpDummyRefreshEndpoints(gitRefreshResponse, renkuRefreshResponse)
	log.Printf("Dummy refresh server running at %s\n", authServerURL.String())
	defer authServerClose()

	// token refresh is needed and succeeds
	store := getTestTokenStore(authServerURL.String(), oldRenkuAccessToken, oldRenkuRefreshToken)
	gitToken, err := store.GetGitAccessToken("example", false)
	assert.Nil(t, err)
	assert.Equal(t, gitToken, newGitToken)
	renkuAccessToken, err := store.getValidRenkuAccessToken()
	assert.Nil(t, err)
	assert.Equal(t, renkuAccessToken, newRenkuToken)

	// change token in server response
	// assert that immediately after the refresh the token is valid and is not refreshed again
	gitRefreshResponse.AccessToken = "SomethingElse"
	evenNewerRenkuToken, err := getDummyAccessToken(time.Now().Add(2 * time.Hour).Unix())
	assert.Nil(t, err)
	renkuRefreshResponse.AccessToken = evenNewerRenkuToken
	gitToken, err = store.GetGitAccessToken("example", false)
	assert.Nil(t, err)
	assert.Equal(t, gitToken, newGitToken)
	renkuAccessToken, err = store.getValidRenkuAccessToken()
	assert.Nil(t, err)
	assert.Equal(t, renkuAccessToken, newRenkuToken)
}

func TestNoRefreshNeeded(t *testing.T) {
	newGitToken := "newGitToken"
	oldRenkuAccessToken, err := getDummyAccessToken(time.Now().Add(time.Hour).Unix())
	assert.Nil(t, err)
	oldRenkuRefreshToken, err := getDummyAccessToken(time.Now().Add(2 * time.Hour).Unix())
	assert.Nil(t, err)
	gitRefreshResponse := &gitTokenRefreshResponse{
		AccessToken: newGitToken,
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}
	// Passing nil means that if the any tokens are attempted to be refreshed errors will be returned
	authServerURL, authServerClose := setUpDummyRefreshEndpoints(gitRefreshResponse, nil)
	defer authServerClose()

	store := getTestTokenStore(authServerURL.String(), oldRenkuAccessToken, oldRenkuRefreshToken)
	gitToken, err := store.GetGitAccessToken("example", false)
	assert.Nil(t, err)
	assert.Equal(t, newGitToken, gitToken)
	renkuAccessToken, err := store.getValidRenkuAccessToken()
	assert.Nil(t, err)
	assert.Equal(t, renkuAccessToken, oldRenkuAccessToken)
}

func TestAutomatedRefreshTokenRenewal(t *testing.T) {
	newRenkuAccessToken, err := getDummyAccessToken(time.Now().Add(time.Hour).Unix())
	assert.Nil(t, err)
	newRenkuRefreshToken, err := getDummyAccessToken(time.Now().Add(24 * time.Hour).Unix())
	assert.Nil(t, err)
	oldRenkuAccessToken, err := getDummyAccessToken(time.Now().Add(-time.Hour).Unix())
	assert.Nil(t, err)
	oldRenkuRefreshToken, err := getDummyAccessToken(time.Now().Add(10 * time.Second).Unix())
	assert.Nil(t, err)
	renkuRefreshResponse := &renkuTokenRefreshResponse{
		AccessToken:  newRenkuAccessToken,
		RefreshToken: newRenkuRefreshToken,
	}
	authServerURL, authServerClose := setUpDummyRefreshEndpoints(nil, renkuRefreshResponse)
	log.Printf("Dummy refresh server running at %s\n", authServerURL.String())
	defer authServerClose()

	config := getTestConfig(authServerURL.String(), oldRenkuAccessToken, oldRenkuRefreshToken)
	config.RefreshCheckPeriodSeconds = 2
	store := New(&config)
	assert.Equal(t, store.getRenkuAccessToken(), oldRenkuAccessToken)
	assert.Equal(t, store.renkuRefreshToken, oldRenkuRefreshToken)
	// Sleep to allow for automated token refresh to occur
	time.Sleep(5 * time.Second)
	assert.Equal(t, store.getRenkuAccessToken(), newRenkuAccessToken)
	assert.Equal(t, store.renkuRefreshToken, newRenkuRefreshToken)
}
