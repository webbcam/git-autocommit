package provider

import (
	"fmt"
	"os/exec"
)

// KiroProvider implements the Provider interface using the kiro-cli.
type KiroProvider struct {
	Binary string
	Model  string
}

// NewKiroProvider creates a new KiroProvider with the given binary path and optional model.
func NewKiroProvider(binary, model string) *KiroProvider {
	return &KiroProvider{Binary: binary, Model: model}
}

// Generate invokes kiro-cli in non-interactive mode with the given prompt and returns the raw output.
func (k *KiroProvider) Generate(prompt string) (string, error) {
	args := []string{
		"chat",
		"--no-interactive",
	}
	if k.Model != "" {
		args = append(args, "--model", k.Model)
	}
	args = append(args, prompt)

	cmd := exec.Command(k.Binary, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("kiro exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run kiro: %w", err)
	}

	return string(out), nil
}
