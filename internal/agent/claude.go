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
func (c *ClaudeAgent) Generate(prompt string) (string, error) {
	args := []string{
		"-p", prompt,
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
