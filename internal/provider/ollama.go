package provider

import (
	"context"
	"errors"
	"fmt"
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
		Host:   host,
		client: &http.Client{Timeout: 3 * time.Minute},
	}, nil
}

// Send sends a prompt to the underlying model and returns the raw response as a string.
func (op *OllamaProvider) Send(ctx context.Context, prompt string, model string) ([]GroceryFlyerMatch, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
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
		Format: groceryFlyerMatchesSchema(),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("nil response from ollama")
	}

	matches, err := decodeGroceryFlyerMatches("ollama", resp.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("ollama decode matches: %w", err)
	}
	return matches, nil
}

// Embed returns one float32 embedding for text from model.
func (op *OllamaProvider) Embed(ctx context.Context, text string, model string) ([]float32, error) {
	if text == "" {
		return nil, &ValidationError{Provider: "ollama", Field: "text", Msg: "empty"}
	}
	if model == "" {
		return nil, &ValidationError{Provider: "ollama", Field: "model", Msg: "empty"}
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	client, err := ollama.NewClient(
		ollama.WithHost(op.Host),
		ollama.WithHTTPClient(op.client),
	)
	if err != nil {
		return nil, err
	}

	resp, err := client.Embed(ctx, &ollama.EmbedRequest{Model: model, Input: text})
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	if resp == nil || len(resp.Embeddings) != 1 {
		return nil, &ModelResponseError{Provider: "ollama", Raw: fmt.Sprintf("expected exactly one embedding, got %d", len(resp.Embeddings))}
	}
	return toFloat32Vector("ollama", resp.Embeddings[0])
}
