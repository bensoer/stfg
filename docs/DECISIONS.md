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
