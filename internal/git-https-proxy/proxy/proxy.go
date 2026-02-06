package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	configLib "github.com/SwissDataScienceCenter/amalthea/internal/git-https-proxy/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/git-https-proxy/tokenstore"
	"github.com/elazarl/goproxy"
)

// Returns a server handler that contains the proxy that injects the Git aithorization header when
// the conditions for doing so are met.
func GetProxyHandler(config configLib.GitProxyConfig) *goproxy.ProxyHttpServer {
	proxyHandler := goproxy.NewProxyHttpServer()
	proxyHandler.Verbose = false

	if config.AnonymousSession {
		return proxyHandler
	}

	tokenStore := tokenstore.New(&config)

	providers := make(map[string]configLib.GitProvider, len(config.Providers))
	for _, p := range config.Providers {
		providers[p.Id] = p
	}

	for _, repo := range config.Repositories {
		repoURL, err := url.Parse(repo.Url)
		if err != nil {
			log.Printf("Cannot parse repository URL (%s), skipping proxy setup.", repo.Url)
			continue
		}
		provider := repo.Provider
		if provider == "" {
			log.Printf("Repository (%s) has no provider, skipping proxy setup.", repo.Url)
			continue
		}
		if _, providerExists := providers[provider]; !providerExists {
			log.Printf("The provider (%s) for repository (%s) is not configured, skipping proxy setup.", provider, repo.Url)
			continue
		}
		log.Printf("Setting up proxy for repository: %s [%s]", repo.Url, provider)

		gitRepoHostWithWww := fmt.Sprintf("www.%s", repoURL.Hostname())

		handlerFunc := func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			validGitRequest := r.URL.Scheme == repoURL.Scheme &&
				hostsMatch(r.URL, repoURL) &&
				getPort(r.URL) == getPort(repoURL) &&
				strings.HasPrefix(strings.TrimLeft(r.URL.Path, "/"), strings.TrimLeft(repoURL.Path, "/"))
			if !validGitRequest {
				// Skip logging healthcheck requests
				if r.URL.Path != "/ping" && r.URL.Path != "/ping/" {
					log.Printf("The request %s does not match the git repository %s letting request through without adding auth headers\n", r.URL.String(), repoURL.String())
				}
				return r, nil
			}
			log.Printf("The request %s matches the git repository %s [%s], adding auth headers\n", r.URL.String(), repoURL.String(), provider)
			gitToken, err := tokenStore.GetGitAccessToken(provider, true)
			if err != nil {
				log.Printf("The git token cannot be refreshed, returning 401, error: %s\n", err.Error())
				return r, goproxy.NewResponse(r, goproxy.ContentTypeText, 401, "The git token could not be refreshed")
			}
			r.Header.Set("Authorization", fmt.Sprintf("Basic %s", gitToken))
			return r, nil
		}

		conditions := goproxy.ReqHostIs(
			repoURL.Hostname(),
			gitRepoHostWithWww,
			fmt.Sprintf("%s:443", repoURL.Hostname()),
			fmt.Sprintf("%s:443", gitRepoHostWithWww),
		)
		// NOTE: We need to eavesdrop on the HTTPS connection to insert the Auth header
		// we do this only for the case where the request host matches the host of the git repo
		// in all other cases we leave the request alone.
		proxyHandler.OnRequest(conditions).HandleConnect(goproxy.AlwaysMitm)
		proxyHandler.OnRequest(conditions).DoFunc(handlerFunc)
	}
	return proxyHandler
}

// Ensure that hosts name match with/without www. I.e.
// ensure www.hostname.com matches hostname.com and vice versa
func hostsMatch(url1 *url.URL, url2 *url.URL) bool {
	var err error
	var url1ContainsWww, url2ContainsWww bool
	wwwRegex := fmt.Sprintf("^%s", regexp.QuoteMeta("www."))
	url1ContainsWww, err = regexp.MatchString(wwwRegex, url1.Hostname())
	if err != nil {
		log.Fatalln(err)
	}
	url2ContainsWww, err = regexp.MatchString(wwwRegex, url2.Hostname())
	if err != nil {
		log.Fatalln(err)
	}
	if url1ContainsWww && !url2ContainsWww {
		return url1.Hostname() == fmt.Sprintf("www.%s", url2.Hostname())
	} else if !url1ContainsWww && url2ContainsWww {
		return fmt.Sprintf("www.%s", url1.Hostname()) == url2.Hostname()
	} else {
		return url1.Hostname() == url2.Hostname()
	}
}

// Infer port if not explicitly specified
func getPort(urlAddress *url.URL) string {
	if urlAddress.Port() == "" {
		if urlAddress.Scheme == "http" {
			return "80"
		} else if urlAddress.Scheme == "https" {
			return "443"
		}
	}
	return urlAddress.Port()
}
