package container

import (
	"context"
	"stfg/internal/provider"
	"stfg/internal/storage"
	"stfg/internal/storage/bolt"
	"stfg/internal/storage/json"
	"stfg/internal/storage/sqlite"
)

type StorageProviderOptions struct {
	BoltOptions   bolt.BoltOptions
	SQLiteOptions sqlite.SQLiteOptions
	JSONOptions   json.JSONOptions
}

type ProviderProviderOptions struct {
	OllamaOptions     provider.OllamaOptions
	OpenRouterOptions provider.OpenRouterOptions
	OpenAIOptions     provider.OpenAIOptions
}

type StorageProviderEntry struct {
	ctx    context.Context
	loader func(ctx context.Context, opts StorageProviderOptions) (storage.StorageProvider, error)
}

type ProviderProviderEntry struct {
	ctx    context.Context
	loader func(ctx context.Context, opts ProviderProviderOptions) (provider.Provider, error)
}
