package config

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Version  int             `yaml:"version"`
	Defaults Defaults        `yaml:"defaults"`
	Tasks    map[string]Task `yaml:"tasks"`
}

type Defaults struct {
	Mode         string         `yaml:"mode"`
	Remote       string         `yaml:"remote"`
	Workspace    string         `yaml:"workspace"`
	WorkspaceDir string         `yaml:"workspaceDir"`
	Editor       string         `yaml:"editor"`
	GitHub       GitHubDefaults `yaml:"github"`
}

type GitHubDefaults struct {
	Collaboration string `yaml:"collaboration"`
	ForkRemote    string `yaml:"forkRemote"`
}

type Task struct {
	Type string   `yaml:"type"`
	From string   `yaml:"from"`
	To   []string `yaml:"to"`
	Mode string   `yaml:"mode"`
}

func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	ApplyDefaults(&cfg)
	return cfg, nil
}

func ApplyDefaults(cfg *Config) {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Defaults.Mode == "" {
		cfg.Defaults.Mode = "direct"
	}
	if cfg.Defaults.Remote == "" {
		cfg.Defaults.Remote = "origin"
	}
	if cfg.Defaults.Workspace == "" {
		cfg.Defaults.Workspace = "isolated"
	}
	if cfg.Defaults.WorkspaceDir == "" {
		cfg.Defaults.WorkspaceDir = ".spread"
	}
	if cfg.Defaults.Editor == "" {
		cfg.Defaults.Editor = "auto"
	}
	if cfg.Defaults.GitHub.Collaboration == "" {
		cfg.Defaults.GitHub.Collaboration = "auto"
	}
	if cfg.Defaults.GitHub.ForkRemote == "" {
		cfg.Defaults.GitHub.ForkRemote = "fork"
	}
	if cfg.Tasks == nil {
		cfg.Tasks = map[string]Task{}
	}
}
