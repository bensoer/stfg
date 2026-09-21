# Provider Architecture Plan

## Overview

This plan scopes the implementation of three new provider implementations in the `@internal/provider/` package:

- **OpenRouter** – replaces the current `openrouter.go` (which is considered insufficient)
- **Ollama** – new provider for local LLM inference via Ollama
- **OpenAI** – new provider for OpenAI-compatible APIs (e.g., ChatCompletions)

All providers must implement a common interface defined in `protocols.go`. Custom types and error types will reside in `types.go` and `errors.go` respectively.

## 1. Protocol Definition (`protocols.go`)

Create a shared interface that all providers conform to.

```go
package provider

// Provider defines the contract for sending prompts to a model and receiving responses.
type Provider interface {
	// Send sends a prompt to the underlying model and returns the raw response as a string.
	Send(prompt string, model string) (*string, error)
}
```

This is the single contract each provider must satisfy. It is defined locally here so the provider package is self-contained.

## 2. OpenRouter Provider (`internal/provider/openrouter.go`)

Replace the current `openrouter.go` (which is considered insufficient) with a proper implementation:

- Use the official OpenRouter SDK (`github.com/OpenRouterTeam/go-sdk`)
- Implement the `Provider` interface
- Handle authentication via `OPENROUTER_API_KEY` environment variable or config
- Include proper error wrapping and recovery
- Add retry logic and circuit-breaker patterns
- Ensure graceful degradation when the provider is unavailable

## 3. Ollama Provider (`internal/provider/ollama.go`)

Implement a provider that interacts with Ollama locally (via `ollama` CLI or `ollama-go` SDK):

- Implement the `Provider` interface
- Support both local model loading and remote model serving
- Handle model download/loading errors gracefully

## 4. OpenAI Provider (`internal/provider/openai.go`)

Implement a provider that wraps OpenAI's Completion API:

- Use the `openai` SDK (`github.com/openai/openai`)
- Implement the `Provider` interface
- Support chat completions (e.g., `gpt-4o`) for natural language reasoning
- Handle rate limiting and retries

## 5. Supporting Files

### `types.go`

Add custom types for the protocol:

- `ProviderConfig` – configuration struct for each provider (API key, model name, etc.)
- `ProviderMetadata` – additional metadata about the provider (version, endpoint, etc.)

### `errors.go`

Add error types for the protocol:

- `ProviderError` – base error for all provider failures
- `ModelResponseError` – malformed model response
- `NetworkError` – connection/timeout issues
- `ValidationError` – invalid input validation

## 6. Migration Strategy

1. **Phase 1** – Define `protocols.go`, `types.go`, `errors.go` (shared contracts)
2. **Phase 2** – Implement `ollama.go` and `openai.go` providers
3. **Phase 3** – Refactor `openrouter.go` to be a proper implementation (not just a template)
4. **Phase 4** – Wire the new providers into the rest of the codebase as needed

## 7. Testing

- Unit tests for each provider's `Provider` implementation
- Integration tests using mock servers or local Docker containers
- Ensure each provider correctly handles success, error, and edge cases

## Summary

| Component | File | Status |
|-----------|------|--------|
| Protocol interface | `internal/provider/protocols.go` (new) | ✅ Planned |
| OpenRouter provider | `internal/provider/openrouter.go` | 🔄 Replace existing |
| Ollama provider | `internal/provider/ollama.go` | ⏳ Implement |
| OpenAI provider | `internal/provider/openai.go` | ⏳ Implement |
| Type definitions | `internal/provider/types.go` | ⏳ Implement |
| Error types | `internal/provider/errors.go` | ⏳ Implement |
