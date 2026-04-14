package prompt

import (
	"fmt"
	"os"
	"strings"
)

// ContextType represents the type of additional context.
type ContextType int

const (
	ContextNone   ContextType = iota
	ContextFile   ContextType = iota
	ContextURL    ContextType = iota
	ContextString ContextType = iota
)

// Context holds the additional context information.
type Context struct {
	Type    ContextType
	Value   string // original path, URL, or string
	Content string // resolved content (file body, URL body, or same as Value for strings)
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
	return Context{Type: ContextString, Value: value, Content: value}
}

// Build constructs the prompt to send to the AI agent.
//
// diff is the output of the git diff/show command.
// templateBody is the rendered commit message template (after variable substitution).
// ctx provides optional additional context; ctx.Content must be pre-resolved.
func Build(diff string, templateBody string, ctx Context) string {
	var sb strings.Builder

	sb.WriteString("You are a git commit message generator.\n\n")
	sb.WriteString("Here is the git diff:\n\n```diff\n")
	sb.WriteString(diff)
	sb.WriteString("```\n\n")

	sb.WriteString("Write a commit message using EXACTLY this template:\n\n")
	sb.WriteString("```\n")
	sb.WriteString(templateBody)
	sb.WriteString("\n```\n\n")

	// Append resolved context
	switch ctx.Type {
	case ContextFile:
		sb.WriteString(fmt.Sprintf("Additional context (from file %s):\n\n%s\n\n", ctx.Value, ctx.Content))
	case ContextURL:
		sb.WriteString(fmt.Sprintf("Additional context (from %s):\n\n%s\n\n", ctx.Value, ctx.Content))
	case ContextString:
		sb.WriteString(fmt.Sprintf("Additional context: %s\n\n", ctx.Content))
	}

	sb.WriteString("Output the commit message wrapped ONLY between these exact delimiters:\n")
	sb.WriteString("===COMMIT_MSG_START===\n")
	sb.WriteString("<your commit message here>\n")
	sb.WriteString("===COMMIT_MSG_END===\n\n")
	sb.WriteString("Output NOTHING outside the delimiters. Do not include any explanation, preamble, or commentary.")

	return sb.String()
}
