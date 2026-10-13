package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	sharedAuth "github.com/SwissDataScienceCenter/amalthea/internal/remote/auth/shared"
	"github.com/SwissDataScienceCenter/amalthea/internal/utils"
	"github.com/golang-jwt/jwt/v5"
)

// RenkuAuth implements authentication as used in Renku
type RenkuAuth struct {
	// renkuAccessToken the current renku access token
	accessToken string
	// renkuAccessTokenExpiresAt is when the renku access token expires
	accessTokenExpiresAt time.Time
	// renkuRefreshToken the current renku refresh token
	refreshToken string
	// renkuTokenURI the URI used for obtaining new renku tokens
	tokenURI string
	// renkuAccessTokenLock ensures that we do not try to refresh
	// the renkuAccessToken twice at the same time.
	accessTokenLock *sync.RWMutex
	// refreshTicker to automate renku access token refresh
	refreshTicker *time.Ticker

	// httpClient is the HTTP client used to obtain access tokens
	httpClient *http.Client
}

// Check that RenkuAuth satisfies the FirecrestAuth interface
var _ RunnersAuth = (*RenkuAuth)(nil)

func newRenkuAuth(accessToken, refreshToken, tokenURI string, options ...RenkuAuthOption) (auth *RenkuAuth, err error) {
	auth = &RenkuAuth{
		accessToken:          accessToken,
		accessTokenExpiresAt: time.Time{},
		refreshToken:         refreshToken,
		tokenURI:             tokenURI,
		accessTokenLock:      &sync.RWMutex{},
	}
	for _, opt := range options {
		if err := opt(auth); err != nil {
			return nil, err
		}
	}
	// Validate auth
	if refreshToken == "" {
		return nil, fmt.Errorf("refreshToken is not set")
	}
	if tokenURI == "" {
		return nil, fmt.Errorf("tokenURI is not set")
	}
	parser := jwt.NewParser()
	claims := jwt.RegisteredClaims{}
	_, _, err = parser.ParseUnverified(auth.accessToken, &claims)
	if err == nil {
		auth.accessTokenExpiresAt = claims.ExpiresAt.Time
	}
	// Create httpClient, if not already present
	if auth.httpClient == nil {
		auth.httpClient = http.DefaultClient
	}

	auth.refreshTicker = time.NewTicker(time.Duration(60) * time.Second)
	// Start a go routine to keep the refresh token valid
	go auth.periodicTokenRefresh()

	return auth, nil
}

// RenkuAuthOption allows setting options
type RenkuAuthOption func(*RenkuAuth) error

// RequestEditor returns a request editor which injects a valid access token
// for Renku API requests.
func (a *RenkuAuth) RequestEditor() sharedAuth.RequestEditorFn {
	return sharedAuth.RequestEditorInjectAccessToken(a)
}

func (a *RenkuAuth) GetAccessToken(ctx context.Context) (token string, err error) {
	a.accessTokenLock.RLock()
	token = a.accessToken
	expiresAt := a.accessTokenExpiresAt
	a.accessTokenLock.RUnlock()

	// Return the current token if it is still valid
	if token != "" && (utils.IsNotExpired(expiresAt, 10*time.Second, false)) {
		return token, nil
	}

	// Refresh the token
	if err := a.refreshAccessToken(ctx); err != nil {
		return token, err
	}
	a.accessTokenLock.RLock()
	defer a.accessTokenLock.RUnlock()
	return a.accessToken, nil
}

func (a *RenkuAuth) refreshAccessToken(ctx context.Context) error {
	a.accessTokenLock.Lock()
	defer a.accessTokenLock.Unlock()

	renkuTokenURL, err := url.Parse(a.tokenURI)
	if err != nil {
		return err
	}
	payload := url.Values{}
	payload.Set("grant_type", "refresh_token")
	payload.Set("refresh_token", a.refreshToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, renkuTokenURL.String(), strings.NewReader(payload.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		err = fmt.Errorf("cannot refresh renku access token, failed with status code: %d", res.StatusCode)
		return err
	}
	var resParsed renkuTokenRefreshResponse
	err = json.NewDecoder(res.Body).Decode(&resParsed)
	if err != nil {
		return err
	}
	a.accessToken = resParsed.AccessToken
	a.refreshToken = resParsed.RefreshToken

	parser := jwt.NewParser()
	claims := jwt.RegisteredClaims{}
	if _, _, err := parser.ParseUnverified(a.accessToken, &claims); err != nil {
		log.Printf("cannot parse token claims: %s\n", err.Error())
		return err
	}
	a.accessTokenExpiresAt = claims.ExpiresAt.Time

	log.Printf("refreshed renku access tokens, renkuAccessTokenExpiresAt = %s\n", a.accessTokenExpiresAt.String())

	return nil
}

type renkuTokenRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Periodically refreshes the renku access token. Used to make sure the refresh token does not expire.
func (a *RenkuAuth) periodicTokenRefresh() {
	for {
		<-a.refreshTicker.C
		a.accessTokenLock.RLock()
		refreshToken := a.refreshToken
		a.accessTokenLock.RUnlock()
		refreshTokenIsValid, err := utils.VerifyJWTExpiresAt(refreshToken, 4*time.Minute, false)
		if err != nil {
			log.Printf("Could not check if renku refresh token is expired: %s\n", err.Error())
		}
		if !refreshTokenIsValid {
			log.Println("Getting a new renku refresh token from automatic checks")
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(60)*time.Second)
			err = a.refreshAccessToken(ctx)
			cancel()
			if err != nil {
				log.Printf("Could not refresh renku token: %s\n", err.Error())
			}
		}
	}
}
