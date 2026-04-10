package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const ollamaDefaultURL = "http://localhost:11434/v1/chat/completions"

// OllamaAgent implements the Agent interface using a locally running Ollama instance.
// Ollama exposes an OpenAI-compatible API; no API key is required.
type OllamaAgent struct {
	Model   string
	BaseURL string
}

// NewOllamaAgent creates an OllamaAgent. If baseURL is empty it defaults to localhost.
func NewOllamaAgent(model, baseURL string) *OllamaAgent {
	if baseURL == "" {
		baseURL = ollamaDefaultURL
	}
	return &OllamaAgent{Model: model, BaseURL: baseURL}
}

type ollamaRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []ollamaMessage `json:"messages"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Generate sends the prompt to the Ollama OpenAI-compatible endpoint and returns the response.
func (o *OllamaAgent) Generate(prompt string) (string, error) {
	reqBody := ollamaRequest{
		Model:     o.Model,
		MaxTokens: 2048,
		Messages:  []ollamaMessage{{Role: "user", Content: prompt}},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, o.BaseURL, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Ollama API response: %w", err)
	}

	var apiResp ollamaResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse Ollama API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if apiResp.Error != nil {
			return "", fmt.Errorf("Ollama API error: %s", apiResp.Error.Message)
		}
		return "", fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in Ollama API response")
	}
	return apiResp.Choices[0].Message.Content, nil
}
