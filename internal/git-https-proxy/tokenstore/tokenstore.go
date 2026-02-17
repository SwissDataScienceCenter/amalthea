package tokenstore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/SwissDataScienceCenter/amalthea/internal/git-https-proxy/config"
	"github.com/golang-jwt/jwt/v5"
)

type TokenSet struct {
	AccessToken string
	ExpiresAt   int64
}

type TokenStore struct {
	// The git proxy config
	Config *config.GitProxyConfig
	// The git providers
	Providers map[string]config.GitProvider
	// Period used to refresh renku tokens
	RefreshTickerPeriod time.Duration
	// Safety margin for when to consider a token expired. For example if this is set to
	// 30 seconds then the token is considered expired if it expires in the next 30 seconds.
	ExpirationLeeway time.Duration

	// The current renku access token
	renkuAccessToken string
	// The current renku refresh token
	renkuRefreshToken string
	// Ensures that the renku token is not refereshed
	// twice at the same time. It also ensures that all other threads that need to simply
	// read the token will wait until the refresh (write) is complete.
	renkuAccessTokenLock *sync.RWMutex
	// Channel that is populated by the timer that triggers the automated renku access token refresh
	refreshTicker *time.Ticker
	// The current git access tokens for each provider
	gitAccessTokens map[string]TokenSet
	// Ensures that the git access token are not refreshed twice at the same time.
	// Note: We use one lock for all tokens for simplicity.
	gitAccessTokensLock *sync.RWMutex
}

func New(c *config.GitProxyConfig) *TokenStore {
	providers := make(map[string]config.GitProvider, len(c.Providers))
	for _, p := range c.Providers {
		providers[p.Id] = p
	}

	store := TokenStore{
		Config:               c,
		Providers:            providers,
		RefreshTickerPeriod:  c.GetRefreshCheckPeriod(),
		ExpirationLeeway:     c.GetExpirationLeeway(),
		renkuAccessToken:     c.RenkuAccessToken,
		renkuRefreshToken:    c.RenkuRefreshToken,
		renkuAccessTokenLock: &sync.RWMutex{},
		refreshTicker:        time.NewTicker(c.GetRefreshCheckPeriod()),
		gitAccessTokens:      make(map[string]TokenSet, len(c.Providers)),
		gitAccessTokensLock:  &sync.RWMutex{},
	}
	// Start a go routine to keep the refresh token valid
	go store.periodicTokenRefresh()
	return &store
}

// Returns a valid access token for the corresponding git provider.
// If the token is expired, a new one will be retrieved using the renku access token.
func (s *TokenStore) GetGitAccessToken(provider string, encode bool) (string, error) {
	s.gitAccessTokensLock.RLock()
	tokenSet, accessTokenExists := s.gitAccessTokens[provider]
	accessTokenExpiresAt := tokenSet.ExpiresAt
	s.gitAccessTokensLock.RUnlock()

	if !accessTokenExists || (0 < accessTokenExpiresAt && accessTokenExpiresAt < time.Now().Add(s.ExpirationLeeway).Unix()) {
		log.Printf("Getting a fresh token for git provider: %s", provider)
		if err := s.refreshGitAccessToken(provider); err != nil {
			return "", err
		}
	}

	s.gitAccessTokensLock.RLock()
	defer s.gitAccessTokensLock.RUnlock()
	if encode {
		return encodeGitCredentials(s.gitAccessTokens[provider].AccessToken), nil
	}
	return s.gitAccessTokens[provider].AccessToken, nil
}

type gitTokenRefreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

// Exchange the renku access token for the access token of the corresponding provider
func (s *TokenStore) refreshGitAccessToken(provider string) error {
	s.gitAccessTokensLock.Lock()
	defer s.gitAccessTokensLock.Unlock()

	providerURL := s.Providers[provider].AccessTokenUrl

	req, err := http.NewRequest(http.MethodGet, providerURL, nil)
	if err != nil {
		return err
	}
	renkuAccessToken, err := s.getValidRenkuAccessToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", renkuAccessToken))
	// NOTE: Without the function below the authorization header is taken out before the request
	// even hits the gateway proxy and therefore the token never reaches the gateway-auth module
	// that swaps this authorization token for a gitlab token.
	preserveAuthzHeader := func(req *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return nil
		}
		authz := via[0].Header.Get("Authorization")
		if authz == "" {
			return nil
		}
		req.Header.Set("Authorization", authz)
		return nil
	}
	c := http.Client{Timeout: time.Second * 30, CheckRedirect: preserveAuthzHeader}
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("cannot exchange renku token for git token, failed with staus code: %d", res.StatusCode)
	}
	var resParsed gitTokenRefreshResponse
	err = json.NewDecoder(res.Body).Decode(&resParsed)
	if err != nil {
		return err
	}
	s.gitAccessTokens[provider] = TokenSet(resParsed)
	return nil
}

