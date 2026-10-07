package container

import (
	"context"
	"fmt"
	"stfg/internal/provider"
	"stfg/internal/storage"
)

type ContainerRegistry struct {
	storageProviders  map[string]StorageProviderEntry
	providerProviders map[string]ProviderProviderEntry
}

func NewContainerRegistry() *ContainerRegistry {
	return &ContainerRegistry{
		storageProviders:  make(map[string]StorageProviderEntry),
		providerProviders: make(map[string]ProviderProviderEntry),
	}
}

func (c *ContainerRegistry) RegisterStorageProvider(ctx context.Context, key string, loader func(ctx context.Context, opts StorageProviderOptions) (storage.StorageProvider, error)) {
	c.storageProviders[key] = StorageProviderEntry{
		ctx:    ctx,
		loader: loader,
	}
}

func (c *ContainerRegistry) RegisterProviderProvider(ctx context.Context, key string, loader func(ctx context.Context, opts ProviderProviderOptions) (provider.Provider, error)) {
	c.providerProviders[key] = ProviderProviderEntry{
		ctx:    ctx,
		loader: loader,
	}
}

func (c *ContainerRegistry) GetStorageProvider(ctx context.Context, key string, opts StorageProviderOptions) (storage.StorageProvider, error) {
	entry, exists := c.storageProviders[key]
	if !exists {
		return nil, fmt.Errorf("storage provider with key '%s' not found", key)
	}
	return entry.loader(ctx, opts)
}

func (c *ContainerRegistry) GetProviderProvider(ctx context.Context, key string, opts ProviderProviderOptions) (provider.Provider, error) {
	entry, exists := c.providerProviders[key]
	if !exists {
		return nil, fmt.Errorf("provider provider with key '%s' not found", key)
	}
	return entry.loader(ctx, opts)
}
