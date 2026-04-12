package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_EnvVarsOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_AUTOCOMMIT_AGENT_TYPE", "claude")
	t.Setenv("GIT_AUTOCOMMIT_AGENT_BINARY", "/usr/local/bin/claude")
	t.Setenv("GIT_AUTOCOMMIT_MODEL", "opus")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Agent.Type != "claude" {
		t.Errorf("Type = %q, want claude", cfg.Agent.Type)
	}
	if cfg.Agent.Binary != "/usr/local/bin/claude" {
		t.Errorf("Binary = %q, want /usr/local/bin/claude", cfg.Agent.Binary)
	}
	if cfg.Agent.Model != "opus" {
		t.Errorf("Model = %q, want opus", cfg.Agent.Model)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Provide only Type to satisfy the "at least one field" check;
	// Binary and Model should get their defaults.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_AUTOCOMMIT_AGENT_TYPE", "claude")
	t.Setenv("GIT_AUTOCOMMIT_AGENT_BINARY", "")
	t.Setenv("GIT_AUTOCOMMIT_MODEL", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Agent.Binary != "claude" {
		t.Errorf("default Binary = %q, want claude", cfg.Agent.Binary)
	}
	if cfg.Agent.Model != "sonnet" {
		t.Errorf("default Model = %q, want sonnet", cfg.Agent.Model)
	}
}

func TestLoad_MissingAll_Error(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_AUTOCOMMIT_AGENT_TYPE", "")
	t.Setenv("GIT_AUTOCOMMIT_AGENT_BINARY", "")
	t.Setenv("GIT_AUTOCOMMIT_MODEL", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	_, err := Load()
	if err == nil {
		t.Error("expected error when no configuration source provides values")
	}
}

func TestLoad_FileConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_AUTOCOMMIT_AGENT_TYPE", "")
	t.Setenv("GIT_AUTOCOMMIT_AGENT_BINARY", "")
	t.Setenv("GIT_AUTOCOMMIT_MODEL", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	cfgDir := filepath.Join(home, ".config", "git-autocommit")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	toml := `
[agent]
type   = "anthropic"
binary = "my-binary"
model  = "claude-opus-4-5"
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Agent.Type != "anthropic" {
		t.Errorf("Type = %q, want anthropic", cfg.Agent.Type)
	}
	if cfg.Agent.Model != "claude-opus-4-5" {
		t.Errorf("Model = %q, want claude-opus-4-5", cfg.Agent.Model)
	}
	if cfg.Agent.Binary != "my-binary" {
		t.Errorf("Binary = %q, want my-binary", cfg.Agent.Binary)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_AUTOCOMMIT_AGENT_TYPE", "")
	t.Setenv("GIT_AUTOCOMMIT_MODEL", "overridden-model")
	t.Setenv("GIT_AUTOCOMMIT_AGENT_BINARY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	cfgDir := filepath.Join(home, ".config", "git-autocommit")
	os.MkdirAll(cfgDir, 0755)
	os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(`
[agent]
type  = "claude"
model = "original-model"
`), 0644)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Agent.Model != "overridden-model" {
		t.Errorf("Model = %q, want overridden-model", cfg.Agent.Model)
	}
}
