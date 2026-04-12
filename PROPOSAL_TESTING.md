# Proposal: Unit Testing Strategy for git-autocommit

## What We're Working With

The codebase has five distinct layers, each with different testing needs:

| Layer | Package | Testability |
|---|---|---|
| Arg parsing | `main` — `parseArgs` | Pure function, trivial to test |
| Prompt builder | `internal/prompt` | Mostly pure, trivial to test |
| Output parser | `internal/parser` | Pure function, trivial to test |
| Config loader | `internal/config` | Pure with env/file I/O |
| Git operations | `internal/git` | Calls `exec.Command("git", ...)` |
| AI agent | `internal/agent` | Calls external binary or LLM |
| Orchestration | `main` — `run`, `handleStandard`, etc. | Calls git + agent |

The pure layers are straightforward. The interesting design question is how to handle the git-calling and agent-calling code.

---

## Option A: Unit tests for pure code only

Test only the stateless, side-effect-free functions:

- `parseArgs` — all flag combinations, unknown flags, missing args
- `git.ParseSquashValue` — integer, single ref, range, edge cases
- `parser.ExtractCommitMessage` — delimiter detection, ANSI stripping, empty message, missing delimiters
- `prompt.Build` / `prompt.BuildWithDiff` — output contains expected substrings for each style/context combo
- `prompt.DetectContextType` — URL prefix detection, file existence check, plain string fallback
- `config.Load` — env var overrides, default values, missing-everything error

**Tradeoffs:**
- No setup complexity. Fast, hermetic, zero external dependencies.
- Leaves the most important behaviour untested: the git operations and the wiring in `handleStandard`/`handleSquash`/`handleRewrite`.
- Gives false confidence — the pure code is not where bugs hide in a git tool.

---

## Option B: Mock the git layer with an interface

Extract a `GitRunner` interface (or pass functions as dependencies) so that git operations can be swapped for fakes in tests:

```go
type GitRunner interface {
    HasStagedChanges() (bool, error)
    Commit(msg string) error
    SoftReset(ref string) error
    // ...
}
```

`handleStandard`, `handleSquash`, etc. receive a `GitRunner` instead of calling the `git` package directly. Tests inject a fake implementation.

**Tradeoffs:**
- Tests are fast and hermetic — no filesystem, no real git process.
- The interface and fakes add meaningful boilerplate to the production code.
- Mocks can and do diverge from real git behaviour. The rebase and squash operations are complex; a mock that returns `nil` tells you nothing about whether the git commands actually work.
- This is a known failure mode — tests pass, production breaks — especially for error paths in the git operations.

**Verdict:** Not recommended as the primary strategy. The interface abstraction cost is high and the confidence gain is low for the complex operations.

---

## Option C: Integration tests with a real temp git repo

Create a real git repository in a temp directory for each test, perform setup commits, then run the actual git functions against it:

```go
func TestSoftReset(t *testing.T) {
    dir := initTestRepo(t) // git init + git config user + initial commit
    // ... make commits, stage files ...
    err := git.SoftReset("HEAD~1")
    // ... assert
}
```

Go's `testing.T.TempDir()` handles cleanup automatically. The tests run the real `git` binary, so they test actual git behaviour including error cases, rebase edge cases, and stash interactions.

This is the approach used by `go-git`, `gh`, `lazygit`, `git-bug`, and most Go git tooling.

**Tradeoffs:**
- Tests reflect real git behaviour — if the rebase logic works in tests, it works in production.
- Requires `git` on the test machine's PATH. This is a reasonable assumption for a tool that requires git to function at all.
- Tests are slower than pure unit tests, but still fast (milliseconds per test — no network, no AI).
- Slightly more setup code per test (`git init`, config, staging files). Extractable into a `testhelper` package.
- Some operations (interactive rebase with `GIT_SEQUENCE_EDITOR`) may behave differently on CI depending on git version. Mitigated by pinning git version in CI.

**Verdict:** This is the right approach for the git layer.

---

## Option D: Stub the Agent interface for orchestration tests

The `agent.Agent` interface already exists. A stub implementation requires three lines:

```go
type stubAgent struct{ response string }
func (s *stubAgent) Generate(prompt string, _, _ bool) (string, error) {
    return s.response, nil
}
```

Combined with a real temp git repo, this lets us test the full `run()` pipeline end-to-end without needing the Claude CLI or a local LLM. The stub returns a canned response containing the delimiters; the real parser extracts it; the real git commit runs against the real temp repo.

