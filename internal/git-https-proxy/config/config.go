package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type GitRepository struct {
	Url      string `json:"url"`
	Provider string `json:"provider"`
}

type GitProvider struct {
	Id             string `json:"id"`
	AccessTokenUrl string `json:"access_token_url"`
}

type GitProxyConfig struct {
	// The port where the proxy is listening on
	ProxyPort int `mapstructure:"port"`
	// The port (separate from the proxy) where the proxy will respond to status probes
	HealthPort int `mapstructure:"health_port"`
	// True if this is an anonymous session
	AnonymousSession bool `mapstructure:"anonymous_session"`
	// The oauth access token issued by Keycloak to a logged in Renku user
	RenkuAccessToken string `mapstructure:"renku_access_token"`
	// The oauth refresh token issued by Keycloak to a logged in Renku user
	// It is assumed that the refresh tokens do not expire after use and can be reused.
	// This means that the 'Revoke Refresh Token' setting in the Renku realm in Keycloak
	// is not enabled.
	RenkuRefreshToken string `mapstructure:"renku_refresh_token"`
	// The url of the renku deployment
	RenkuURL *url.URL `mapstructure:"renku_url"`
	// The name of the Renku realm in Keycloak
	RenkuRealm string `mapstructure:"renku_realm"`
	// The Keycloak client ID to which the access token and refresh tokens were issued to
	RenkuClientID string `mapstructure:"renku_client_id"`
	// The client secret for the client ID
	RenkuClientSecret string `mapstructure:"renku_client_secret"`
	// The git repositories to proxy
	Repositories []GitRepository `mapstructure:"repositories"`
	// The git providers
	Providers []GitProvider `mapstructure:"providers"`
	// The time interval used for refreshing renku tokens
	RefreshCheckPeriodSeconds int64 `mapstructure:"refresh_check_period_seconds"`
}

func GetConfig() (GitProxyConfig, error) {
	v := viper.New()
	v.SetConfigType("env")
	v.SetEnvPrefix("git_proxy")
	v.AutomaticEnv()

	v.SetDefault("port", 8080)
	v.SetDefault("health_port", 8081)
	v.SetDefault("anonymous_session", true)
	v.SetDefault("renku_access_token", "")
	v.SetDefault("renku_refresh_token", "")
	v.SetDefault("renku_url", nil)
	v.SetDefault("renku_realm", "")
	v.SetDefault("renku_client_id", "")
	v.SetDefault("renku_client_secret", "")
	v.SetDefault("repositories", []GitRepository{})
	v.SetDefault("providers", []GitProvider{})
	v.SetDefault("refresh_check_period_seconds", 600)

	var config GitProxyConfig
	dh := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		parseStringAsURL(),
		parseJsonArray(),
		parseJsonVariable(),
	))
	if err := v.Unmarshal(&config, dh); err != nil {
		return GitProxyConfig{}, err
	}

	return config, nil
}

func (c *GitProxyConfig) Validate() error {
	//? INFO: The proxy is a pass-through for anonymous sessions, so no config is required.
	if c.AnonymousSession {
		return nil
	}
	if c.RenkuAccessToken == "" {
		return fmt.Errorf("the renku access token is not defined")
	}
	if c.RenkuRefreshToken == "" {
		return fmt.Errorf("the renku refresh token is not defined")
	}
	if c.RenkuURL == nil {
		return fmt.Errorf("the renku URL is not defined")
	}
	if c.RenkuRealm == "" {
		return fmt.Errorf("the renku realm is not defined")
	}
	if c.RenkuClientID == "" {
		return fmt.Errorf("the renku client id is not defined")
	}
	if c.RenkuClientSecret == "" {
		return fmt.Errorf("the renku client secret is not defined")
	}
	if c.RefreshCheckPeriodSeconds <= 0 {
		return fmt.Errorf("the refresh token period is invalid")
	}
	return nil
}

func (c *GitProxyConfig) GetRefreshCheckPeriod() time.Duration {
	return time.Duration(c.RefreshCheckPeriodSeconds) * time.Second
}

func (c *GitProxyConfig) GetExpirationLeeway() time.Duration {
	return 4 * c.GetRefreshCheckPeriod()
}

func parseStringAsURL() mapstructure.DecodeHookFuncType {
	return func(f reflect.Type, t reflect.Type, data any) (interface{}, error) {
		// Check that the data is string
		if f.Kind() != reflect.String {
			return data, nil
		}

		// Check that the target type is our custom type
		if t != reflect.TypeOf(url.URL{}) {
			return data, nil
		}

		// Return the parsed value
		dataStr, ok := data.(string)
		if !ok {
			return nil, fmt.Errorf("cannot cast URL value to string")
		}
		if dataStr == "" {
			return nil, fmt.Errorf("empty values are not allowed for URLs")
		}
		url, err := url.Parse(dataStr)
		if err != nil {
			return nil, err
		}
		return url, nil
	}
}

func parseJsonArray() mapstructure.DecodeHookFuncType {
	return func(f reflect.Type, t reflect.Type, data any) (interface{}, error) {
		// Check that the data is a string
		if f.Kind() != reflect.String {
			return data, nil
		}

		// Check that the target type is a slice
		if t.Kind() != reflect.Slice {
			return data, nil
		}

		raw := data.(string)
		if raw == "" {
			return nil, fmt.Errorf("cannot parse empty string as a slice")
		}

		var slice []json.RawMessage
		if err := json.Unmarshal([]byte(raw), &slice); err != nil {
			return data, nil
		}

		var value []string
		for _, v := range slice {
			value = append(value, string(v))
		}

		return value, nil
	}
}

func parseJsonVariable() mapstructure.DecodeHookFuncType {
	return func(f reflect.Type, t reflect.Type, data any) (interface{}, error) {
		// Check that the data is a string
		if f.Kind() != reflect.String {
			return data, nil
		}

		// Check that the target type is a struct
		if t.Kind() != reflect.Struct {
			return data, nil
		}

		raw := data.(string)
		if raw == "" {
			return nil, fmt.Errorf("cannot parse empty string as a struct")
		}

		value := reflect.New(t)
		if err := json.Unmarshal([]byte(raw), value.Interface()); err != nil {
			return data, nil
		}

		return value.Interface(), nil
	}
}
