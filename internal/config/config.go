package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	AppName      = "daily-work"
	DefaultModel = "llama3.2:3b"
)

// Config is the on-disk configuration (no secrets).
type Config struct {
	AI       AIConfig          `yaml:"ai"`
	Projects map[string]string `yaml:"projects"`
}

type AIConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	Host     string `yaml:"host,omitempty"` // Ollama host, default localhost
}

func Default() Config {
	return Config{
		AI: AIConfig{
			Provider: "ollama",
			Model:    DefaultModel,
			Host:     "http://localhost:11434",
		},
		Projects: map[string]string{},
	}
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", AppName), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (Config, error) {
	cfg := Default()
	path, err := Path()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	// Force local-only defaults if missing or still on old Gemini settings.
	if cfg.AI.Provider == "" || cfg.AI.Provider == "gemini" {
		cfg.AI.Provider = "ollama"
	}
	if cfg.AI.Model == "" || strings.HasPrefix(cfg.AI.Model, "gemini") {
		cfg.AI.Model = DefaultModel
	}
	if cfg.AI.Host == "" {
		cfg.AI.Host = "http://localhost:11434"
	}
	if cfg.Projects == nil {
		cfg.Projects = map[string]string{}
	}
	return cfg, nil
}

func Save(cfg Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path, err := Path()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ProjectShortName returns a configured override or empty string.
func (c Config) ProjectShortName(repoName string) string {
	if c.Projects == nil {
		return ""
	}
	return c.Projects[repoName]
}
