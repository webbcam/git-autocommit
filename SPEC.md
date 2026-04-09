# git-autocommit — Specification

A CLI tool that generates git commit messages using an AI agent, then performs the commit.

## Prerequisites

- `git` — the tool operates inside a git repository
- A supported AI agent (see Agent Configuration below)

## Core Concept

The tool runs the appropriate git command itself to obtain the diff, constructs a prompt containing the diff output and the desired commit message format, sends it to a configured AI agent, parses the generated message from the output using delimiters, and then performs the appropriate git operation (commit, amend, squash, or rewrite).

The AI agent is only responsible for reading the prompt and generating text — it does not need shell access or tool use capabilities.

## Modes of Operation

The tool has four mutually exclusive modes. If none of `--squash`, `--rewrite` are specified, it defaults to **standard commit** mode.

### 1. Standard Commit (default)

Commits currently staged changes (`git diff --cached`).

- Precondition: there must be staged changes, otherwise exit with an error
- The tool runs `git diff --cached` and includes the output in the prompt

### 2. Squash (`--squash VALUE`)

Squashes multiple commits into one with a new AI-generated message. The argument determines what to squash:

| Argument Format | Behavior |
|---|---|
| Integer N (N ≥ 2) | Squash the last N commits. Uses `git reset --soft HEAD~N` then commits. |
| Single ref (SHA, tag, etc.) | Squash from that ref to HEAD (inclusive). Uses `git reset --soft REF~1` then commits. |
| `REF1..REF2` | Squash a range within history via interactive rebase. REF1 must be an ancestor of REF2. The first commit in the range is reworded, the rest are fixup'd. |

- Validation: for integer N, must be ≥ 2 (error otherwise, suggest `--rewrite`). For refs, validate they exist via `git rev-parse`. For ranges, verify ancestry.
- The tool runs the appropriate `git diff` spanning the squash range and includes the output in the prompt

### 3. Rewrite (`--rewrite [REF]`)

Regenerates the commit message for an existing commit without changing its content.

| Argument | Behavior |
|---|---|
| (none) | Amend the last commit's message via `git commit --amend -m` |
| A ref (SHA, HEAD~N, etc.) | Rewrite a specific commit's message via interactive rebase (reword) |

- When rewriting a non-HEAD commit: stash any uncommitted changes before rebasing, restore after
- The tool runs `git diff HEAD~1 HEAD` (last commit) or `git show <SHA> --format= -p` (specific commit) and includes the output in the prompt

## Message Styles

Controlled by `--formal` (default) and `--informal`. These are combinable with any mode.

### Formal (default)

A multi-section commit message following this template:

```
<subject line, 80 chars max>

<body: description of the solution/change>

[Problem]
<why this change is needed>

[Test]
<how it was tested>

[Ticket]
<ticket URL>
```

The AI should be given an example of this format in the prompt. The `[Ticket]` field should default to the placeholder `JIRA-XXXXX`.

### Informal

A single-line commit message, max 72 characters.

## Arguments

| Argument | Required | Description |
|---|---|---|
| `--formal` | No | Use the formal multi-section template (default) |
| `--informal` | No | Use a single-line message (max 72 chars) |
| `--skip` | No | Skip the confirmation prompt and commit immediately |
| `--squash VALUE` | No | Squash commits. VALUE is an integer N, a single ref, or `REF1..REF2` |
| `--rewrite [REF]` | No | Rewrite an existing commit's message. Optional REF targets a specific commit |
| `--context VALUE` | No | Additional context for the AI. VALUE is a file path, URL, or plain string |
| `-h`, `--help` | No | Print usage and exit |

All flags are combinable (e.g., `--squash 3 --informal --skip`).

Unknown options should print an error and show usage.

## Additional Context (`--context VALUE`)

Provides supplementary information to the AI when generating the message. The type is auto-detected and resolved by the tool before the prompt is sent:

| VALUE type | Detection | Behaviour |
|---|---|---|
| File path | File exists on disk | Tool reads the file and includes its contents in the prompt |
| URL | Starts with `http://` or `https://` | Tool fetches the URL and includes the response body in the prompt |
| Plain string | Everything else | Included directly in the prompt as-is |

This is appended to the prompt after the main instructions.

## Confirmation Flow

By default, the tool displays the proposed message and asks for confirmation:

```
--- Proposed commit message ---
<message>
-------------------------------
Commit with this message? [y/N]
```

- `y` or `Y` → proceed with the commit
- Anything else → abort with exit code 1

When `--skip` is passed, the commit happens immediately with no prompt.

## AI Agent Interaction

### Agent Configuration

The AI agent requires no special capabilities — it only needs to receive a text prompt and return a text response. No shell access, tool use, or external API calls are required from the agent.

Supported agent types are defined in the config file (see Configuration).

### Prompt Construction

1. Run the appropriate git command (`git diff --cached`, `git show <SHA>`, etc.) and capture the output
2. If `--context` is a file path, read the file contents; if a URL, fetch the response body
3. Build a prompt containing:
   - The diff output inline
   - The commit message style instructions and template
   - Any additional context (file contents, URL body, or plain string)
   - Instructions to wrap the output between delimiters (`===COMMIT_MSG_START===` and `===COMMIT_MSG_END===`) and output nothing else
4. Send the prompt to the AI agent

### Response Parsing

1. Strip ANSI escape codes from the raw output
2. Extract text between the start and end delimiters
3. If no message is found between delimiters, exit with an error

## Error Conditions

| Condition | Behavior |
|---|---|
| No staged changes (standard mode) | Error: "No staged changes found. Stage changes with 'git add' first." |
| `--squash` with no argument | Error: "--squash requires an argument." |
| `--squash N` where N < 2 | Error: suggest using `--rewrite` instead |
| Invalid git ref | Error: "invalid ref '<ref>'" |
| Range where REF1 is not ancestor of REF2 | Error with explanation |
| AI fails to produce a message | Error: "Failed to generate commit message." |
| `--context` with no argument | Error: "--context requires a value." |
| `--context` URL fetch fails | Error: "Failed to fetch context URL: <url>" |
| Unknown option | Error + print usage |

All errors exit with code 1.

## Exit Codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Error or user aborted |
