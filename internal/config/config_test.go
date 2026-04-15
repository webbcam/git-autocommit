package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_EnvVarsOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_AUTOCOMMIT_PROVIDER_TYPE", "claude")
	t.Setenv("GIT_AUTOCOMMIT_PROVIDER_BINARY", "/usr/local/bin/claude")
	t.Setenv("GIT_AUTOCOMMIT_MODEL", "opus")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Provider.Type != "claude" {
		t.Errorf("Type = %q, want claude", cfg.Provider.Type)
	}
	if cfg.Provider.Binary != "/usr/local/bin/claude" {
		t.Errorf("Binary = %q, want /usr/local/bin/claude", cfg.Provider.Binary)
	}
	if cfg.Provider.Model != "opus" {
		t.Errorf("Model = %q, want opus", cfg.Provider.Model)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Provide only Type to satisfy the "at least one field" check;
	// Binary and Model should get their defaults.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_AUTOCOMMIT_PROVIDER_TYPE", "claude")
	t.Setenv("GIT_AUTOCOMMIT_PROVIDER_BINARY", "")
	t.Setenv("GIT_AUTOCOMMIT_MODEL", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Provider.Binary != "claude" {
		t.Errorf("default Binary = %q, want claude", cfg.Provider.Binary)
	}
	if cfg.Provider.Model != "sonnet" {
		t.Errorf("default Model = %q, want sonnet", cfg.Provider.Model)
	}
}

func TestLoad_MissingAll_Error(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_AUTOCOMMIT_PROVIDER_TYPE", "")
	t.Setenv("GIT_AUTOCOMMIT_PROVIDER_BINARY", "")
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
	t.Setenv("GIT_AUTOCOMMIT_PROVIDER_TYPE", "")
	t.Setenv("GIT_AUTOCOMMIT_PROVIDER_BINARY", "")
	t.Setenv("GIT_AUTOCOMMIT_MODEL", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	cfgDir := filepath.Join(home, ".config", "git-autocommit")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	toml := `
[provider]
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

	if cfg.Provider.Type != "anthropic" {
		t.Errorf("Type = %q, want anthropic", cfg.Provider.Type)
	}
	if cfg.Provider.Model != "claude-opus-4-5" {
		t.Errorf("Model = %q, want claude-opus-4-5", cfg.Provider.Model)
	}
	if cfg.Provider.Binary != "my-binary" {
		t.Errorf("Binary = %q, want my-binary", cfg.Provider.Binary)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_AUTOCOMMIT_PROVIDER_TYPE", "")
	t.Setenv("GIT_AUTOCOMMIT_MODEL", "overridden-model")
	t.Setenv("GIT_AUTOCOMMIT_PROVIDER_BINARY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	cfgDir := filepath.Join(home, ".config", "git-autocommit")
	os.MkdirAll(cfgDir, 0755)
	os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(`
[provider]
type  = "claude"
model = "original-model"
`), 0644)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Provider.Model != "overridden-model" {
		t.Errorf("Model = %q, want overridden-model", cfg.Provider.Model)
	}
}
