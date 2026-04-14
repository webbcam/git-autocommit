package tmpl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Load loads a template by name or file path, using the resolution order:
//  1. If ref looks like a path (contains separators or ends in .tmpl), load from disk.
//  2. Check ~/.config/git-autocommit/templates/<name>.tmpl (user override).
//  3. Check embedded built-ins.
func Load(ref string) (*Template, error) {
	if looksLikePath(ref) {
		return loadFromFile(ref)
	}
	return loadByName(ref)
}

// LoadRaw returns the raw (un-rendered) content of a template by name or path,
// using the same lookup order as Load.
func LoadRaw(ref string) (string, error) {
	if looksLikePath(ref) {
		b, err := os.ReadFile(ref)
		if err != nil {
			return "", fmt.Errorf("loading template from %s: %w", ref, err)
		}
		return string(b), nil
	}

	if dir, err := UserTemplateDir(); err == nil {
		p := filepath.Join(dir, ref+".tmpl")
		if _, err := os.Stat(p); err == nil {
			b, err := os.ReadFile(p)
			if err != nil {
				return "", fmt.Errorf("loading template from %s: %w", p, err)
			}
			return string(b), nil
		}
	}

	if content, ok := embeddedTemplates[ref]; ok {
		return content, nil
	}

	return "", fmt.Errorf("template %q not found", ref)
}

// UserTemplates returns all templates found in the user template directory,
// sorted by name. Returns nil (not an error) if the directory doesn't exist.
func UserTemplates() ([]*Template, error) {
	dir, err := UserTemplateDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var result []*Template
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tmpl") {
			continue
		}
		t, err := loadFromFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // skip malformed templates
		}
		result = append(result, t)
	}
	return result, nil
}

// UserTemplateDir returns the path to the user's template directory,
// respecting $XDG_CONFIG_HOME.
func UserTemplateDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "git-autocommit", "templates"), nil
}

// looksLikePath reports whether ref should be treated as a file path.
func looksLikePath(ref string) bool {
	return strings.ContainsAny(ref, "/\\") || strings.HasSuffix(ref, ".tmpl")
}

// loadFromFile reads and parses a template from disk.
func loadFromFile(path string) (*Template, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading template from %s: %w", path, err)
	}
	sourceName := strings.TrimSuffix(filepath.Base(path), ".tmpl")
	return Parse(string(b), sourceName)
}

// loadByName resolves a bare name: user dir first, then embedded built-ins.
func loadByName(name string) (*Template, error) {
	if dir, err := UserTemplateDir(); err == nil {
		p := filepath.Join(dir, name+".tmpl")
		if _, err := os.Stat(p); err == nil {
			return loadFromFile(p)
		}
	}

	if content, ok := embeddedTemplates[name]; ok {
		return Parse(content, name)
	}

	return nil, fmt.Errorf("template %q not found", name)
}
