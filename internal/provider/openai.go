package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openAIAPIURL = "https://api.openai.com/v1/chat/completions"

// OpenAIProvider implements the Provider interface using the OpenAI-compatible chat completions API.
type OpenAIProvider struct {
	APIKey  string
	Model   string
	BaseURL string // defaults to openAIAPIURL; override for compatible providers
}

// NewOpenAIProvider creates a new OpenAIProvider with the given API key, model, and optional base URL.
func NewOpenAIProvider(apiKey, model, baseURL string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = openAIAPIURL
	}
	return &OpenAIProvider{APIKey: apiKey, Model: model, BaseURL: baseURL}
}

type openAIRequest struct {
	Model               string          `json:"model"`
	MaxCompletionTokens int             `json:"max_completion_tokens"`
	Messages            []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
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

// Generate sends the prompt to the OpenAI-compatible chat completions API and returns the response text.
func (o *OpenAIProvider) Generate(prompt string) (string, error) {
	reqBody := openAIRequest{
		Model:               o.Model,
		MaxCompletionTokens: 2048,
		Messages:            []openAIMessage{{Role: "user", Content: prompt}},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, o.BaseURL, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read OpenAI API response: %w", err)
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse OpenAI API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if apiResp.Error != nil {
			return "", fmt.Errorf("OpenAI API error (%s): %s", apiResp.Error.Type, apiResp.Error.Message)
		}
		return "", fmt.Errorf("OpenAI API returned status %d", resp.StatusCode)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in OpenAI API response")
	}
	return apiResp.Choices[0].Message.Content, nil
}
