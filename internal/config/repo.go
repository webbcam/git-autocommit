package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// RepoConfig holds the content of a repo-local .git-autocommit.toml file.
type RepoConfig struct {
	Template string `toml:"template"`
}

// FindRepoConfig searches for .git-autocommit.toml starting from startDir and
// walking up to repoRoot. Returns the parsed config and its path on success,
// or nil/"" if no config file is found.
func FindRepoConfig(repoRoot, startDir string) (*RepoConfig, string, error) {
	dir := startDir
	for {
		p := filepath.Join(dir, ".git-autocommit.toml")
		if _, err := os.Stat(p); err == nil {
			var cfg RepoConfig
			if _, err := toml.DecodeFile(p, &cfg); err != nil {
				return nil, "", fmt.Errorf("failed to parse %s: %w", p, err)
			}
			return &cfg, p, nil
		}
		if filepath.Clean(dir) == filepath.Clean(repoRoot) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, "", nil
}
