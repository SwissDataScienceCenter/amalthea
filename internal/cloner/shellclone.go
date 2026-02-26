package cloner

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "github.com/joho/godotenv/autoload"
)

type User struct {
	Username   string `mapstructure:"user__username"`
	Email      string `mapstructure:"user__email"`
	FullName   string `mapstructure:"user__full_name"`
	RenkuToken string `mapstructure:"user__renku_token"`
}

func (u *User) IsAnonymous() bool {
	return u.RenkuToken == ""
}

type GitProvider struct {
	Id             string `json:"id"`
	AccessTokenUrl string `json:"access_token_url"`
}

type CloneConfig struct {
	MountPath         string `mapstructure:"mount_path"`
	LfsAutoFetch      bool   `mapstructure:"lfs_auto_fetch"`
	IsGitProxyEnabled bool   `mapstructure:"is_git_proxy_enabled"`
	GitProxyPort      int    `mapstructure:"git_proxy_port"`
	Repositories      []Repository
	GitProviders      []GitProvider
	StorageMounts     []string
}

type RefreshTokenAnswer struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

func loadRepositories(config CloneConfig) []Repository {
	template := "GIT_CLONE_REPOSITORIES_%v_"

	repositories := []Repository{}
	i := 0
	for {
		val, ok := os.LookupEnv(fmt.Sprintf(template, i))
		if !ok {
			// All variables have been handled
			break
		}
		var repository Repository
		err := json.Unmarshal([]byte(val), &repository)
		if err != nil {
			log.Fatal("Failed to parse repository: ", err)
		}

		endpoint, err := url.Parse(repository.URL)
		if err != nil {
			log.Fatal("failed to parse", repository.URL, err)
		}
		splittedRepo := strings.FieldsFunc(endpoint.Path, func(c rune) bool { return c == '/' }) // FieldsFunc handles repeated and beginning/ending separator characters more sanely than Split
		if !(len(splittedRepo) > 0) {
			log.Fatal("expecting repo in url path, received: ", endpoint.Path)
		}
		projectName := splittedRepo[len(splittedRepo)-1]
		projectName = strings.TrimSuffix(projectName, ".git")

		repository.SetClonePath(filepath.Join(config.MountPath, projectName))
		repositories = append(repositories, repository)
		i += 1
	}
	return repositories
}

func loadGitProviders() []GitProvider {
	template := "GIT_CLONE_GIT_PROVIDERS_%v_"

	providers := []GitProvider{}

	i := 0
	for {
		val, ok := os.LookupEnv(fmt.Sprintf(template, i))
		if !ok {
			// All variables have been handled
			break
		}
		var provider GitProvider
		err := json.Unmarshal([]byte(val), &provider)
		if err != nil {
			log.Fatal("Failed to parse provider: ", err)
		}
		providers = append(providers, provider)
		i += 1
	}

	return providers
}

func loadStorageMounts() []string {
	template := "GIT_CLONE_STORAGE_MOUNTS_%v"

	mounts := []string{}

	i := 0
	for {
		val, ok := os.LookupEnv(fmt.Sprintf(template, i))
		if !ok {
			// All variables have been handled
			break
		}
		mounts = append(mounts, strings.Trim(val, "\""))
		i += 1
	}

	return mounts
}

func shellClone(cmd *cobra.Command, args []string) {
	v := viper.New()
	v.SetConfigType("env")
	v.SetEnvPrefix("git_clone")
	v.SetEnvKeyReplacer(strings.NewReplacer(`.`, `_`))
	v.AutomaticEnv()

	v.SetDefault("mount_path", "")
	v.SetDefault("lfs_auto_fetch", false)
	v.SetDefault("is_git_proxy_enabled", false)
	v.SetDefault("git_proxy_port", 8080)
	v.SetDefault("user__username", "")
	v.SetDefault("user__email", "")
	v.SetDefault("user__full_name", "")
	v.SetDefault("user__renku_token", "")
	v.SetDefault("user__renku_url", "")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Fatal(err)
		}
	}

	var config CloneConfig
	if err := v.Unmarshal(&config); err != nil {
		log.Fatal(err)
	}

	var user User
	if err := v.Unmarshal(&user); err != nil {
		log.Fatal(err)
	}

	config.Repositories = loadRepositories(config)
	config.GitProviders = loadGitProviders()
	config.StorageMounts = loadStorageMounts()

	cloner := Cloner{config, user, "origin"}

	if err := cloner.Run(); err != nil {
		log.Fatal("failed to clone repo:", err)
	}
}
