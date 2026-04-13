package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds the configuration for git-autocommit.
type Config struct {
	Agent AgentConfig `toml:"agent"`
}

// AgentConfig holds agent-specific configuration.
type AgentConfig struct {
	Type    string `toml:"type"`
	Binary  string `toml:"binary"`
	Model   string `toml:"model"`
	APIKey  string `toml:"api_key"`
	BaseURL string `toml:"base_url"`
}

// Load loads configuration from file and/or environment variables.
// Environment variables override file-based config.
// Returns an error if no configuration source provides the required fields.
func Load() (*Config, error) {
	cfg := &Config{}

	// Try to load from file first
	configPath, err := DefaultConfigPath()
	if err == nil {
		if _, statErr := os.Stat(configPath); statErr == nil {
			if _, parseErr := toml.DecodeFile(configPath, cfg); parseErr != nil {
				return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, parseErr)
			}
		}
	}

	// Apply environment variable overrides
	if v := os.Getenv("GIT_AUTOCOMMIT_AGENT_TYPE"); v != "" {
		cfg.Agent.Type = v
	}
	if v := os.Getenv("GIT_AUTOCOMMIT_AGENT_BINARY"); v != "" {
		cfg.Agent.Binary = v
	}
	if v := os.Getenv("GIT_AUTOCOMMIT_MODEL"); v != "" {
		cfg.Agent.Model = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" && cfg.Agent.APIKey == "" {
		cfg.Agent.APIKey = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" && cfg.Agent.APIKey == "" {
		cfg.Agent.APIKey = v
	}

	// Validate required fields
	if cfg.Agent.Type == "" && cfg.Agent.Binary == "" && cfg.Agent.Model == "" {
		return nil, fmt.Errorf(
			"no configuration found. Run 'git-autocommit config' or create %s manually.\n"+
				"Environment variable overrides:\n"+
				"  GIT_AUTOCOMMIT_AGENT_TYPE\n"+
				"  GIT_AUTOCOMMIT_AGENT_BINARY\n"+
				"  GIT_AUTOCOMMIT_MODEL",
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

// Save writes cfg to the default config file path, creating directories as needed.
func Save(cfg *Config) error {
	configPath, err := DefaultConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	f, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "git-autocommit", "config.toml"), nil
}
