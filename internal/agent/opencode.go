package agent

import (
	"fmt"
	"os/exec"
)

// OpenCodeAgent implements the Agent interface using the opencode CLI.
type OpenCodeAgent struct {
	Binary string
	Model  string
}

// NewOpenCodeAgent creates a new OpenCodeAgent with the given binary path and model.
func NewOpenCodeAgent(binary, model string) *OpenCodeAgent {
	return &OpenCodeAgent{Binary: binary, Model: model}
}

// Generate invokes the opencode CLI with the given prompt and returns the raw output.
func (o *OpenCodeAgent) Generate(prompt string) (string, error) {
	args := []string{
		"-p", prompt,
		"--model", o.Model,
		"-q", // suppress spinner
	}

	cmd := exec.Command(o.Binary, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("opencode agent exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run opencode agent: %w", err)
	}

	return string(out), nil
}
