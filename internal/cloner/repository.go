package cloner

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Build a list of all the parent folder of the given path
func parents(path string) []string {
	parents := []string{}
	parent := filepath.Dir(path)
	for parent != "/" && parent != "." {
		// if parent == "/" || parent == "." {
		// 	break
		// }
		parents = append(parents, parent)
		parent = filepath.Dir(parent)
	}

	return parents
}

type Repository struct {
	URL       string
	Provider  string
	Branch    *string
	CommitSha *string

	clonePath string
	Cli       *GitCli
}

func (r *Repository) SetClonePath(clonePath string) {
	r.clonePath = clonePath
	log.Println("Checking if clone path:", r.clonePath, "exists")
	_, err := os.Stat(r.clonePath)
	if err != nil {
		log.Println("Creating clone path")
		if errors.Is(err, os.ErrNotExist) {
			err := os.MkdirAll(r.clonePath, os.ModePerm)
			if err != nil {
				log.Fatal("failed to create clone path:", err)
			}
		} else {
			log.Fatal("unexpected error:", err)
		}
	}

	r.Cli = NewGitCli(clonePath)
}

func (r *Repository) Exists() bool {
	is_inside, err := r.Cli.RevParse([]string{"--is-inside-work-tree"})
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(is_inside), "true")
}

func (r *Repository) DefaultBranch(remoteName string) (string, error) {
	_, err := r.Cli.Remote([]string{"set-head", remoteName, "--auto"})
	if err != nil {
		return "", err
	}
	ref, err := r.Cli.SymbolicRef([]string{fmt.Sprintf("refs/remotes/%v/HEAD", remoteName)})
	if err != nil {
		return "", err
	}
	rx := regexp.MustCompile(fmt.Sprintf(`refs/remotes/%v/(?P<branch>.*)`, remoteName))
	branch := rx.FindString(ref)

	if branch == "" {
		return "", fmt.Errorf("branch does not exist")
	}
	log.Println("Default branch is", branch)
	return branch, nil
}

// Get the total size of all LFS files in bytes.
func (r *Repository) GetLFSTotalSizeBytes() uint64 {
	res, err := r.Cli.Lfs([]string{"ls-files", "--json"})
	if err != nil {
		return 0
	}

	type File struct {
		Name string `json:"name"`
		Size uint64 `json:"size"`
	}

	type LsFiles struct {
		Files []File `json:"files"`
	}

	var lsFiles LsFiles
	err = json.Unmarshal([]byte(res), &lsFiles)
	if err != nil {
		log.Println("Failed to parse ls-files output:", err)
		return 0
	}

	totalSize := uint64(0)
	for _, file := range lsFiles.Files {
		totalSize += file.Size
	}
	return totalSize
}

func (r *Repository) ExcludePathFromGit(paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	excludesPath := filepath.Join(r.clonePath, ".git", "info", "exclude")
	f, err := os.OpenFile(excludesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if slices.Contains(parents(path), r.clonePath) {
			excludePath, err := filepath.Rel(r.clonePath, path)
			if err != nil {
				return err
			}
			log.Println("Excluding:", excludePath)
			if _, err := fmt.Fprintf(f, "%v\n", excludePath); err != nil {
				return err
			}
		}
	}
	return f.Close()
}
