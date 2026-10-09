package bootstrap

import (
	"context"
	"fmt"
	"stfg/internal/container"
	"stfg/internal/provider"
	"stfg/internal/storage"
	"stfg/internal/storage/bolt"
	"stfg/internal/storage/json"
	"stfg/internal/storage/sqlite"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// registryCtxKey is the context key for the container registry.
type registryCtxKey struct{}

func GetContainer(cmd *cobra.Command) *container.ContainerRegistry {
	reg, ok := cmd.Context().Value(registryCtxKey{}).(*container.ContainerRegistry)
	if ok && reg != nil {
		return reg
	}
	return nil
}

// getStore retrieves the storage.Storage handle from the cobra command context.
// The storage handle is set by the root command's PersistentPreRunE, so every
// subcommand has access to it. If the handle is missing (should not happen in
// normal operation) a fresh store is created as a fallback.
func RegisterProviders(ctx context.Context) *container.ContainerRegistry {
	reg := container.NewContainerRegistry()

	// --- Storage providers ---
	reg.RegisterStorageProvider(ctx, "json", func(c context.Context) (storage.StorageProvider, error) {
		return json.NewJSON(c, json.JSONOptions{})
	})
	reg.RegisterStorageProvider(ctx, "sqlite", func(c context.Context) (storage.StorageProvider, error) {
		return sqlite.NewSQLite(c, sqlite.SQLiteOptions{
			CacheDir:       viper.GetString("storage.sqlite.cache_dir"),
			SQLiteFileName: viper.GetString("storage.sqlite.sqlite_file_name"),
		})
	})
	reg.RegisterStorageProvider(ctx, "bolt", func(c context.Context) (storage.StorageProvider, error) {
		return bolt.NewBolt(c, bolt.BoltOptions{
			CacheDir:     viper.GetString("storage.bolt.cache_dir"),
			BoltFileName: viper.GetString("storage.bolt.bolt_file_name"),
		})
	})

	// --- Provider providers ---
	reg.RegisterProviderProvider(ctx, "openrouter", func(c context.Context) (provider.Provider, error) {
		apiKey := viper.GetString("providers.openrouter.api_key")
		if apiKey == "" {
			return nil, fmt.Errorf("openrouter API key is not set. Please set it in the config file or via the environment variable STFG_PROVIDERS_OPENROUTER_API_KEY")
		}
		zap.S().Info("Using OpenRouter API Key: ", apiKey)
		return provider.NewOpenRouterProvider(provider.OpenRouterOptions{
			APIKey: viper.GetString("providers.openrouter.api_key"),
		})
	})
	reg.RegisterProviderProvider(ctx, "ollama", func(c context.Context) (provider.Provider, error) {
		return provider.NewOllamaProvider(provider.OllamaOptions{
			Host: viper.GetString("providers.ollama.host"),
		})
	})
	reg.RegisterProviderProvider(ctx, "openai", func(c context.Context) (provider.Provider, error) {
		return provider.NewOpenAIProvider(provider.OpenAIOptions{
			APIKey: viper.GetString("providers.openai.api_key"),
		})
	})

	return reg
}
