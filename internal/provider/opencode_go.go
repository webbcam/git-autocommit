package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	opencodeGoOpenAIURL    = "https://opencode.ai/zen/go/v1/chat/completions"
	opencodeGoAnthropicURL = "https://opencode.ai/zen/go/v1/messages"

	// EndpointTypeOpenAI selects the OpenAI-compatible endpoint.
	// Supported models: glm-5, glm-5.1, kimi-k2.5, mimo-v2-pro, mimo-v2-omni
	EndpointTypeOpenAI = "openai"

	// EndpointTypeAnthropic selects the Anthropic-compatible endpoint.
	// Supported models: minimax-m2.5, minimax-m2.7
	EndpointTypeAnthropic = "anthropic"
)

// OpenCodeGoProvider implements the Provider interface using the OpenCode Go API directly.
type OpenCodeGoProvider struct {
	APIKey       string
	Model        string
	EndpointType string // "openai" or "anthropic"
}

// NewOpenCodeGoProvider creates a new OpenCodeGoProvider.
// endpointType must be "openai" (default) or "anthropic".
func NewOpenCodeGoProvider(apiKey, model, endpointType string) *OpenCodeGoProvider {
	if endpointType == "" {
		endpointType = EndpointTypeOpenAI
	}
	return &OpenCodeGoProvider{APIKey: apiKey, Model: model, EndpointType: endpointType}
}

// Generate sends the prompt to the OpenCode Go API and returns the response text.
func (o *OpenCodeGoProvider) Generate(prompt string) (string, error) {
	switch o.EndpointType {
	case EndpointTypeAnthropic:
		return o.generateAnthropic(prompt)
	default:
		return o.generateOpenAI(prompt)
	}
}

// openai-compatible types

type opencodeGoOpenAIRequest struct {
	Model               string                    `json:"model"`
	MaxCompletionTokens int                       `json:"max_completion_tokens"`
	Messages            []opencodeGoOpenAIMessage `json:"messages"`
}

type opencodeGoOpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type opencodeGoOpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (o *OpenCodeGoProvider) generateOpenAI(prompt string) (string, error) {
	reqBody := opencodeGoOpenAIRequest{
		Model:               o.Model,
		MaxCompletionTokens: 2048,
		Messages:            []opencodeGoOpenAIMessage{{Role: "user", Content: prompt}},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, opencodeGoOpenAIURL, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call OpenCode Go API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read OpenCode Go API response: %w", err)
	}

	var apiResp opencodeGoOpenAIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse OpenCode Go API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if apiResp.Error != nil {
			return "", fmt.Errorf("OpenCode Go API error (%s): %s", apiResp.Error.Type, apiResp.Error.Message)
		}
		return "", fmt.Errorf("OpenCode Go API returned status %d", resp.StatusCode)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in OpenCode Go API response")
	}
	return apiResp.Choices[0].Message.Content, nil
}

// anthropic-compatible types

type opencodeGoAnthropicRequest struct {
	Model     string                       `json:"model"`
	MaxTokens int                          `json:"max_tokens"`
	Messages  []opencodeGoAnthropicMessage `json:"messages"`
}

type opencodeGoAnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type opencodeGoAnthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (o *OpenCodeGoProvider) generateAnthropic(prompt string) (string, error) {
	reqBody := opencodeGoAnthropicRequest{
		Model:     o.Model,
		MaxTokens: 2048,
		Messages:  []opencodeGoAnthropicMessage{{Role: "user", Content: prompt}},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, opencodeGoAnthropicURL, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call OpenCode Go API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read OpenCode Go API response: %w", err)
	}

	var apiResp opencodeGoAnthropicResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse OpenCode Go API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if apiResp.Error != nil {
			return "", fmt.Errorf("OpenCode Go API error (%s): %s", apiResp.Error.Type, apiResp.Error.Message)
		}
		return "", fmt.Errorf("OpenCode Go API returned status %d", resp.StatusCode)
	}

	for _, c := range apiResp.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in OpenCode Go API response")
}
