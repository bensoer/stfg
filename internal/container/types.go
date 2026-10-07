package container

import (
	"context"
	"stfg/internal/provider"
	"stfg/internal/storage"
)

type StorageProviderEntry struct {
	ctx    context.Context
	loader func(ctx context.Context) (storage.StorageProvider, error)
}

type ProviderProviderEntry struct {
	ctx    context.Context
	loader func(ctx context.Context) (provider.Provider, error)
}
