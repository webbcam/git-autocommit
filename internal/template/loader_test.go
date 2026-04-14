package tmpl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_PathLike_WithSlash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.tmpl")
	if err := os.WriteFile(path, []byte("---\nname: custom\n---\ncustom body\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tmpl, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tmpl.Name != "custom" {
		t.Errorf("Name = %q, want custom", tmpl.Name)
	}
}

func TestLoad_PathLike_DotTmplSuffix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mytemplate.tmpl")
	if err := os.WriteFile(path, []byte("body text"), 0644); err != nil {
		t.Fatal(err)
	}
	// Use a relative path via OS chdir isn't safe in parallel tests, so use
	// absolute path which still has a path separator.
	tmpl, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tmpl.RawBody != "body text" {
		t.Errorf("RawBody = %q, want %q", tmpl.RawBody, "body text")
	}
}

func TestLoad_BuiltinFull(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate from real user templates

	tmpl, err := Load("full")
	if err != nil {
		t.Fatalf("Load full: %v", err)
	}
	if tmpl.Name != "full" {
		t.Errorf("Name = %q, want full", tmpl.Name)
	}
	if tmpl.MaxSubjectLength != 80 {
		t.Errorf("MaxSubjectLength = %d, want 80", tmpl.MaxSubjectLength)
	}
}

func TestLoad_BuiltinShort(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	tmpl, err := Load("short")
	if err != nil {
		t.Fatalf("Load short: %v", err)
	}
	if tmpl.Name != "short" {
		t.Errorf("Name = %q, want short", tmpl.Name)
	}
	if tmpl.MaxSubjectLength != 72 {
		t.Errorf("MaxSubjectLength = %d, want 72", tmpl.MaxSubjectLength)
	}
}

func TestLoad_UserDirOverridesBuiltin(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)

	// Write a user "full" template that overrides the built-in.
	tmplDir := filepath.Join(userDir, "git-autocommit", "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	customContent := "---\nname: full\ndescription: my custom full\n---\ncustom body\n"
	if err := os.WriteFile(filepath.Join(tmplDir, "full.tmpl"), []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	tmpl, err := Load("full")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tmpl.Description != "my custom full" {
		t.Errorf("Description = %q, want user override", tmpl.Description)
	}
	if tmpl.RawBody != "custom body\n" {
		t.Errorf("RawBody = %q, want custom body", tmpl.RawBody)
	}
}

func TestLoad_NotFound(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := Load("nonexistent-template")
	if err == nil {
		t.Fatal("expected error for missing template, got nil")
	}
}

func TestLooksLikePath(t *testing.T) {
	tests := []struct {
		ref  string
		want bool
	}{
		{"full", false},
		{"short", false},
		{"conventional", false},
		{"./my.tmpl", true},
		{"/abs/path.tmpl", true},
		{"relative/path", true},
		{"my.tmpl", true},
		{"templates/custom.tmpl", true},
	}
	for _, tt := range tests {
		got := looksLikePath(tt.ref)
		if got != tt.want {
			t.Errorf("looksLikePath(%q) = %v, want %v", tt.ref, got, tt.want)
		}
	}
}

func TestLoadRaw_Builtin(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	raw, err := LoadRaw("full")
	if err != nil {
		t.Fatalf("LoadRaw: %v", err)
	}
	if raw == "" {
		t.Error("expected non-empty raw content")
	}
}

func TestLoadRaw_NotFound(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := LoadRaw("no-such-template")
	if err == nil {
		t.Fatal("expected error for missing template")
	}
}
