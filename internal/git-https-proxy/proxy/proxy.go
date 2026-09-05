package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	configLib "github.com/SwissDataScienceCenter/amalthea/internal/git-https-proxy/config"
	"github.com/SwissDataScienceCenter/amalthea/internal/git-https-proxy/tokenstore"
	"github.com/elazarl/goproxy"
)

// Match a Git pkt-line ref update:
// {old_id} {new_id} refs/{namespace}/{name}[\0{capabilities}]\n
// Doesn't check for old/new id lengths (SHA-1, SHA-256), other than it must be hex.
// Ref: https://git-scm.com/docs/http-protocol#_smart_service_git_receive_pack
var gitRefUpdateRE = regexp.MustCompile(
	`^([0-9a-f]+) +([0-9a-f]+) +(refs/[^/[:space:]\x00]+/[^[:space:]\x00]+)(?:\x00.*)?\n$`,
)

// Authorize a Git HTTP request against the configured reference allowlist.
//
// GET requests and POST git-upload-pack requests are allowed. POST
// git-receive-pack requests are allowed only when every ref update in the
// request targets an allowed reference. Other requests are rejected.
//
// The caller must ensure that the token's scope does not permit unrelated
// write operations.
//
// Returns:
// http.StatusOK = allowed
// http.StatusBadRequest = malformed Git request
// http.StatusForbidden = forbidden
func authorizeGitRequest(r *http.Request, allowedRefs map[string]struct{}) int {

	const maxCommandLength = 1024 * 1024

	if r.Method == http.MethodGet {
		return http.StatusOK
	}

	if r.Method != http.MethodPost {
		return http.StatusForbidden
	}

	if strings.HasSuffix(r.URL.Path, "/git-upload-pack") {
		return http.StatusOK
	}

	if !strings.HasSuffix(r.URL.Path, "/git-receive-pack") {
		return http.StatusForbidden
	}

	if len(allowedRefs) == 0 {
		return http.StatusForbidden
	}

	// Request bodies are consumed as they are read.
	// Buffer the bytes consumed while parsing the pkt-line command section so
	// the body can be restored after parsing is complete.
	var consumed bytes.Buffer

	readPktLine := func() ([]byte, error) {
		header := make([]byte, 4)

		if _, err := io.ReadFull(r.Body, header); err != nil {
			return nil, err
		}
		consumed.Write(header)

		length, err := strconv.ParseUint(string(header), 16, 16)
		if err != nil {
			return nil, err
		}

		if length == 0x0000 {
			// Flush packet: end of the ref-update command section.
			return nil, nil
		}

		if length < 4 {
			return nil, fmt.Errorf("invalid git pkt-line length: %04x", length)
		}

		if uint64(consumed.Len())+length > maxCommandLength {
			return nil, fmt.Errorf("command section exceeds maximum size")
		}

		payload := make([]byte, int(length)-4)
		if _, err := io.ReadFull(r.Body, payload); err != nil {
			return nil, err
		}
		consumed.Write(payload)

		return payload, nil
	}

	// Check that all ref updates are in the allow list.
	for {
		// Read next command
		command, err := readPktLine()
		if err != nil {
			return http.StatusBadRequest
		}

		// End of command section
		if command == nil {
			break
		}

		// Parse ref update
		matches := gitRefUpdateRE.FindSubmatch(command)
		if matches == nil {
			return http.StatusBadRequest
		}
		newOID := matches[2]
		refs := matches[3]

		// Do not allow all 0s newOID which is for branch deletion, whether it is in allowed list or not
		if len(bytes.Trim(newOID, "0")) == 0 {
			return http.StatusForbidden
		}

		// Ref must be in allowed list
		if _, ok := allowedRefs[string(refs)]; !ok {
			return http.StatusForbidden
		}
	}

	// Restore the request body so the Git handler sees the original stream.
	// The bytes consumed while parsing the command section are replayed first,
	// followed by the unread remainder of the original body.
	//
	// Note: the command section is limited to maxCommandLength to bound memory usage.
	// This limits the number of ref updates that can be inspected (it does not
	// limit the size of the Git packfile that follows it.)
	r.Body = io.NopCloser(
		io.MultiReader(
			bytes.NewReader(consumed.Bytes()),
			r.Body,
		),
	)

	return http.StatusOK
}

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

		var allowedReferences map[string]struct{}

		if repo.References != nil {
			allowedReferences = make(map[string]struct{}, len(*repo.References))

			for _, ref := range *repo.References {
				allowedReferences[ref] = struct{}{}
			}
		}

		log.Printf("Setting up proxy for repository: %s [%s]", repo.Url, provider)

		gitRepoHostWithWww := fmt.Sprintf("www.%s", repoURL.Hostname())

		handlerFunc := func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			// TODO: the existing repository path matching uses a raw HasPrefix, which may match unauthorized paths
			// (e.g. repoURL.Path /repo/name matches r.URL.Path /repo/name-super-private)
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
			if allowedReferences != nil {
				status := authorizeGitRequest(r, allowedReferences)
				switch status {
				case http.StatusOK:
					// authorized
				case http.StatusForbidden:
					return r, goproxy.NewResponse(r, goproxy.ContentTypeText, http.StatusForbidden, "Forbidden")
				case http.StatusBadRequest:
					return r, goproxy.NewResponse(r, goproxy.ContentTypeText, http.StatusBadRequest, "Bad request")
				default:
					return r, goproxy.NewResponse(r, goproxy.ContentTypeText, http.StatusInternalServerError, "An internal error occured")
				}
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
func hostsMatch(url1, url2 *url.URL) bool {
	host1 := strings.TrimPrefix(strings.ToLower(url1.Hostname()), "www.")
	host2 := strings.TrimPrefix(strings.ToLower(url2.Hostname()), "www.")
	return host1 == host2
}

// Infer port if not explicitly specified
func getPort(urlAddress *url.URL) string {
	port := urlAddress.Port()
	if port == "" {
		switch urlAddress.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	return port
}
