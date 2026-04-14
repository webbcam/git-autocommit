package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ---- NormalizeRemoteURL ----

func TestNormalizeRemoteURL(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"git@github.com:foo/bar.git", "github.com/foo/bar"},
		{"git@github.com:foo/bar", "github.com/foo/bar"},
		{"https://github.com/foo/bar.git", "github.com/foo/bar"},
		{"https://github.com/foo/bar", "github.com/foo/bar"},
		{"http://github.com/foo/bar.git", "github.com/foo/bar"},
		{"ssh://github.com/foo/bar.git", "github.com/foo/bar"},
		{"github.com/foo/bar", "github.com/foo/bar"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormalizeRemoteURL(tt.raw)
		if got != tt.want {
			t.Errorf("NormalizeRemoteURL(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

// ---- matchesRule ----

func TestMatchesRule_PathOnly(t *testing.T) {
	home, _ := os.UserHomeDir()
	rule := ProjectRule{
		MatchPath: filepath.Join(home, "work", "**"),
		Template:  "conventional",
	}

	repoRoot := filepath.Join(home, "work", "myproject")
	if !matchesRule(rule, repoRoot, "") {
		t.Errorf("expected rule to match %q", repoRoot)
	}

	otherRoot := filepath.Join(home, "personal", "myproject")
	if matchesRule(rule, otherRoot, "") {
		t.Errorf("expected rule NOT to match %q", otherRoot)
	}
}

func TestMatchesRule_RemoteOnly(t *testing.T) {
	rule := ProjectRule{
		MatchRemote: "github.com/openclaw/*",
		Template:    "conventional",
	}

	if !matchesRule(rule, "", "github.com/openclaw/myrepo") {
		t.Error("expected rule to match openclaw remote")
	}
	if matchesRule(rule, "", "github.com/other/myrepo") {
		t.Error("expected rule NOT to match other org remote")
	}
}

func TestMatchesRule_BothMustMatch(t *testing.T) {
	home, _ := os.UserHomeDir()
	rule := ProjectRule{
		MatchPath:   filepath.Join(home, "work", "**"),
		MatchRemote: "github.com/acme/*",
		Template:    "acme",
	}

	workRoot := filepath.Join(home, "work", "myrepo")

	// Both match
	if !matchesRule(rule, workRoot, "github.com/acme/myrepo") {
		t.Error("expected both-match to succeed")
	}

	// Path matches, remote doesn't
	if matchesRule(rule, workRoot, "github.com/other/myrepo") {
		t.Error("expected to fail when remote doesn't match")
	}

	// Remote matches, path doesn't
	if matchesRule(rule, "/home/other/myrepo", "github.com/acme/myrepo") {
		t.Error("expected to fail when path doesn't match")
	}
}

func TestMatchesRule_NoMatcherSet(t *testing.T) {
	rule := ProjectRule{Template: "full"}
	if matchesRule(rule, "/any/path", "github.com/any/repo") {
		t.Error("rule with no matchers should not match")
	}
}

func TestMatchesRule_EmptyTemplate(t *testing.T) {
	rule := ProjectRule{MatchRemote: "github.com/**"}
	if matchesRule(rule, "", "github.com/foo/bar") {
		t.Error("rule with empty template should not match")
	}
}

// ---- ResolveTemplate precedence ----

func TestResolveTemplate_FlagTemplateWins(t *testing.T) {
	t.Setenv("GIT_AUTOCOMMIT_TEMPLATE", "env-template")
	ref, err := ResolveTemplate(ResolveInput{
		FlagTemplate: "my-template",
		GlobalCfg:    &Config{DefaultTemplate: "global-default"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Ref != "my-template" {
		t.Errorf("Ref = %q, want my-template", ref.Ref)
	}
	if ref.Source != "--template flag" {
		t.Errorf("Source = %q, want --template flag", ref.Source)
	}
}

func TestResolveTemplate_ShortFlagBeforeEnv(t *testing.T) {
	t.Setenv("GIT_AUTOCOMMIT_TEMPLATE", "env-template")
	ref, err := ResolveTemplate(ResolveInput{FlagShort: true})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Ref != "short" {
		t.Errorf("Ref = %q, want short", ref.Ref)
	}
	if ref.Source != "--short flag" {
		t.Errorf("Source = %q, want --short flag", ref.Source)
	}
}

func TestResolveTemplate_EnvVar(t *testing.T) {
	t.Setenv("GIT_AUTOCOMMIT_TEMPLATE", "env-template")
	ref, err := ResolveTemplate(ResolveInput{
		GlobalCfg: &Config{DefaultTemplate: "global-default"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Ref != "env-template" {
		t.Errorf("Ref = %q, want env-template", ref.Ref)
	}
}

func TestResolveTemplate_RepoLocalConfig(t *testing.T) {
	t.Setenv("GIT_AUTOCOMMIT_TEMPLATE", "")
	dir := t.TempDir()

	// Write a .git-autocommit.toml in the dir.
	cfgContent := `template = "conventional"`
	if err := os.WriteFile(filepath.Join(dir, ".git-autocommit.toml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	ref, err := ResolveTemplate(ResolveInput{
		RepoRoot:  dir,
		Cwd:       dir,
		GlobalCfg: &Config{DefaultTemplate: "global-default"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Ref != "conventional" {
		t.Errorf("Ref = %q, want conventional", ref.Ref)
	}
	if ref.Source != ".git-autocommit.toml at "+dir {
		t.Errorf("Source = %q", ref.Source)
	}
}

func TestResolveTemplate_ProjectRule(t *testing.T) {
	t.Setenv("GIT_AUTOCOMMIT_TEMPLATE", "")

	ref, err := ResolveTemplate(ResolveInput{
		RemoteURL: "https://github.com/openclaw/myrepo.git",
		GlobalCfg: &Config{
			Projects: []ProjectRule{
				{MatchRemote: "github.com/openclaw/*", Template: "conventional"},
			},
			DefaultTemplate: "full",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Ref != "conventional" {
		t.Errorf("Ref = %q, want conventional", ref.Ref)
	}
	if ref.Source != "global config project rule #1" {
		t.Errorf("Source = %q", ref.Source)
	}
}

func TestResolveTemplate_GlobalDefault(t *testing.T) {
	t.Setenv("GIT_AUTOCOMMIT_TEMPLATE", "")

	ref, err := ResolveTemplate(ResolveInput{
		GlobalCfg: &Config{DefaultTemplate: "my-default"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Ref != "my-default" {
		t.Errorf("Ref = %q, want my-default", ref.Ref)
	}
	if ref.Source != "global config default_template" {
		t.Errorf("Source = %q", ref.Source)
	}
}

func TestResolveTemplate_BuiltinDefault(t *testing.T) {
	t.Setenv("GIT_AUTOCOMMIT_TEMPLATE", "")

	ref, err := ResolveTemplate(ResolveInput{GlobalCfg: &Config{}})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Ref != "full" {
		t.Errorf("Ref = %q, want full", ref.Ref)
	}
	if ref.Source != "built-in default (full)" {
		t.Errorf("Source = %q", ref.Source)
	}
}

func TestResolveTemplate_NilGlobalCfg(t *testing.T) {
	t.Setenv("GIT_AUTOCOMMIT_TEMPLATE", "")

	ref, err := ResolveTemplate(ResolveInput{GlobalCfg: nil})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Ref != "full" {
		t.Errorf("Ref = %q, want full", ref.Ref)
	}
}

// ---- repo config discovery ----

func TestFindRepoConfig_WalksUp(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	// Place the config at the root (not in the sub-dir).
	content := `template = "conventional"`
	if err := os.WriteFile(filepath.Join(root, ".git-autocommit.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, cfgPath, err := FindRepoConfig(root, sub)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected config to be found by walking up")
	}
	if cfg.Template != "conventional" {
		t.Errorf("Template = %q, want conventional", cfg.Template)
	}
	if filepath.Dir(cfgPath) != root {
		t.Errorf("cfgPath dir = %q, want %q", filepath.Dir(cfgPath), root)
	}
}

func TestFindRepoConfig_StopsAtRepoRoot(t *testing.T) {
	root := t.TempDir()
	above := filepath.Dir(root)

	// Place config ABOVE the repo root — should NOT be found.
	content := `template = "should-not-find"`
	if err := os.WriteFile(filepath.Join(above, ".git-autocommit.toml"), []byte(content), 0644); err != nil {
		// If we can't write there, skip the test.
		t.Skip("cannot write above temp dir")
	}
	t.Cleanup(func() { os.Remove(filepath.Join(above, ".git-autocommit.toml")) })

	cfg, _, err := FindRepoConfig(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Error("expected no config to be found when it is above repo root")
	}
}

func TestFindRepoConfig_NoConfig(t *testing.T) {
	dir := t.TempDir()

	cfg, path, err := FindRepoConfig(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil || path != "" {
		t.Errorf("expected nil cfg and empty path, got %v %q", cfg, path)
	}
}
