package provider

import (
	"testing"
)

// Compile-time assertion that *OpenAIProvider satisfies Provider.
// If *OpenAIProvider stops implementing Provider, this line fails here.
var _ Provider = (*OpenAIProvider)(nil)

// Compile-time assertion that *OllamaProvider satisfies Provider.
// If *OllamaProvider stops implementing Provider, this line fails here.
var _ Provider = (*OllamaProvider)(nil)

// Compile-time assertion that *OpenRouterProvider satisfies Provider.
// If *OpenRouterProvider stops implementing Provider, this line fails here.
var _ Provider = (*OpenRouterProvider)(nil)

// TestProvidersImplementContract keeps the three compile-time assertions
// reachable from `go test` so a missing Embed is reported by the test runner
// rather than only at package build time.
func TestProvidersImplementContract(t *testing.T) {
	var _ Provider = (*OpenAIProvider)(nil)
	var _ Provider = (*OllamaProvider)(nil)
	var _ Provider = (*OpenRouterProvider)(nil)
}
