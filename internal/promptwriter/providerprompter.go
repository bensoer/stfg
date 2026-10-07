package promptwriter

import (
	"context"

	"stfg/internal/provider"
)

type ProviderPrompter interface {
	// Sends a prompt to a model and returns response as a map[string]any
	Send(ctx context.Context, prompt string, model string) ([]provider.GroceryFlyerMatch, error)
}
