package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// TemplateRef is the result of template resolution: the name/path and where it came from.
type TemplateRef struct {
	Ref    string // template name or file path
	Source string // human-readable description for verbose logging
}

// ResolveInput contains all context needed to pick a template.
type ResolveInput struct {
	FlagTemplate string  // --template flag value (empty if not provided)
	FlagShort    bool    // --short flag
	GlobalCfg    *Config // loaded global config (may be nil)
	RepoRoot     string  // from git rev-parse --show-toplevel (empty if unavailable)
	Cwd          string  // current working directory
	RemoteURL    string  // raw origin remote URL (empty if unavailable)
}

// ResolveTemplate determines the template ref to use, applying the 7-level
// precedence chain defined in the spec.
func ResolveTemplate(in ResolveInput) (*TemplateRef, error) {
	// 1. --template flag
	if in.FlagTemplate != "" {
		return &TemplateRef{Ref: in.FlagTemplate, Source: "--template flag"}, nil
	}

	// 2. --short flag
	if in.FlagShort {
		return &TemplateRef{Ref: "short", Source: "--short flag"}, nil
	}

	// 3. GIT_AUTOCOMMIT_TEMPLATE env var
	if v := os.Getenv("GIT_AUTOCOMMIT_TEMPLATE"); v != "" {
		return &TemplateRef{Ref: v, Source: "GIT_AUTOCOMMIT_TEMPLATE env"}, nil
	}

	// 4. Repo-local .git-autocommit.toml
	if in.RepoRoot != "" && in.Cwd != "" {
		repoCfg, cfgPath, err := FindRepoConfig(in.RepoRoot, in.Cwd)
		if err != nil {
			return nil, err
		}
		if repoCfg != nil && repoCfg.Template != "" {
			ref := repoCfg.Template
			// Resolve relative paths against the config file's directory.
			if isRelativePath(ref) {
				ref = filepath.Join(filepath.Dir(cfgPath), ref)
			}
			return &TemplateRef{
				Ref:    ref,
				Source: fmt.Sprintf(".git-autocommit.toml at %s", filepath.Dir(cfgPath)),
			}, nil
		}
	}

	// 5 & 6. Global config rules and default
	if in.GlobalCfg != nil {
		normalized := NormalizeRemoteURL(in.RemoteURL)

		// 5. First matching [[project]] rule
		for i, rule := range in.GlobalCfg.Projects {
			if matchesRule(rule, in.RepoRoot, normalized) {
				return &TemplateRef{
					Ref:    rule.Template,
					Source: fmt.Sprintf("global config project rule #%d", i+1),
				}, nil
			}
		}

		// 6. Global config default_template
		if in.GlobalCfg.DefaultTemplate != "" {
			return &TemplateRef{
				Ref:    in.GlobalCfg.DefaultTemplate,
				Source: "global config default_template",
			}, nil
		}
	}

	// 7. Built-in default
	return &TemplateRef{Ref: "full", Source: "built-in default (full)"}, nil
}

// NormalizeRemoteURL strips scheme prefixes, SSH notation, and .git suffix so
// the result can be matched against a glob pattern.
//
//	git@github.com:foo/bar.git  → github.com/foo/bar
//	https://github.com/foo/bar.git → github.com/foo/bar
func NormalizeRemoteURL(raw string) string {
	s := raw

	if strings.HasPrefix(s, "git@") {
		// SSH format: git@host:path → host/path
		s = strings.TrimPrefix(s, "git@")
		s = strings.Replace(s, ":", "/", 1)
	} else {
		for _, pfx := range []string{"https://", "http://", "ssh://"} {
			if strings.HasPrefix(s, pfx) {
				s = strings.TrimPrefix(s, pfx)
				break
			}
		}
	}

	return strings.TrimSuffix(s, ".git")
}

// matchesRule reports whether a project rule matches the given repo root and
// normalized remote URL. If both match_path and match_remote are set, both
// must match (AND). At least one must be set; template must be non-empty.
func matchesRule(rule ProjectRule, repoRoot, normalizedRemote string) bool {
	if rule.Template == "" {
		return false
	}
	if rule.MatchPath == "" && rule.MatchRemote == "" {
		return false
	}

	if rule.MatchPath != "" {
		expanded := expandTilde(rule.MatchPath)
		matched, _ := doublestar.Match(expanded, repoRoot)
		if !matched {
			return false
		}
	}

	if rule.MatchRemote != "" {
		matched, _ := doublestar.Match(rule.MatchRemote, normalizedRemote)
		if !matched {
			return false
		}
	}

	return true
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}

// isRelativePath reports whether a template ref is a relative file path
// (has path separators or .tmpl suffix but is not absolute).
func isRelativePath(ref string) bool {
	return (strings.ContainsAny(ref, "/\\") || strings.HasSuffix(ref, ".tmpl")) &&
		!filepath.IsAbs(ref)
}
