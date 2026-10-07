# Architecture

## Overview

`stfg` is a CLI tool for searching the best deals on groceries from flyers and stores. It's built as a Go application using the Cobra framework for command-line interface management.

## Core Components

### CLI Framework
- **Cobra** – Used for building the command structure, subcommands, and global configuration.
- Global flags: `--config` (config file path), `--log-file` (log destination), `--log-level` (debug/info/warn/error), `--quiet` (suppress stdout/stderr).
- Persistent pre-run hook initializes logging and storage.

### Storage Layer
Three storage backends are supported:
- **JSON File Storage** – Primary implementation using `stfg/internal/storage/json/jsonfilestorage`. Files are cached in the OS cache directory (`~/.cache/stfg`). Provides human-readable persistence for grocery lists and flyer data.
- **SQLite Backend** – Relational storage for structured queries (flyer metadata, grocery items).
- **BoltDB Backend** – High-performance key-value storage for fast lookups.

All storage handles are injected via Cobra context so subcommands can retrieve them consistently.

### Models
- **GroceryItem** – Contains name and embedding vector. The vector is produced by `GroceryEmbedder` in `internal/embedding`, which calls `Provider.Embed`.
- **FlyerFinder** – Interfaces with the Flipp API to parse and categorize flyers.
- **Reconciler** – Matches flyer items against the user's grocery list.
- **PromptWriter** – One `PromptWriter` in `internal/promptwriter/prompt_writer.go` takes a `provider.Provider` and a model name, builds the flyer/grocery prompt, calls `Provider.Send`, and checks the returned matches against the real `storage.FlyerItem` rows.

### Providers
- **OpenAI, Ollama, and OpenRouter** – Implementations of `internal/provider.Provider`. `Send` returns `[]GroceryFlyerMatch` decoded from a shared `{"matches":[...]}` JSON schema. The three providers also implement `Embed`, which returns one `[]float32` vector for one text input.
- **Flipp API** – Source for retail flyers; parsed by the `Flipp` client.

### Data Flow
1. **Add Grocery** – User adds an item; `add-grocery` constructs `NewOpenRouterProvider` from `api_key` / `--api-key` and passes that provider plus `openai/text-embedding-3-small` to `NewGroceryEmbedder`. The embedder calls `Provider.Embed` and stores the returned vector in the chosen storage backend. It does not select among providers.
2. **Scrape Flyers** – The `scrape-flyers` command queries the Flipp API for flyers in a given postal code, filters by a whitelist of retailers, and stores them.
3. **Find Deals** – The `find-deals` command loads all groceries and flyers, constructs `NewOpenRouterProvider`, and passes that provider plus `openrouter/free` to `NewPromptWriter`. The prompt writer calls `Provider.Send` and produces a ranked list of deals.

## Directory Structure

```
stfg/
├── cmd/              # Cobra commands (groceries, add-grocery, list, remove, scrape-flyers, find-deals)
├── internal/
│   ├── embedding/   # GroceryEmbedder: provider-backed vector generation
│   ├── storage/     # Storage DTOs and backends (JSON, SQLite, BoltDB)
│   ├── flyerfinder/ # Flipp API integration
│   ├── reconciler/  # Flyer matching logic
│   ├── provider/    # OpenAI, Ollama, and OpenRouter LLM providers
│   └── promptwriter/ # Prompt writer over any Provider
├── docs/            # Documentation (ARCHITECTURE.md, DECISIONS.md, PROJECT_CONTEXT.md)
├── main.go          # Entry point (invokes root.Execute())
└── Makefile         # Build orchestration
```

## Design Principles

- **Separation of Concerns** – Each component (storage, models, providers) is isolated behind interfaces.
- **Testability** – All major components are unit-testable (e.g., `test(storage): ...`).
- **Extensibility** – New storage backends or providers can be added by implementing the common interfaces.
- **Configuration** – Centralized via Viper (YAML config file, environment variables, and CLI flags).

## Technology Stack

- **Language** – Go 1.26+
- **CLI Framework** – Cobra
- **Logging** – Zap (structured JSON + console)
- **Serialization** – `github.com/spf13/viper` (config); embeddings are produced by hosted LLM providers via `Provider.Embed`
- **Database** – JSON file (default), SQLite, BoltDB
- **External Services** – Flipp API (retail flyers), OpenAI, Ollama, and OpenRouter (LLM)
