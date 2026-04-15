package provider

import (
	"fmt"
	"os/exec"
)

// OpenCodeProvider implements the Provider interface using the opencode CLI.
type OpenCodeProvider struct {
	Binary string
	Model  string
}

// NewOpenCodeProvider creates a new OpenCodeProvider with the given binary path and model.
func NewOpenCodeProvider(binary, model string) *OpenCodeProvider {
	return &OpenCodeProvider{Binary: binary, Model: model}
}

// Generate invokes the opencode CLI with the given prompt and returns the raw output.
func (o *OpenCodeProvider) Generate(prompt string) (string, error) {
	args := []string{
		"-p", prompt,
		"--model", o.Model,
		"-q", // suppress spinner
	}

	cmd := exec.Command(o.Binary, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("opencode exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run opencode: %w", err)
	}

	return string(out), nil
}
