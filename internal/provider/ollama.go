package provider

import (
	"context"
	"errors"
	"net/http"
	"time"

	ollama "github.com/liliang-cn/ollama-go"
)

// OllamaProvider implements the Provider interface using the Ollama Go client.
type OllamaProvider struct {
	// Host is the Ollama server address (default: http://localhost:11434)
	Host string
	// HTTP client for making requests
	client *http.Client
}

// NewOllamaProvider creates a new Ollama provider instance.
// host: optional Ollama server address (e.g., "http://localhost:11434", "http://127.0.0.1:11435")
// If host is empty, it defaults to http://localhost:11434
func NewOllamaProvider(host string) (*OllamaProvider, error) {
	if host == "" {
		host = "http://localhost:11434"
	}
	return &OllamaProvider{
		Host: host,
		client: &http.Client{Timeout: 3 * time.Minute},
	}, nil
}

// Send sends a prompt to the underlying model and returns the raw response as a string.
func (op *OllamaProvider) Send(prompt string, model string) (*string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	client, err := ollama.NewClient(
		ollama.WithHost(op.Host),
		ollama.WithHTTPClient(op.client),
	)
	if err != nil {
		return nil, err
	}

	resp, err := client.Chat(ctx, &ollama.ChatRequest{
		Model: model,
		Messages: []ollama.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("nil response from ollama")
	}

	content := resp.Message.Content
	return &content, nil
}