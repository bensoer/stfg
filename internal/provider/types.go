package provider

// ProviderConfig holds configuration for a specific provider.
type ProviderConfig struct {
	// API key for authentication (e.g., OpenRouter API key)
	APIKey string
	// Model identifier to use for inference
	Model string
	// Optional: endpoint URL for the provider
	Endpoint string
}

// ProviderMetadata holds additional metadata about a provider.
type ProviderMetadata struct {
	// Name of the provider (e.g., "openrouter", "ollama", "openai")
	Name string
	// Version of the provider
	Version string
	// Endpoint URL (if applicable)
	Endpoint string
}