**Tradeoffs:**
- Tests the entire pipeline including wiring, config, prompt construction, parsing, and committing.
- The only thing not tested is the AI output quality, which is not a unit test concern.
- Requires minor refactoring of `run()` to accept injected dependencies rather than hard-coding `os.Args` and `agent.NewClaudeAgent`. See refactoring notes below.

---

## Recommended Approach

Combine **Option A + C + D**:

### 1. Pure unit tests (no setup required)

Test all pure/near-pure functions in their own `_test.go` files in the same package:

- `parseArgs` — every flag, combinations, error cases
- `git.ParseSquashValue` — all three formats, edge cases (N=1, empty range, etc.)
- `parser.ExtractCommitMessage` — happy path, ANSI stripping, missing start delimiter, missing end delimiter, empty content between delimiters
- `prompt.Build` / `prompt.BuildWithDiff` — correct template emitted for formal/informal, context sections present/absent
- `prompt.DetectContextType` — URL prefix, existing file (using `t.TempDir`), plain string
- `config.Load` — env override priority, defaults, error when all empty

### 2. Git integration tests (real temp repo)

A `testhelper` package (or `internal/git/testhelper_test.go`) provides:

```go
func InitRepo(t *testing.T) string          // git init, set user.name/email, return dir
func MakeCommit(t *testing.T, dir, msg string) string  // stage a file, commit, return SHA
func StageFile(t *testing.T, dir, name, content string) // write + git add
```

Tests for `internal/git`:

- `HasStagedChanges` — with and without staged files
- `SoftReset` — verify commits are collapsed correctly
- `Commit` — verify commit appears in log with correct message
- `AmendCommit` — verify HEAD message changes, commit count stays the same
- `ListCommitsInRange` — verify correct SHAs returned
- `RebaseRewordCommit` — verify commit message changed, SHA changed, content unchanged
- `RebaseSquashRange` — verify commit count reduced, message updated, commits after range preserved

### 3. Orchestration integration tests (stub agent + real repo)

Minor refactor to `run()` to accept deps:

```go
type runDeps struct {
    args      []string
    stdin     io.Reader
    stderr    io.Writer
    newAgent  func(cfg *config.Config) (agent.Agent, bool)
    loadConfig func() (*config.Config, error)
}

func runWithDeps(deps runDeps) error { ... }
```

This is a small change — `run()` becomes a one-liner that calls `runWithDeps` with real OS values. Tests pass a `stubAgent` and a temp-dir-scoped config.

Orchestration tests cover:
- Standard commit: staged changes → stub agent → commit appears in log
- `--squash 3`: 3 commits → squashed to 1 with stub message
- `--rewrite`: HEAD message updated
- `--skip`: commit happens without stdin interaction
- `--informal`: prompt sent to agent contains informal instructions (assert on stub's received prompt)
- Error paths: no staged changes, invalid ref, agent returns no delimiters

### 4. What we do NOT test

- Actual AI output quality — not a unit test concern
- End-to-end invocation of the real Claude CLI or local LLM — these belong in a manual or integration test suite run separately
- Platform-specific git edge cases beyond what git itself handles

---

## File Layout

```
internal/
  git/
    git.go
    git_test.go          # integration tests for git operations
    testhelper_test.go   # InitRepo, MakeCommit, StageFile helpers (test-only)
  parser/
    parser_test.go       # pure unit tests
  prompt/
    prompt_test.go       # pure unit tests
  config/
    config_test.go       # pure unit tests with env/file manipulation
  agent/
    agent.go
    stub_test.go         # stubAgent for use in orchestration tests
main_test.go             # orchestration tests using runWithDeps + stubAgent
parseargs_test.go        # pure unit tests for parseArgs
```

---

## Minimal Refactoring Required

The only code change needed to enable all of the above:

1. **`run()`** — accept a `runDeps` struct (or individual function parameters) so tests can inject a stub agent and a pre-loaded config pointing at the temp repo's git dir. All existing behaviour is preserved; `run()` just delegates to `runWithDeps` with the real values.

2. **`internal/git`** functions** — no changes needed. The existing functions already work against whatever repo is the current working directory; tests just `os.Chdir` to the temp repo.

No interface extraction needed for git itself. No mocks. The git functions are tested against real git.
