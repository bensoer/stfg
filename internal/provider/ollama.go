package provider

import (
	"context"
	"errors"
	"time"

	ollama "github.com/liliang-cn/ollama-go"
)

// OllamaProvider implements the Provider interface using the Ollama Go client.
type OllamaProvider struct {
	Host string
}

// NewOllamaProvider creates a new Ollama provider instance.
func NewOllamaProvider(host string) (*OllamaProvider, error) {
	if host == "" {
		host = "http://localhost:11434"
	}
	return &OllamaProvider{
		Host: host,
	}, nil
}

// Send sends a prompt to the underlying model and returns the raw response as a string.
func (op *OllamaProvider) Send(prompt string, model string) (*string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	client, err := ollama.NewClient(ollama.WithHost(op.Host))
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