package cloner

import (
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func gitExecPath(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		t.Fatalf("git --exec-path: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func runGit(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitServer(t *testing.T, root, backend string, onRequest func(*http.Request)) *httptest.Server {
	t.Helper()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if onRequest != nil {
			onRequest(r)
		}
		(&cgi.Handler{
			Path: filepath.Join(backend, "git-http-backend"),
			Env: []string{
				"GIT_PROJECT_ROOT=" + root,
				"GIT_HTTP_EXPORT_ALL=1",
			},
		}).ServeHTTP(w, r)
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestCloneDoesNotForwardCredentialsToSubmodules(t *testing.T) {
	backend := gitExecPath(t)
	home := t.TempDir()
	gitEnv := []string{
		"HOME=" + home,
		"GIT_CONFIG_GLOBAL=" + filepath.Join(home, "gitconfig"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_TERMINAL_PROMPT=0",
	}

	// Second server hosts the submodule target and records requests.
	rootB := t.TempDir()
	var mu sync.Mutex
	var authSeen bool
	var hitsB int
	serverB := gitServer(t, rootB, backend, func(r *http.Request) {
		mu.Lock()
		hitsB++
		if r.Header.Get("Authorization") != "" {
			authSeen = true
		}
		mu.Unlock()
	})

	// Build the submodule repo repoB.git served by serverB.
	workB := filepath.Join(t.TempDir(), "workB")
	runGit(t, "", gitEnv, "-c", "init.defaultBranch=main", "init", workB)
	if err := os.WriteFile(filepath.Join(workB, "b.txt"), []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, workB, gitEnv, "add", ".")
	runGit(t, workB, gitEnv, "commit", "-m", "init b")
	runGit(t, "", gitEnv, "clone", "--bare", workB, filepath.Join(rootB, "repoB.git"))

	// First server hosts repoA.git which declares a submodule pointing at serverB.
	rootA := t.TempDir()
	serverA := gitServer(t, rootA, backend, nil)

	workA := filepath.Join(t.TempDir(), "workA")
	runGit(t, "", gitEnv, "-c", "init.defaultBranch=main", "init", workA)
	if err := os.WriteFile(filepath.Join(workA, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, workA, gitEnv, "add", ".")
	runGit(t, workA, gitEnv, "commit", "-m", "init a")
	runGit(t, workA, gitEnv, "submodule", "add", serverB.URL+"/repoB.git", "sub")
	runGit(t, workA, gitEnv, "commit", "-m", "add submodule")
	runGit(t, "", gitEnv, "clone", "--bare", workA, filepath.Join(rootA, "repoA.git"))

	// Drive the cloner with credentials against serverA.
	user := "git"
	cfg := filepath.Join(t.TempDir(), "clone-config.yaml")
	if err := os.WriteFile(cfg, []byte("username: "+user+"\npassword: secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Ignore requests made by the test setup above; only observe the cloner.
	mu.Lock()
	hitsB = 0
	authSeen = false
	mu.Unlock()

	dst := t.TempDir()
	remote = serverA.URL + "/repoA.git"
	revision = "refs/heads/main"
	path = dst
	configPath = cfg
	preCloningStrategy = newEnum(PreCloningStrategies, NoStrategy)
	t.Cleanup(func() { remote = ""; revision = ""; path = ""; configPath = "" })

	clone(nil, nil)

	if _, err := os.Stat(filepath.Join(dst, "repoA", "a.txt")); err != nil {
		t.Fatalf("expected repoA to be cloned: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if authSeen {
		t.Fatalf("credentials were forwarded to the submodule host")
	}
	if hitsB != 0 {
		t.Fatalf("submodule host was contacted %d time(s); submodules should not be recursed", hitsB)
	}
}
