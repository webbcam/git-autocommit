package parser

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	startDelimiter = "===COMMIT_MSG_START==="
	endDelimiter   = "===COMMIT_MSG_END==="
)

// ansiEscapeRe matches ANSI escape sequences.
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[mGKHFJsu]`)

// StripANSI removes ANSI escape codes from the input string.
func StripANSI(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

// ExtractCommitMessage extracts the commit message from the agent output.
// It strips ANSI codes, then finds text between the delimiters.
// Returns an error if the delimiters are not found.
func ExtractCommitMessage(raw string) (string, error) {
	clean := StripANSI(raw)

	start := strings.Index(clean, startDelimiter)
	if start == -1 {
		return "", fmt.Errorf("Failed to generate commit message.")
	}
	start += len(startDelimiter)

	end := strings.Index(clean[start:], endDelimiter)
	if end == -1 {
		return "", fmt.Errorf("Failed to generate commit message.")
	}

	msg := clean[start : start+end]
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "", fmt.Errorf("Failed to generate commit message.")
	}
	return msg, nil
}
