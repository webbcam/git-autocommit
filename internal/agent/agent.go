package agent

// Agent defines the interface for AI agents that generate commit messages.
type Agent interface {
	// Generate sends the prompt to the AI agent and returns the raw output.
	Generate(prompt string) (string, error)
}
