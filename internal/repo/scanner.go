package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type RepoInfo struct {
	Path  string
	Owner string
	Name  string
}

func IsGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func GetRepoInfo(path string) (*RepoInfo, error) {
	if !IsGitRepo(path) {
		return nil, nil
	}

	cmd := exec.Command("git", "-C", path, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	remoteURL := strings.TrimSpace(string(output))
	owner, name := parseGitHubURL(remoteURL)
	if owner == "" || name == "" {
		return nil, nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	return &RepoInfo{
		Path:  absPath,
		Owner: owner,
		Name:  name,
	}, nil
}

func parseGitHubURL(url string) (owner, name string) {
	// SSH format: git@github.com:owner/repo.git
	sshPattern := regexp.MustCompile(`git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)
	if matches := sshPattern.FindStringSubmatch(url); len(matches) == 3 {
		return matches[1], matches[2]
	}

	// HTTPS format: https://github.com/owner/repo.git
	httpsPattern := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+?)(?:\.git)?$`)
	if matches := httpsPattern.FindStringSubmatch(url); len(matches) == 3 {
		return matches[1], matches[2]
	}

	return "", ""
}

func ListDirectories(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, entry.Name())
		}
	}

	return dirs, nil
}

func ListAllEntries(path string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []os.DirEntry
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") || entry.Name() == ".." {
			result = append(result, entry)
		}
	}

	return result, nil
}
