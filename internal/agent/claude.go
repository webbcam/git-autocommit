package agent

import (
	"fmt"
	"os/exec"
)

// ClaudeAgent implements the Agent interface using the Claude CLI.
type ClaudeAgent struct {
	Binary string
	Model  string
}

// NewClaudeAgent creates a new ClaudeAgent with the given binary path and model.
func NewClaudeAgent(binary, model string) *ClaudeAgent {
	return &ClaudeAgent{
		Binary: binary,
		Model:  model,
	}
}

// Generate invokes the Claude CLI with the given prompt and returns the raw output.
// needsWebFetch and needsFileRead control which additional tools are enabled.
func (c *ClaudeAgent) Generate(prompt string, needsWebFetch bool, needsFileRead bool) (string, error) {
	tools := "Bash"
	if needsWebFetch || needsFileRead {
		if needsWebFetch {
			tools += ",WebFetch"
		}
		if needsFileRead {
			tools += ",Read"
		}
	}

	args := []string{
		"--bare",
		"-p", prompt,
		"--tools", tools,
		"--allowedTools", tools,
		"--model", c.Model,
	}

	cmd := exec.Command(c.Binary, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude agent exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run claude agent: %w", err)
	}

	return string(out), nil
}
