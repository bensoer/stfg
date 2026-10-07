package provider

import "context"

// Provider defines the contract for sending prompts to a model and receiving responses.
type Provider interface {
	// Send sends a prompt to the underlying model and returns the raw response as a string.
	Send(ctx context.Context, prompt string, model string) ([]GroceryFlyerMatch, error)
}
