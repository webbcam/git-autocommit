# Proposal: Rename `git-ai-commit` to `git-autocommit`

## Summary

Rename the project from `git-ai-commit` to `git-autocommit` across all code, configuration, and documentation.

## Motivation

`git-autocommit` is shorter, more natural to type, and less dependent on the word "AI" — keeping the name stable if the underlying mechanism changes (e.g. local LLM, templates). It also reads cleanly as a git subcommand: `git autocommit`.

## Changes Required

### 1. Binary name

The compiled binary is renamed from `git-ai-commit` to `git-autocommit`.

- All documentation and usage examples updated accordingly.

### 2. Go module path

`go.mod`:

```
module github.com/webbcam/git-autocommit
```

All internal imports updated from `github.com/webbcam/git-ai-commit/internal/...` to `github.com/webbcam/git-autocommit/internal/...`.

Files affected:
- `go.mod`
- `main.go`

### 3. Config directory

The default config path changes:

| Before | After |
|---|---|
| `~/.config/git-ai-commit/config.toml` | `~/.config/git-autocommit/config.toml` |

File affected: `internal/config/config.go`

No automatic migration of existing config files — users must move the file manually. The error message shown when no config is found will reference the new path.

### 4. Environment variables

| Before | After |
|---|---|
| `GIT_AI_COMMIT_AGENT_TYPE` | `GIT_AUTOCOMMIT_AGENT_TYPE` |
| `GIT_AI_COMMIT_AGENT_BINARY` | `GIT_AUTOCOMMIT_AGENT_BINARY` |
| `GIT_AI_COMMIT_MODEL` | `GIT_AUTOCOMMIT_MODEL` |

File affected: `internal/config/config.go`

### 5. Usage text and error messages

The usage text in `main.go` references `git-ai-commit` in the header and examples. All occurrences updated to `git-autocommit`.

### 6. Temp file prefix

`internal/git/git.go` uses `git-ai-commit-*` as the prefix for temporary files created during rebase. Updated to `git-autocommit-*`.

### 7. Documentation

- `README.md` — all references to `git-ai-commit` updated
- `SPEC.md` — update binary name in examples and references
- `HOMEBREW.md` — update formula name and binary reference
- Proposal documents — no changes needed (historical)

## Migration Notes for Existing Users

1. Move config file:
   ```
   mv ~/.config/git-ai-commit/config.toml ~/.config/git-autocommit/config.toml
   ```
2. Update any shell aliases or scripts that reference `git-ai-commit`.
3. Update any environment variables if set (e.g. in `.zshrc`, `.bashrc`).

## Out of Scope

- Renaming the GitHub repository (separate decision).
- Changes to config file format or environment variable values.
- Any functional behaviour changes.
