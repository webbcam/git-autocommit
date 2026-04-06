package prompt

import (
	"fmt"
	"os"
	"strings"
)

// Style represents the commit message style.
type Style int

const (
	StyleFormal  Style = iota
	StyleInformal Style = iota
)

// ContextType represents the type of additional context.
type ContextType int

const (
	ContextNone    ContextType = iota
	ContextFile    ContextType = iota
	ContextURL     ContextType = iota
	ContextString  ContextType = iota
)

// Context holds the additional context information.
type Context struct {
	Type  ContextType
	Value string
}

// DetectContextType determines the type of context from the value string.
func DetectContextType(value string) Context {
	if value == "" {
		return Context{Type: ContextNone}
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return Context{Type: ContextURL, Value: value}
	}
	if _, err := os.Stat(value); err == nil {
		return Context{Type: ContextFile, Value: value}
	}
	return Context{Type: ContextString, Value: value}
}

// NeedsWebFetch returns true if the context requires web fetching capability.
func (c Context) NeedsWebFetch() bool {
	return c.Type == ContextURL
}

// NeedsFileRead returns true if the context requires file reading capability.
func (c Context) NeedsFileRead() bool {
	return c.Type == ContextFile
}

const formalTemplate = `<subject line, 80 chars max>

<body: description of the solution/change>

[Problem]
<why this change is needed>

[Test]
<how it was tested or should be tested>`

// Build constructs the prompt to send to the AI agent.
//
// gitCmd is the git command the agent should run to obtain the diff
// (e.g., "git diff --cached" or "git show <SHA> --format= -p").
// style controls the commit message format.
// ctx provides optional additional context.
func Build(gitCmd string, style Style, ctx Context) string {
	var sb strings.Builder

	sb.WriteString("You are a git commit message generator.\n\n")
	sb.WriteString(fmt.Sprintf("Run the following git command to see the changes:\n\n  %s\n\n", gitCmd))

	switch style {
	case StyleFormal:
		sb.WriteString("Write a formal, multi-section commit message using EXACTLY this template:\n\n")
		sb.WriteString("```\n")
		sb.WriteString(formalTemplate)
		sb.WriteString("\n```\n\n")
		sb.WriteString("Guidelines:\n")
		sb.WriteString("- Subject line: max 80 characters, imperative mood, no trailing period\n")
		sb.WriteString("- Body: describe what changed and how\n")
		sb.WriteString("- [Problem]: explain why this change is needed\n")
		sb.WriteString("- [Test]: describe how the change was or should be tested\n")
		sb.WriteString("- [Ticket]: include ONLY if a ticket URL is available from the provided context. If no ticket URL is present, omit the [Ticket] section entirely.\n\n")
	case StyleInformal:
		sb.WriteString("Write a single-line commit message, maximum 72 characters.\n")
		sb.WriteString("Use the imperative mood (e.g., 'Fix bug' not 'Fixed bug').\n\n")
	}

	// Append context instructions
	switch ctx.Type {
	case ContextFile:
		sb.WriteString(fmt.Sprintf("Read the file at %s for additional context.\n\n", ctx.Value))
	case ContextURL:
		sb.WriteString(fmt.Sprintf("Fetch and review %s for additional context.\n\n", ctx.Value))
	case ContextString:
		sb.WriteString(fmt.Sprintf("Additional context: %s\n\n", ctx.Value))
	}

	sb.WriteString("Output the commit message wrapped ONLY between these exact delimiters:\n")
	sb.WriteString("===COMMIT_MSG_START===\n")
	sb.WriteString("<your commit message here>\n")
	sb.WriteString("===COMMIT_MSG_END===\n\n")
	sb.WriteString("Output NOTHING outside the delimiters. Do not include any explanation, preamble, or commentary.")

	return sb.String()
}
