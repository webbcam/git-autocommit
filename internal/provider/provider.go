package provider

// Provider defines the interface for LLM providers that generate commit messages.
type Provider interface {
	// Generate sends the prompt to the provider and returns the raw output.
	Generate(prompt string) (string, error)
}
