# git-autocommit

A CLI tool that generates git commit messages using an AI provider, then performs the commit.

## How It Works

`git-autocommit` runs the relevant git command to obtain the diff, constructs a prompt with the diff inlined, sends it to a configured AI provider, parses the generated message from the output, and performs the appropriate git operation.

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

Create `~/.config/git-autocommit/config.toml`.

### Provider summary

| Provider | Requires | `binary` | `model` | `api_key` | `base_url` | `endpoint_type` |
|---|---|---|---|---|---|---|
| `claude` | [Claude Code](https://claude.ai/code) CLI | optional (`claude`) | optional | — | — | — |
| `anthropic` | Anthropic API key | — | required | required* | — | — |
| `openai` | OpenAI API key | — | required | required* | optional | — |
| `ollama` | [Ollama](https://ollama.com) running locally | — | required | — | optional | — |
| `opencode` | [opencode](https://opencode.ai) CLI | optional (`opencode`) | required | — | — | — |
| `opencode-go` | [OpenCode Go](https://opencode.ai/docs/go/) API key | — | required | required† | — | optional (`openai`) |
| `kiro` | [kiro-cli](https://kiro.dev/docs/cli/) | optional (`kiro-cli`) | optional | — | — | — |

\* Can be set via environment variable instead (`ANTHROPIC_API_KEY` / `OPENAI_API_KEY`).

† Can be set via the `OPENCODE_GO_API_KEY` environment variable.

### Provider configuration

#### `claude` — Claude CLI

Requires the [Claude Code](https://claude.ai/code) CLI on `$PATH`.

```toml
[provider]
type   = "claude"
binary = "claude"
model  = "sonnet"
```

| Field | Description |
|---|---|
| `type` | `claude` |
| `binary` | Path or name of the Claude CLI binary. |
| `model` | Model name or alias (e.g. `sonnet`, `opus`, `claude-sonnet-4-6`). |

#### `anthropic` — Anthropic API

Calls the Anthropic Messages API directly. Requires an API key.

```toml
[provider]
type  = "anthropic"
model = "claude-opus-4-6"
# api_key = "sk-ant-..."  # or set ANTHROPIC_API_KEY env var
```

| Field | Description |
|---|---|
| `type` | `anthropic` |
| `model` | Full model ID (e.g. `claude-opus-4-6`, `claude-sonnet-4-6`). |
| `api_key` | Anthropic API key. Falls back to the `ANTHROPIC_API_KEY` environment variable. |

#### `openai` — OpenAI-compatible API

Calls the OpenAI chat completions API (or any compatible provider). Requires an API key.

```toml
[provider]
type  = "openai"
model = "gpt-4o"
# api_key  = "sk-..."  # or set OPENAI_API_KEY env var
# base_url = "..."     # defaults to https://api.openai.com/v1/chat/completions
```

| Field | Description |
|---|---|
| `type` | `openai` |
| `model` | Model ID (e.g. `gpt-4o`, `gpt-4-turbo`). |
| `api_key` | API key. Falls back to the `OPENAI_API_KEY` environment variable. |
| `base_url` | API endpoint. Defaults to `https://api.openai.com/v1/chat/completions`. Override to use any OpenAI-compatible provider. |

**Compatible providers via `base_url`:**

| Provider | `base_url` | `model` example |
|---|---|---|
| [OpenRouter](https://openrouter.ai) | `https://openrouter.ai/api/v1/chat/completions` | `anthropic/claude-sonnet-4-5` |
| [Mistral](https://mistral.ai) | `https://api.mistral.ai/v1/chat/completions` | `mistral-large-latest` |
| [Groq](https://groq.com) | `https://api.groq.com/openai/v1/chat/completions` | `llama-3.3-70b-versatile` |
| [xAI (Grok)](https://x.ai) | `https://api.x.ai/v1/chat/completions` | `grok-3` |
| [DeepSeek](https://deepseek.com) | `https://api.deepseek.com/v1/chat/completions` | `deepseek-chat` |
| [Together AI](https://together.ai) | `https://api.together.xyz/v1/chat/completions` | `meta-llama/Llama-3-70b-chat-hf` |
| [AWS Bedrock](https://aws.amazon.com/bedrock/) | `https://bedrock-runtime.{region}.amazonaws.com/openai/v1/chat/completions` | `amazon.nova-pro-v1:0` |
| [LM Studio](https://lmstudio.ai) (local) | `http://localhost:1234/v1/chat/completions` | _(set in LM Studio)_ |

Example using OpenRouter:

```toml
[provider]
type     = "openai"
model    = "anthropic/claude-sonnet-4-5"
base_url = "https://openrouter.ai/api/v1/chat/completions"
# api_key = "sk-or-..."  # or set OPENAI_API_KEY env var
```

Example using AWS Bedrock (requires a [Bedrock API key](https://docs.aws.amazon.com/bedrock/latest/userguide/api-keys.html), not an AWS access key):

```toml
[provider]
type     = "openai"
model    = "amazon.nova-pro-v1:0"
base_url = "https://bedrock-runtime.us-east-1.amazonaws.com/openai/v1/chat/completions"
# api_key = "..."  # Bedrock API key
```

#### `ollama` — Ollama (local or network)

Calls a locally running [Ollama](https://ollama.com) instance. No API key required.

```toml
[provider]
type  = "ollama"
model = "qwen2.5-coder:3b"
# base_url = "http://localhost:11434/v1/chat/completions"  # default
```

| Field | Description |
|---|---|
| `type` | `ollama` |
| `model` | Any model you have pulled locally (e.g. `qwen2.5-coder:3b`, `llama3.2`, `mistral`). |
| `base_url` | Defaults to `http://localhost:11434/v1/chat/completions`. |

**Prerequisites:** Install Ollama (`brew install ollama`), pull a model (`ollama pull qwen2.5-coder:3b`), and ensure the daemon is running (`ollama serve`).

**Using Ollama on another machine on your local network:**

```toml
[provider]
type     = "ollama"
model    = "qwen2.5-coder:3b"
base_url = "http://192.168.1.50:11434/v1/chat/completions"
```

The remote machine must run Ollama with `OLLAMA_HOST=0.0.0.0 ollama serve` to accept connections from the network.

#### `opencode` — OpenCode CLI

Uses the [opencode](https://opencode.ai) CLI. Supports any provider/model that opencode is configured for.

```toml
[provider]
type   = "opencode"
model  = "anthropic/claude-opus-4-6"
# binary = "opencode"  # default
```

| Field | Description |
|---|---|
| `type` | `opencode` |
| `model` | Provider and model in `provider/model` format (e.g. `anthropic/claude-opus-4-6`, `openai/gpt-4o`). |
| `binary` | Path to the opencode binary. Defaults to `opencode` (must be on `$PATH`). |

#### `opencode-go` — OpenCode Go API

Calls the [OpenCode Go](https://opencode.ai/docs/go/) API directly. Requires an API key (subscribe at [opencode.ai](https://opencode.ai/auth)).

```toml
[provider]
type          = "opencode-go"
model         = "kimi-k2.5"
# api_key     = "..."  # or set OPENCODE_GO_API_KEY env var
# endpoint_type = "openai"  # default; use "anthropic" for minimax models
```

| Field | Description |
|---|---|
| `type` | `opencode-go` |
| `model` | Model ID (e.g. `kimi-k2.5`, `glm-5.1`, `mimo-v2-pro`, `minimax-m2.7`). |
| `api_key` | OpenCode Go API key. Falls back to the `OPENCODE_GO_API_KEY` environment variable. |
| `endpoint_type` | `openai` (default) for most models; `anthropic` for MiniMax models (`minimax-m2.5`, `minimax-m2.7`). |

Available models and their required endpoint type:

| Model | `endpoint_type` |
|---|---|
| `kimi-k2.5` | `openai` |
| `glm-5` | `openai` |
| `glm-5.1` | `openai` |
| `mimo-v2-pro` | `openai` |
| `mimo-v2-omni` | `openai` |
| `minimax-m2.5` | `anthropic` |
| `minimax-m2.7` | `anthropic` |

#### `kiro` — Kiro CLI

Uses the [kiro-cli](https://kiro.dev/docs/cli/).

```toml
[provider]
type = "kiro"
# binary = "kiro-cli"  # default
# model = "claude-sonnet-4-5"  # optional; omit to use kiro's configured default
```

| Field | Description |
|---|---|
| `type` | `kiro` |
| `binary` | Path to the kiro-cli binary. Defaults to `kiro-cli` (must be on `$PATH`). |
| `model` | Model to pass via `--model`. Optional — omit to use kiro's configured default. |

### Environment variables

Environment variables override config file values:

| Variable | Description |
|---|---|
| `GIT_AUTOCOMMIT_PROVIDER_TYPE` | Provider type |
| `GIT_AUTOCOMMIT_PROVIDER_BINARY` | Provider binary (`claude` type only) |
| `GIT_AUTOCOMMIT_MODEL` | Model name/ID |
| `GIT_AUTOCOMMIT_TEMPLATE` | Template name or path (overridden by `--template` / `--short`) |
| `ANTHROPIC_API_KEY` | API key for the `anthropic` provider type |
| `OPENAI_API_KEY` | API key for the `openai` provider type |
| `OPENCODE_GO_API_KEY` | API key for the `opencode-go` provider type |

## Usage

```
git-autocommit [options]
```

### Options

| Option | Description |
|---|---|
| `--template <name-or-path>` | Use the named template or a path to a `.tmpl` file |
| `--short` | Shorthand for `--template short` (single-line message) |
| `--skip` | Skip the confirmation prompt and commit immediately |
| `--squash VALUE` | Squash commits (see below) |
| `--rewrite [REF]` | Rewrite an existing commit's message (see below) |
| `--context VALUE` | Provide additional context to the AI (file path, URL, or plain string) |
| `-v`, `--verbose` | Log the resolved template and its source before generating |
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

## Commit message templates

`git-autocommit` uses template files to control the shape of the generated commit message. Two built-in templates are included; you can also write your own.

### Built-in templates

| Name | Description |
|---|---|
| `full` | Multi-section message with subject, body, Problem, Test, and Ticket sections. Used by default. |
| `short` | Single-line message, 72 characters max. |

```sh
git-autocommit                           # uses full template (default)
git-autocommit --short                   # uses short template
git-autocommit --template conventional  # uses a named user template
git-autocommit --template ./my.tmpl     # loads a template file directly
```

### Template files

A template is a `.tmpl` file with optional YAML frontmatter:

```
---
name: conventional
description: Conventional Commits format
max_subject_length: 72
---
<type(scope): subject, {{ .MaxSubjectLength }} chars max>

<body>

<BREAKING CHANGE: description, if any>
```

Frontmatter fields:

| Field | Default | Purpose |
|---|---|---|
| `name` | filename without `.tmpl` | Identifier for lookup |
| `description` | `""` | Shown in `templates list` |
| `max_subject_length` | `80` | Exposed as `{{ .MaxSubjectLength }}` in the body |

Place custom templates in `~/.config/git-autocommit/templates/`. A user file with the same name as a built-in overrides it.

### Listing and inspecting templates

```sh
git-autocommit templates list          # list all available templates
git-autocommit templates show full     # print a template's contents
```

### Template resolution order

The template is resolved using this precedence (most specific wins):

1. `--template <name-or-path>` CLI flag
2. `--short` CLI flag
3. `GIT_AUTOCOMMIT_TEMPLATE` environment variable
4. Repo-local `.git-autocommit.toml` `template` field
5. Global config `[[project]]` rule matching the current repo
6. Global config `default_template`
7. Built-in `full`

### Repo-local config (`.git-autocommit.toml`)

Place a `.git-autocommit.toml` at your repo root (or anywhere between cwd and the root) to pin a template for that repo:

```toml
template = "conventional"
```

### Global config (`~/.config/git-autocommit/config.toml`)

Add `default_template` and `[[project]]` rules to the global config:

```toml
default_template = "full"

[[project]]
match_remote = "github.com/myorg/*"
template = "conventional"

[[project]]
match_path = "~/work/acme/**"
template = "acme-jira"
```

Each `[[project]]` entry can match by `match_path` (glob against repo root absolute path, supports `~` and `**`) and/or `match_remote` (glob against the normalized `origin` remote URL). If both are set, both must match. Rules are evaluated in order; first match wins.

## Additional context (`--context VALUE`)

Provides supplementary information to the AI. The type is auto-detected:

| Value | Detection | Behaviour |
|---|---|---|
| File path | File exists on disk | File contents are included in the prompt |
| URL | Starts with `http://` or `https://` | URL is fetched and the response body is included in the prompt |
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

## Releasing

Releases are automated via GoReleaser and GitHub Actions. On a pushed `v*` tag, the workflow builds binaries for `darwin/amd64` and `darwin/arm64`, publishes a GitHub Release, and updates the Homebrew formula in `webbcam/homebrew-tap`.

```sh
git tag v0.x.x
git push origin v0.x.x
```

## Prerequisites

- `git` on `$PATH`
- A configured AI provider (see [Configuration](#configuration))
