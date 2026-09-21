// Package provider implements the Provider interface for LLM interactions.
//
// The provider package supports multiple LLM backends through a unified interface:
// - OpenRouter: Official OpenRouter SDK integration
// - Ollama: Local LLM inference via Ollama client
// - OpenAI: Official OpenAI SDK integration
//
// Error Handling:
// This package defines a structured error hierarchy for robust error handling:
// - ProviderError: Base error type for all provider failures
//   - Includes provider identification and operation context
//   - Supports error unwrapping for underlying causes
// - ModelResponseError: Malformed or unexpected model responses
//   - Preserves raw response data for debugging
// - NetworkError: Transient network connectivity issues
//   - Includes operation context for retry logic
// - ValidationError: Invalid input or configuration
//   - Identifies specific fields and validation messages
//
// Usage:
//   provider, _ := provider.NewOpenRouterProvider(apiKey)
//   // Providers return errors that can be type-asserted:
//   // if errors.As(err, &errors.NetworkError{}) { ... }
//   // if errors.As(err, &errors.ModelResponseError{}) { ... }
//
// Architecture:
// All providers implement the Provider interface defined in protocols.go:
//   Send(prompt string, model string) (*string, error)
//
// The interface ensures consistent behavior across different LLM backends
// while allowing each provider to leverage its specific SDK's features.
package provider

import (
	"fmt"
)

// ProviderError is the base error type for all provider failures.
type ProviderError struct {
	// Provider identifies which provider failed (e.g., "openrouter", "ollama", "openai")
	Provider string
	// Op describes the operation that failed (e.g., "Send", "Configure")
	Op string
	// Err contains the underlying error cause
	Err error
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s provider error (%s): %v", e.Provider, e.Op, e.Err)
	}
	return fmt.Sprintf("%s provider error (%s)", e.Provider, e.Op)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// ModelResponseError indicates a malformed or unexpected model response.
type ModelResponseError struct {
	// Provider identifies which provider returned the error
	Provider string
	// Raw contains the problematic response data for debugging
	Raw string
}

func (e *ModelResponseError) Error() string {
	return fmt.Sprintf("%s: unexpected model response: %s", e.Provider, e.Raw)
}

// NetworkError indicates a transient network connectivity issue.
type NetworkError struct {
	// Provider identifies which provider experienced the network issue
	Provider string
	// Op describes the network operation that failed
	Op string
	// Err contains the underlying network error
	Err error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("%s network error (%s): %v", e.Provider, e.Op, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// ValidationError indicates invalid input or configuration.
type ValidationError struct {
	// Provider identifies which provider detected the validation error
	Provider string
	// Field specifies which field or parameter was invalid
	Field string
	// Msg provides the validation error message
	Msg string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s validation error: %s: %s", e.Provider, e.Field, e.Msg)
}