package provider

import (
	"context"
	"errors"
	"testing"
)

func TestSend_RejectsEmptyRequiredParameters(t *testing.T) {
	openaiProvider := &OpenAIProvider{}
	ollamaProvider := &OllamaProvider{}

	cases := []struct {
		name     string
		send     func(context.Context, string, string) ([]GroceryFlyerMatch, error)
		provider string
		prompt   string
		model    string
		field    string
	}{
		{
			name:     "openai empty prompt",
			send:     openaiProvider.Send,
			provider: "openai",
			model:    "gpt-4.1-nano",
			field:    "prompt",
		},
		{
			name:     "openai empty model",
			send:     openaiProvider.Send,
			provider: "openai",
			prompt:   "milk",
			field:    "model",
		},
		{
			name:     "ollama empty prompt",
			send:     ollamaProvider.Send,
			provider: "ollama",
			model:    "llama3",
			field:    "prompt",
		},
		{
			name:     "ollama empty model",
			send:     ollamaProvider.Send,
			provider: "ollama",
			prompt:   "milk",
			field:    "model",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.send(context.Background(), tc.prompt, tc.model)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}
			if ve.Provider != tc.provider || ve.Field != tc.field || ve.Msg != "empty" {
				t.Fatalf("got provider=%q field=%q msg=%q", ve.Provider, ve.Field, ve.Msg)
			}
		})
	}
}
