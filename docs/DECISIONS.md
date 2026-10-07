# Decision Log

This document records important architectural decisions, deviations, and non-intuitive choices for future reference.

## Storage Abstraction

**Decision:** Implement a storage abstraction interface (`internal/storage/storage.go`) with concrete implementations for JSON file storage, SQLite, and BoltDB.

**Reason:** Allows switching backends without changing the rest of the code. The JSON file backend is the default and most human-readable; SQLite and BoltDB are provided for performance and relational querying needs.

## Embedding Model Choice

**Decision:** Use llama.cpp via `tcpipuk/llama-go` with the Qwen3-Embedding-0.6B model for generating grocery item embeddings.

**Reason:** Self-contained and offline-capable. The model is downloaded on demand and stored in the `models/` directory. Context is trimmed to 1024 tokens for memory efficiency.

## Flyer Scraping via Flipp API

**Decision:** Use the Flipp API to retrieve retail flyers for a given postal code, then filter results to a whitelist of preferred retailers.

**Reason:** Flipp provides a broad coverage of Canadian grocery flyers. Whitelisting keeps the deal list relevant to the user's typical stores.

## LLM-Based Matching

**Decision:** Use OpenRouter for semantic matching between grocery items and flyer items.

**Reason:** The LLM can understand product names and context better than keyword matching alone, producing more accurate matches. The `promptwriter` package generates consistent prompts for the provider.

## Logging

**Decision:** Use Zap with JSON encoding for the file output and console output.

**Reason:** Structured JSON logs are easier to parse and debug than plain text. The `--quiet` flag suppresses console output while still writing to the log file.

## Configuration

**Decision:** Centralize configuration via Viper, reading from a YAML file at `~/.stfg.yaml`, environment variables prefixed with `STFG_`, and CLI flags.

**Reason:** Makes the CLI easy to configure without recompiling. Default retailer whitelist is defined in the config.

## Data Model

**Decision:** Store grocery items with their generated embedding vector, and flyers with their items as separate collections.

**Reason:** Separating flyers from grocery items allows for efficient querying of all items within a flyer when finding deals.

## Structured Provider Matches

**Decision:** `Provider.Send` returns `[]GroceryFlyerMatch` for every backend. The model is asked for one JSON object, `{"matches":[...]}`, not a bare array and not free text. OpenAI, Ollama, and OpenRouter all receive that same schema. Decoding lives next to the schema in the provider package so the three SDKs cannot drift.

**Reason:** The only model call in the program has one response shape. A generic `Send[T]` was rejected. OpenAI strict structured outputs need a root object, so the wire format is an envelope even though callers receive the inner slice.

## Provider-Agnostic Prompt Writer

**Decision:** `PromptWriter` depends on `provider.Provider` directly. The model name is a constructor argument. `find-deals` passes `NewOpenRouterProvider` and `openrouter/free`. There is no local prompter interface and no OpenRouter-specific writer type.

**Reason:** After `Send` returned `[]GroceryFlyerMatch`, the local interface duplicated `provider.Provider` and the writer hardcoded one model. Keeping provider choice at the command leaves OpenAI and Ollama unwired until a caller needs them, without a factory.

## Provider-Backed Grocery Embeddings

**Decision:** Grocery embeddings now come from `Provider.Embed`. The `internal/models` package — which loaded a local Qwen GGUF through llama.cpp — is gone. `add-grocery` constructs `NewOpenRouterProvider` from the `api_key` viper key (falling back to the `--api-key` / `-a` flag) and hardcodes the model `openai/text-embedding-3-small` on `NewGroceryEmbedder`. The model string lives in the command, not in `internal/embedding`.

**Reason:** The llama.cpp path was the only Go call site for `tcpipuk/llama-go`. With `Provider.Embed` available on OpenAI, Ollama, and OpenRouter, embeddings can use the same plumbing as `find-deals` and the program no longer ships a GGUF loader in Go. Stored Qwen vectors are not migrated: a re-embedding run is required to compare against the new model. The `Makefile` still builds llama.cpp because `make` remains the documented build entrypoint, but that build is no longer consumed by this package.
