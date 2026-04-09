# git-autocommit

A CLI tool that generates git commit messages using an AI agent, then performs the commit.

## How It Works

`git-autocommit` constructs a prompt describing the diff to examine and the desired commit message format, sends it to a configured AI agent (e.g. the Claude CLI), parses the generated message from the output, and performs the appropriate git operation.

## Installation

### Homebrew

```sh
brew install webbcam/tap/git-autocommit
```

### Build from source

Requires Go 1.21+.

```sh
git clone https://github.com/webbcam/git-autocommit
cd git-autocommit
go build -o git-autocommit .
mv git-autocommit /usr/local/bin/
```

## Configuration

Configuration is required — either a config file, environment variables, or both.

### Config file

Create `~/.config/git-autocommit/config.toml`:

```toml
[agent]
type   = "claude"
binary = "claude"
model  = "sonnet"
```

| Field | Description |
|---|---|
| `type` | Agent type. Currently only `claude` is supported. |
| `binary` | Path or name of the agent binary (must be on `$PATH` or an absolute path). |
| `model` | Model name or alias passed to the agent (e.g. `sonnet`, `opus`, `claude-sonnet-4-6`). |

### Environment variables

Environment variables override config file values:

| Variable | Description |
|---|---|
| `GIT_AUTOCOMMIT_AGENT_TYPE` | Agent type |
| `GIT_AUTOCOMMIT_AGENT_BINARY` | Agent binary |
| `GIT_AUTOCOMMIT_MODEL` | Model name/alias |

## Usage

```
git-autocommit [options]
```

### Options

| Option | Description |
|---|---|
| `--formal` | Use the formal multi-section commit message template (default) |
| `--informal` | Use a single-line commit message (max 72 characters) |
| `--skip` | Skip the confirmation prompt and commit immediately |
| `--squash VALUE` | Squash commits (see below) |
| `--rewrite [REF]` | Rewrite an existing commit's message (see below) |
| `--context VALUE` | Provide additional context to the AI (file path, URL, or plain string) |
| `-h`, `--help` | Print usage |

## Modes

### Standard commit (default)

Generates a message for currently staged changes and commits them.

```sh
git add .
git-autocommit
```

Requires staged changes — exits with an error if nothing is staged.

### Squash (`--squash VALUE`)

Squashes multiple commits into one with a new AI-generated message.

| Value | Behaviour |
|---|---|
| Integer N (≥ 2) | Squash the last N commits |
| Single ref (SHA, tag, etc.) | Squash from that ref to HEAD |
| `REF1..REF2` | Squash a range via interactive rebase |

```sh
git-autocommit --squash 3              # squash last 3 commits
git-autocommit --squash abc123         # squash from abc123 to HEAD
git-autocommit --squash abc123..HEAD   # squash range via rebase
```

### Rewrite (`--rewrite [REF]`)

Regenerates the commit message for an existing commit without changing its content.

```sh
git-autocommit --rewrite               # amend the last commit's message
git-autocommit --rewrite HEAD~2        # rewrite a specific commit via rebase
git-autocommit --rewrite abc123        # rewrite by SHA
```

When rewriting a non-HEAD commit, any uncommitted changes are stashed before the rebase and restored after.

## Message styles

### Formal (default)

A structured, multi-section message:

```
<subject line, 80 chars max>

<body: description of the solution/change>

[Problem]
<why this change is needed>

[Test]
<how it was tested or should be tested>

[Ticket]
<ticket URL — only included when available from context>
```

### Informal (`--informal`)

A single-line message, maximum 72 characters.

```sh
git-autocommit --informal
```

## Additional context (`--context VALUE`)

Provides supplementary information to the AI. The type is auto-detected:

| Value | Detection | Behaviour |
|---|---|---|
| File path | File exists on disk | AI reads the file |
| URL | Starts with `http://` or `https://` | AI fetches the URL |
| Plain string | Everything else | Included directly in the prompt |

```sh
git-autocommit --context ./TICKET-123.md
git-autocommit --context https://linear.app/team/issue/ENG-456
git-autocommit --context "Part of the auth refactor — keep the message focused on session handling"
```

## Confirmation flow

By default, the tool displays the proposed message and asks for confirmation before committing:

```
--- Proposed commit message ---
Fix session token expiry not being checked on refresh

The token refresh endpoint was not validating expiry before issuing a
new token, allowing expired sessions to be silently extended.

[Problem]
Expired sessions could be refreshed indefinitely, bypassing the
intended session lifetime limit.

[Test]
Added unit tests for the token validation middleware. Verified manually
with an expired token that the endpoint now returns 401.
-------------------------------
Commit with this message? [y/N]
```

Pass `--skip` to bypass the prompt and commit immediately.

## Prerequisites

- `git` on `$PATH`
- A supported AI agent CLI on `$PATH` (currently: `claude` from [Claude Code](https://claude.ai/code))
