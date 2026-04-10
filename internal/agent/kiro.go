package agent

import (
	"fmt"
	"os/exec"
)

// KiroAgent implements the Agent interface using the kiro-cli.
// Model selection is not supported per-invocation; configure it globally
// with: kiro-cli settings chat.defaultModel <model>
type KiroAgent struct {
	Binary string
}

// NewKiroAgent creates a new KiroAgent with the given binary path.
func NewKiroAgent(binary string) *KiroAgent {
	return &KiroAgent{Binary: binary}
}

// Generate invokes kiro-cli in non-interactive mode with the given prompt and returns the raw output.
func (k *KiroAgent) Generate(prompt string) (string, error) {
	args := []string{
		"chat",
		"--no-interactive",
		prompt,
	}

	cmd := exec.Command(k.Binary, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("kiro agent exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run kiro agent: %w", err)
	}

	return string(out), nil
}
