package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const appName = "project-tracker"

type Repo struct {
	Path  string `json:"path"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

type Config struct {
	Repos         []Repo `json:"repos"`
	ProfileName   string `json:"profile_name,omitempty"`
	LastOpenedDir string `json:"last_opened_dir,omitempty"`
	LastPath      string `json:"last_path,omitempty"`
}

func configDir() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, appName), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path) // #nosec G304 -- path is a fixed filename under the user config dir
	if os.IsNotExist(err) {
		return &Config{Repos: []Repo{}}, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.sanitizePaths()
	return &cfg, nil
}

// ValidRepoPath returns the cleaned absolute form of path if it points at an
// existing directory, or "" if it is not usable. All repo paths coming from
// config files or user input must pass through here before being handed to
// file operations or subprocesses.
func ValidRepoPath(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return ""
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return ""
	}
	return abs
}

// sanitizePaths drops repo entries whose paths are missing or do not resolve
// to an existing directory, so nothing downstream ever sees a bad path.
func (c *Config) sanitizePaths() {
	valid := c.Repos[:0]
	for _, r := range c.Repos {
		if p := ValidRepoPath(r.Path); p != "" {
			r.Path = p
			valid = append(valid, r)
		}
	}
	c.Repos = valid

	if c.LastOpenedDir != "" {
		c.LastOpenedDir = ValidRepoPath(c.LastOpenedDir)
	}
}

func (c *Config) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func (c *Config) AddRepo(repo Repo) {
	for _, r := range c.Repos {
		if r.Owner == repo.Owner && r.Name == repo.Name {
			return
		}
	}
	c.Repos = append(c.Repos, repo)
}

func (c *Config) RemoveRepo(owner, name string) {
	for i, r := range c.Repos {
		if r.Owner == owner && r.Name == name {
			c.Repos = append(c.Repos[:i], c.Repos[i+1:]...)
			return
		}
	}
}

func (c *Config) HasLastOpenedDir() bool {
	return c.LastOpenedDir != ""
}

// Profile support

func profilesDir() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles"), nil
}

// validProfileName rejects names that could escape the profiles directory
// (path separators, "..") or hide files (leading dot).
func validProfileName(name string) error {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") || strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid profile name: %q", name)
	}
	return nil
}

func (c *Config) SaveProfile(name string) error {
	if err := validProfileName(name); err != nil {
		return err
	}

	dir, err := profilesDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path := filepath.Join(dir, name+".json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func LoadProfile(name string) (*Config, error) {
	if err := validProfileName(name); err != nil {
		return nil, err
	}

	dir, err := profilesDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, name+".json")
	data, err := os.ReadFile(path) // #nosec G304 -- path is the config dir plus a validated profile name
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.sanitizePaths()
	return &cfg, nil
}

func ListProfiles() ([]string, error) {
	dir, err := profilesDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	var profiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			profiles = append(profiles, name[:len(name)-5])
		}
	}

	return profiles, nil
}
