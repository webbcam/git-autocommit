package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds the configuration for git-ai-commit.
type Config struct {
	Agent AgentConfig `toml:"agent"`
}

// AgentConfig holds agent-specific configuration.
type AgentConfig struct {
	Type   string `toml:"type"`
	Binary string `toml:"binary"`
	Model  string `toml:"model"`
}

// Load loads configuration from file and/or environment variables.
// Environment variables override file-based config.
// Returns an error if no configuration source provides the required fields.
func Load() (*Config, error) {
	cfg := &Config{}

	// Try to load from file first
	configPath, err := defaultConfigPath()
	if err == nil {
		if _, statErr := os.Stat(configPath); statErr == nil {
			if _, parseErr := toml.DecodeFile(configPath, cfg); parseErr != nil {
				return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, parseErr)
			}
		}
	}

	// Apply environment variable overrides
	if v := os.Getenv("GIT_AI_COMMIT_AGENT_TYPE"); v != "" {
		cfg.Agent.Type = v
	}
	if v := os.Getenv("GIT_AI_COMMIT_AGENT_BINARY"); v != "" {
		cfg.Agent.Binary = v
	}
	if v := os.Getenv("GIT_AI_COMMIT_MODEL"); v != "" {
		cfg.Agent.Model = v
	}

	// Validate required fields
	if cfg.Agent.Type == "" && cfg.Agent.Binary == "" && cfg.Agent.Model == "" {
		return nil, fmt.Errorf(
			"no configuration found. Create %s or set environment variables:\n"+
				"  GIT_AI_COMMIT_AGENT_TYPE\n"+
				"  GIT_AI_COMMIT_AGENT_BINARY\n"+
				"  GIT_AI_COMMIT_MODEL",
			configPath,
		)
	}

	// Apply defaults for any still-missing fields
	if cfg.Agent.Type == "" {
		cfg.Agent.Type = "claude"
	}
	if cfg.Agent.Binary == "" {
		cfg.Agent.Binary = "claude"
	}
	if cfg.Agent.Model == "" {
		cfg.Agent.Model = "sonnet"
	}

	return cfg, nil
}

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "git-ai-commit", "config.toml"), nil
}
