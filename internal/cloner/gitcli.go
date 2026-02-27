package cloner

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// askpass is the name of the script that will
// grab the user name and password from environment
// variables
const askpass = "env-pass.sh"

type GitCli struct {
	repoDirectory string
	username      string
	password      string
}

func (g *GitCli) SetCredentials(username string, password string) {
	g.username = username
	g.password = password
}

func (g *GitCli) execute(cmd string, args []string) (string, error) {
	stdoutBuf := bytes.NewBufferString("")
	stderrBuf := bytes.NewBufferString("")
	fullargs := append([]string{cmd}, args...)
	git := exec.Command("git", fullargs...)
	env := os.Environ()

	if g.username != "" {
		env = append(env, fmt.Sprintf("GIT_USERNAME=%v", g.username))
	}
	if g.password != "" {
		env = append(env, fmt.Sprintf("GIT_PASSWORD=%v", g.password))
	}

	if g.username != "" && g.password != "" {
		path, err := os.Getwd()
		if err != nil {
			return "", err
		}

		askpassPath := fmt.Sprintf("GIT_ASKPASS=%v", filepath.Join(path, askpass))
		env = append(env, askpassPath)
	}

	git.Env = env
	git.Stdout = stdoutBuf
	git.Stderr = stderrBuf
	git.Dir = g.repoDirectory

	err := git.Run()

	if err != nil {
		return "", fmt.Errorf("git failed: %v, %v", err, stderrBuf)
	}

	return stdoutBuf.String(), nil
}

func (g *GitCli) Config(args []string) (string, error) {
	return g.execute("config", args)
}

func (g *GitCli) Push(args []string) (string, error) {
	return g.execute("push", args)
}

func (g *GitCli) Submodule(args []string) (string, error) {
	return g.execute("submodule", args)
}

func (g *GitCli) Checkout(args []string) (string, error) {
	return g.execute("checkout", args)
}

func (g *GitCli) Lfs(args []string) (string, error) {
	return g.execute("lfs", args)
}

func (g *GitCli) Branch(args []string) (string, error) {
	return g.execute("branch", args)
}
func (g *GitCli) Remote(args []string) (string, error) {
	return g.execute("remote", args)
}

func (g *GitCli) Reset(args []string) (string, error) {
	return g.execute("reset", args)
}

func (g *GitCli) Fetch(args []string) (string, error) {
	return g.execute("fetch", args)
}

func (g *GitCli) RevParse(args []string) (string, error) {
	return g.execute("rev-parse", args)
}

func (g *GitCli) Init(args []string) (string, error) {
	return g.execute("init", args)
}

func (g *GitCli) Status(args []string) (string, error) {
	return g.execute("status", args)
}

func (g *GitCli) Add(args []string) (string, error) {
	return g.execute("add", args)
}

func (g *GitCli) Commit(args []string) (string, error) {
	return g.execute("commit", args)
}

func (g *GitCli) Clean(args []string) (string, error) {
	return g.execute("clean", args)
}

func (g *GitCli) Pull(args []string) (string, error) {
	return g.execute("pull", args)
}

func (g *GitCli) Clone(args []string) (string, error) {
	return g.execute("clone", args)
}

func (g *GitCli) Diff(args []string) (string, error) {
	return g.execute("diff", args)
}

func (g *GitCli) SymbolicRef(args []string) (string, error) {
	return g.execute("symbolic-ref", args)
}

func NewGitCli(repoDirectory string) *GitCli {
	_, err := os.Stat(repoDirectory)
	if err != nil {
		log.Fatal(err)
	}

	return &GitCli{repoDirectory, "", ""}
}
