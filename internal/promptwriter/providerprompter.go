package promptwriter

type ProviderPrompter interface {
	// Sends a prompt to a model and returns response as a map[string]any
	Send(prompt, model string) (*string, error)
}
