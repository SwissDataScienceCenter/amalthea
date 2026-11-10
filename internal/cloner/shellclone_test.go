package cloner

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var testCommitter = []string{
	"GIT_AUTHOR_NAME=test",
	"GIT_AUTHOR_EMAIL=test@example.com",
	"GIT_COMMITTER_NAME=test",
	"GIT_COMMITTER_EMAIL=test@example.com",
}

func setupShellClone(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "git-lfs"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	script, err := os.ReadFile(filepath.Join("..", "..", askpass))
	if err != nil {
		t.Fatal(err)
	}
	script = []byte(strings.Replace(string(script), "#!/usr/bin/env sh", "#!/usr/bin/env bash", 1))
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, askpass), script, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
}

func createBareRepo(t *testing.T, dest string) string {
	t.Helper()
	work := filepath.Join(t.TempDir(), "work")
	runGit(t, "", testCommitter, "-c", "init.defaultBranch=main", "init", work)
	if err := os.WriteFile(filepath.Join(work, "b.txt"), []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, testCommitter, "add", ".")
	runGit(t, work, testCommitter, "commit", "-m", "init")
	runGit(t, "", testCommitter, "clone", "--bare", work, dest)
	return runGit(t, work, testCommitter, "rev-parse", "HEAD")
}

func createBareRepoWithSubmodule(t *testing.T, dest, submoduleURL, submoduleCommit string) {
	t.Helper()
	work := filepath.Join(t.TempDir(), "work")
	runGit(t, "", testCommitter, "-c", "init.defaultBranch=main", "init", work)
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitmodules := "[submodule \"vendor\"]\n\tpath = vendor\n\turl = " + submoduleURL + "\n"
	if err := os.WriteFile(filepath.Join(work, ".gitmodules"), []byte(gitmodules), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, testCommitter, "add", "a.txt", ".gitmodules")
	runGit(t, work, testCommitter, "update-index", "--add", "--cacheinfo", "160000,"+submoduleCommit+",vendor")
	runGit(t, work, testCommitter, "commit", "-m", "add submodule")
	runGit(t, "", testCommitter, "clone", "--bare", work, dest)
}

func runShellClone(t *testing.T, repoURL, token string) string {
	t.Helper()
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"` + token + `","expires_at":0}`))
	}))
	t.Cleanup(tokenServer.Close)

	mount := t.TempDir()
	clonePath := filepath.Join(mount, "repo")
	repo := Repository{URL: repoURL, Provider: "github"}
	repo.SetClonePath(clonePath)
	c := Cloner{
		config: CloneConfig{
			MountPath:    mount,
			Repositories: []Repository{repo},
			GitProviders: []GitProvider{{Id: "github", AccessTokenUrl: tokenServer.URL}},
		},
		user:       User{RenkuToken: "renku-token"},
		remoteName: "origin",
	}
	if err := c.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return clonePath
}

func TestShellCloneDoesNotSendCredentialsToOtherHosts(t *testing.T) {
	backend := gitExecPath(t)
	setupShellClone(t)

	var mu sync.Mutex
	var leaked string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); ok {
			mu.Lock()
			leaked = user + ":" + pass
			mu.Unlock()
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(other.Close)
	otherURL := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)

	root := t.TempDir()
	srv := gitServer(t, root, backend, nil)
	createBareRepoWithSubmodule(t, filepath.Join(root, "repo.git"), otherURL+"/lib.git", strings.Repeat("1", 40))

	runShellClone(t, srv.URL+"/repo.git", "provider-token")

	mu.Lock()
	defer mu.Unlock()
	if leaked != "" {
		t.Fatalf("submodule host %s received credentials %q", otherURL, leaked)
	}
}

func TestShellCloneSameHostSubmoduleAuthenticates(t *testing.T) {
	backend := gitExecPath(t)
	setupShellClone(t)

	const token = "same-host-token"
	root := t.TempDir()
	h := gitBackend(root, backend)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "oauth2" || pass != token {
			w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	libHead := createBareRepo(t, filepath.Join(root, "lib.git"))
	createBareRepoWithSubmodule(t, filepath.Join(root, "repo.git"), srv.URL+"/lib.git", libHead)

	clonePath := runShellClone(t, srv.URL+"/repo.git", token)

	if b, err := os.ReadFile(filepath.Join(clonePath, "ERROR")); err == nil {
		t.Fatalf("clone failed: %s", b)
	}
	if _, err := os.Stat(filepath.Join(clonePath, "vendor", "b.txt")); err != nil {
		t.Fatalf("submodule was not checked out: %v", err)
	}
}