// Returns a valid renku access token. If the token is expired, the token will be refreshed first.
func (s *TokenStore) getValidRenkuAccessToken() (string, error) {
	isExpired, err := s.isJWTExpired(s.getRenkuAccessToken())
	if err != nil {
		return "", err
	}
	if isExpired {
		if err = s.refreshRenkuAccessToken(); err != nil {
			return "", err
		}
	}
	return s.getRenkuAccessToken(), nil
}

func (s *TokenStore) getRenkuAccessToken() string {
	s.renkuAccessTokenLock.RLock()
	defer s.renkuAccessTokenLock.RUnlock()
	return s.renkuAccessToken
}

// VerifyExpiresAt implements the same logic as can be found in jwt v4 but
// in the style of v5.
//
// Main difference is that the exp nil check is hard coded to false.
//
// v4 implementation boils down to comparing the value passed to the expiration time.
// v5 changed that: "now" is compared to the expiration time with leeway added.
func VerifyExpiresAt(claims jwt.RegisteredClaims, leeway time.Duration) (bool, error) {
	exp, err := claims.GetExpirationTime()
	if err != nil {
		return true, err
	}

	// Here we have it setup so that if the exp claim is not defined we assume the token is not expired.
	// Keycloak does not set the `exp` claim on tokens that have the offline access grant - because they do not expire.
	if exp == nil {
		return false, nil
	}

	cmp := time.Now().Add(+leeway)
	return cmp.Before(exp.Time), nil
}

// Checks if the expiry of the token has passed or is coming up soon based on a predefined threshold.
// NOTE: no signature validation is performed at all. All of the tokens in the proxy are trusted implicitly
// because they come from trusted/controlled sources.
func (s *TokenStore) isJWTExpired(token string) (bool, error) {
	parser := jwt.NewParser()
	claims := jwt.RegisteredClaims{}
	if _, _, err := parser.ParseUnverified(token, &claims); err != nil {
		log.Printf("Cannot parse token claims, assuming token is expired: %s\n", err.Error())
		return true, err
	}

	jwtIsNotExpired, err := VerifyExpiresAt(claims, s.ExpirationLeeway)
	if err != nil {
		return true, err
	}

	return !jwtIsNotExpired, err
}

type renkuTokenRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Refreshes the renku access token.
func (s *TokenStore) refreshRenkuAccessToken() error {
	s.renkuAccessTokenLock.Lock()
	defer s.renkuAccessTokenLock.Unlock()
	payload := url.Values{}
	payload.Add("grant_type", "refresh_token")
	payload.Add("refresh_token", s.renkuRefreshToken)
	body := strings.NewReader(payload.Encode())
	req, err := http.NewRequest(http.MethodPost, s.Config.RenkuURL.JoinPath(fmt.Sprintf("auth/realms/%s/protocol/openid-connect/token", s.Config.RenkuRealm)).String(), body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.Config.RenkuClientID, s.Config.RenkuClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
	s.renkuAccessToken = resParsed.AccessToken
	if resParsed.RefreshToken != "" {
		s.renkuRefreshToken = resParsed.RefreshToken
	}
	return nil
}

// Periodically refreshes the renku access token. Used to make sure the refresh token does not expire.
func (s *TokenStore) periodicTokenRefresh() {
	for {
		<-s.refreshTicker.C
		s.renkuAccessTokenLock.RLock()
		renkuRefreshToken := s.renkuRefreshToken
		s.renkuAccessTokenLock.RUnlock()
		refreshTokenIsExpired, err := s.isJWTExpired(renkuRefreshToken)
		if err != nil {
			log.Printf("Could not check if renku refresh token is expired: %s\n", err.Error())
		}
		if refreshTokenIsExpired {
			log.Println("Getting a new renku refresh token from automatic checks")
			err = s.refreshRenkuAccessToken()
			if err != nil {
				log.Printf("Could not refresh renku token: %s\n", err.Error())
			}
		}
	}
}

func encodeGitCredentials(token string) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("oauth2:%s", token)))
}
