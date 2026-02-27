package cloner

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/sys/unix"
)

type Cloner struct {
	config     CloneConfig
	user       User
	remoteName string
}

func (c *Cloner) Run() error {
	for _, repository := range c.config.Repositories {
		log.Println("Processing", repository.URL)
		err := c.execute(repository)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cloner) proxyURL() string {
	return fmt.Sprintf("http://localhost:%v", c.config.GitProxyPort)
}

func (c *Cloner) execute(repository Repository) error {
	log.Println("Checking if the repo already exists.")
	if repository.Exists() {
		err := c.setupProxy(repository)
		if err != nil {
			return err
		}
	}

	gitUser := "oauth2"
	gitAccessToken, err := c.getAccessToken(repository.Provider)
	if err != nil {
		return err
	}
	err = c.initializeRepository(repository)
	if err != nil {
		return err
	}

	if !c.user.IsAnonymous() {
		repository.Cli.SetCredentials(gitUser, gitAccessToken)
	}

	err = c.clone(repository)
	if err != nil {
		return err
	}

	return c.setupProxy(repository)
}

func (c *Cloner) setupProxy(repository Repository) error {
	if !c.config.IsGitProxyEnabled {
		log.Println("Skipping git proxy setup")
		return nil
	}

	log.Println("Setting up git proxy to", c.proxyURL())
	_, err := repository.Cli.Config([]string{"http.proxy", c.proxyURL()})
	if err != nil {
		return err
	}
	_, err = repository.Cli.Config([]string{"http.sslVerify", "false"})
	return err
}

func (c *Cloner) getAccessToken(providerId string) (string, error) {

	var provider *GitProvider
	for _, gitProvider := range c.config.GitProviders {
		if gitProvider.Id == providerId {
			provider = &gitProvider
			break
		}
	}

	if provider == nil {
		return "", fmt.Errorf("failed to find provider %v", providerId)
	}

	log.Printf("Get token for: %v\n", provider.Id)
	req, err := http.NewRequest(http.MethodGet, provider.AccessTokenUrl, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.user.RenkuToken))
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
	cl := http.Client{Timeout: time.Second * 30, CheckRedirect: preserveAuthzHeader}
	res, err := cl.Do(req)
	if err != nil {
		return "", err
	}
	if res.StatusCode != 200 {
		return "", fmt.Errorf("cannot exchange renku token for git token, failed with status code: %d", res.StatusCode)
	}
	var parsed RefreshTokenAnswer
	err = json.NewDecoder(res.Body).Decode(&parsed)
	if err != nil {
		return "", err
	}

	return parsed.AccessToken, nil
}

func (c *Cloner) initializeRepository(repository Repository) error {
	log.Println("Initializing repo")

	_, err := repository.Cli.Init([]string{})
	if err != nil {
		return err
	}

	// NOTE: For anonymous sessions email and name are not known for the user
	if c.user.Email != "" {
		log.Printf("Setting email %v in git config\n", c.user.Email)
		_, err = repository.Cli.Config([]string{"user.email", c.user.Email})
		if err != nil {
			return err
		}
	}
	if c.user.FullName != "" {
		log.Printf("Setting name %v in git config", c.user.FullName)
		_, err := repository.Cli.Config([]string{"user.name", c.user.FullName})
		if err != nil {
			return err
		}
	}

	_, err = repository.Cli.Config([]string{"push.default", "simple"})
	return err
}

func (c *Cloner) clone(repository Repository) error {
	log.Printf("Cloning repository %v from %v\n", repository.clonePath, repository.URL)
	args := []string{"install"}
	if !c.config.LfsAutoFetch {
		args = append(args, "--skip-smudge")
	}
	args = append(args, "--local")
	_, err := repository.Cli.Lfs(args)
	if err != nil {
		return err
	}

	// The only possible error is that the remote already exists
	_, err = repository.Cli.Remote([]string{"add", c.remoteName, repository.URL})
	if err != nil {
		return err
	}

	_, err = repository.Cli.Fetch([]string{c.remoteName})
	if err != nil {
		return err
	}

	var branch string
	if repository.Branch == nil {
		branch, err = repository.DefaultBranch(c.remoteName)
		if err != nil {
			return err
		}
	} else {
		branch = *repository.Branch
	}
	log.Println("Checking out branch", branch)
	_, err = repository.Cli.Checkout([]string{branch})
	if err != nil {
		return err
	}

	if c.config.LfsAutoFetch {
		log.Println("Dealing with LFS")
		totalLfsSize := repository.GetLFSTotalSizeBytes()
		log.Println("Lfs size:", totalLfsSize)

		var stat unix.Statfs_t
		err = unix.Statfs(repository.clonePath, &stat)
		if err != nil {
			return err
		}
		// Available blocks * size per block = available space in bytes
		availableSpace := stat.Bavail * uint64(stat.Bsize)
		if availableSpace < totalLfsSize {
			return fmt.Errorf("not enough free space")
		}
		_, err = repository.Cli.Lfs([]string{"install", "--local"})
		if err != nil {
			return err
		}
		_, err = repository.Cli.Lfs([]string{"pull"})
		if err != nil {
			return err
		}

	}

	log.Println("Dealing with submodules")
	_, err = repository.Cli.Submodule([]string{"update", "--init"})
	if err != nil {
		return fmt.Errorf("failed to inialize submodules: %v", err)
	}

	return nil
}
