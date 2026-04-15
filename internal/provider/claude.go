package provider

import (
	"fmt"
	"os/exec"
)

// ClaudeProvider implements the Provider interface using the Claude CLI.
type ClaudeProvider struct {
	Binary string
	Model  string
}

// NewClaudeProvider creates a new ClaudeProvider with the given binary path and model.
func NewClaudeProvider(binary, model string) *ClaudeProvider {
	return &ClaudeProvider{
		Binary: binary,
		Model:  model,
	}
}

// Generate invokes the Claude CLI with the given prompt and returns the raw output.
func (c *ClaudeProvider) Generate(prompt string) (string, error) {
	args := []string{
		"-p", prompt,
		"--model", c.Model,
	}

	cmd := exec.Command(c.Binary, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run claude: %w", err)
	}

	return string(out), nil
}
