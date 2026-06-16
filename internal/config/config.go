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
	Collaboration string   `yaml:"collaboration"`
	ForkRemote    string   `yaml:"forkRemote"`
	PRTitle       string   `yaml:"prTitle"`
	PRBody        string   `yaml:"prBody"`
	Draft         bool     `yaml:"draft"`
	Labels        []string `yaml:"labels"`
	Reviewers     []string `yaml:"reviewers"`
}

type Task struct {
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	Group       string   `yaml:"group"`
	From        string   `yaml:"from"`
	To          []string `yaml:"to"`
	Mode        string   `yaml:"mode"`
}

func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := validateUniqueTaskNames(&node); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	var cfg Config
	if err := node.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	ApplyDefaults(&cfg)
	return cfg, nil
}

func validateUniqueTaskNames(doc *yaml.Node) error {
	if doc == nil || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	tasks := mappingValue(root, "tasks")
	if tasks == nil || tasks.Kind != yaml.MappingNode {
		return nil
	}
	seen := map[string]struct{}{}
	for i := 0; i+1 < len(tasks.Content); i += 2 {
		name := tasks.Content[i].Value
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate task name %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
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
	if cfg.Defaults.GitHub.PRTitle == "" {
		cfg.Defaults.GitHub.PRTitle = "Propagate {source} to {target}"
	}
	if cfg.Defaults.GitHub.PRBody == "" {
		cfg.Defaults.GitHub.PRBody = "Created by Git Spread for {target}."
	}
	if cfg.Tasks == nil {
		cfg.Tasks = map[string]Task{}
	}
}
