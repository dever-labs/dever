// Package ai provides a thin, dependency-free LLM client used by devx for
// two features:
//
//  1. Connection string detection — scanning a service's source directory and
//     asking the LLM which env var names the application expects.
//  2. Deployment generation — producing production-ready deployment artifacts
//     (Docker Compose, Kubernetes manifests, Helm charts, Terraform) from the
//     project's devx.yaml profile.
//
// Supported providers: openai, anthropic, ollama, azure-openai.
// Credentials are read from environment variables — never stored in devx.yaml.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client is a lightweight LLM client. Create one with New().
type Client struct {
	provider string
	model    string
	baseURL  string
	apiKey   string
	http     *http.Client
}

// New creates a Client from the given provider, model and optional baseURL.
// The API key is read from the environment at construction time:
//
//	openai:       OPENAI_API_KEY
//	anthropic:    ANTHROPIC_API_KEY
//	azure-openai: AZURE_OPENAI_KEY
//	ollama:       no key required
func New(provider, model, baseURL string) (*Client, error) {
	if provider == "" {
		return nil, fmt.Errorf("ai provider is not configured — add an 'ai' block to devx.yaml")
	}
	if model == "" {
		return nil, fmt.Errorf("ai model is not configured — add a model to the 'ai' block in devx.yaml")
	}

	var apiKey string
	switch provider {
	case "openai":
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable is not set")
		}
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
	case "anthropic":
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is not set")
		}
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
	case "azure-openai":
		apiKey = os.Getenv("AZURE_OPENAI_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("AZURE_OPENAI_KEY environment variable is not set")
		}
		if baseURL == "" {
			return nil, fmt.Errorf("ai.baseURL is required for azure-openai provider")
		}
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
	default:
		return nil, fmt.Errorf("unknown ai provider %q — supported: openai, anthropic, ollama, azure-openai", provider)
	}

	return &Client{
		provider: provider,
		model:    model,
		baseURL:  baseURL,
		apiKey:   apiKey,
		http:     &http.Client{Timeout: 90 * time.Second},
	}, nil
}

// complete sends a single system + user message pair and returns the text reply.
func (c *Client) complete(ctx context.Context, system, user string) (string, error) {
	switch c.provider {
	case "openai", "azure-openai":
		return c.openAIComplete(ctx, system, user)
	case "anthropic":
		return c.anthropicComplete(ctx, system, user)
	case "ollama":
		return c.ollamaComplete(ctx, system, user)
	default:
		return "", fmt.Errorf("unsupported provider: %s", c.provider)
	}
}

// ── OpenAI / Azure OpenAI ────────────────────────────────────────────────────

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) openAIComplete(ctx context.Context, system, user string) (string, error) {
	body := openAIRequest{
		Model: c.model,
		Messages: []openAIMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
	}

	url := c.baseURL + "/chat/completions"
	resp, err := c.postJSON(ctx, url, body, map[string]string{
		"Authorization": "Bearer " + c.apiKey,
	})
	if err != nil {
		return "", err
	}

	var result openAIResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parsing openai response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("openai API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices")
	}
	return result.Choices[0].Message.Content, nil
}

// ── Anthropic ────────────────────────────────────────────────────────────────

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) anthropicComplete(ctx context.Context, system, user string) (string, error) {
	body := anthropicRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    system,
		Messages:  []anthropicMessage{{Role: "user", Content: user}},
	}

	url := c.baseURL + "/v1/messages"
	resp, err := c.postJSON(ctx, url, body, map[string]string{
		"x-api-key":         c.apiKey,
		"anthropic-version": "2023-06-01",
	})
	if err != nil {
		return "", err
	}

	var result anthropicResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parsing anthropic response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("anthropic API error: %s", result.Error.Message)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("anthropic returned empty content")
	}
	return result.Content[0].Text, nil
}

// ── Ollama ───────────────────────────────────────────────────────────────────

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"` // same shape as OpenAI
	Stream   bool            `json:"stream"`
}

type ollamaResponse struct {
	Message openAIMessage `json:"message"`
	Error   string        `json:"error,omitempty"`
}

func (c *Client) ollamaComplete(ctx context.Context, system, user string) (string, error) {
	body := ollamaRequest{
		Model: c.model,
		Messages: []openAIMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Stream: false,
	}

	url := c.baseURL + "/api/chat"
	resp, err := c.postJSON(ctx, url, body, nil)
	if err != nil {
		return "", err
	}

	var result ollamaResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parsing ollama response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("ollama error: %s", result.Error)
	}
	return result.Message.Content, nil
}

// ── HTTP helper ──────────────────────────────────────────────────────────────

func (c *Client) postJSON(ctx context.Context, url string, body any, headers map[string]string) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AI request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("AI API returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}
