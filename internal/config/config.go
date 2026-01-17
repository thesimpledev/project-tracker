package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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

	data, err := os.ReadFile(path)
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

	return &cfg, nil
}

func (c *Config) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
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

	return os.WriteFile(path, data, 0644)
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

func (c *Config) SaveProfile(name string) error {
	dir, err := profilesDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, name+".json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func LoadProfile(name string) (*Config, error) {
	dir, err := profilesDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

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
