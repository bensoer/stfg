# AGENTS.md - stfg Project Guide

## Project Overview

**stfg (Shop The Flyers Grocer)** - CLI tool for searching grocery deals from flyers and stores.

- **Language**: Go 1.26+
- **Framework**: Cobra CLI
- **Logging**: Zap (structured JSON + console)
- **Config**: Viper (YAML, env vars, flags)
- **Storage**: JSON files in OS cache dir
- **Embeddings**: Hosted LLM via `provider.Provider.Embed` (OpenRouter with `openai/text-embedding-3-small` for `add-grocery`)
- **Scraping**: Flipp API integration

---

## Quick Start

```bash
# Build (fetches llama-go, builds llama.cpp, builds binary)
make

# Run commands
./bin/stfg groceries add-grocery milk
./bin/stfg groceries list
./bin/stfg scrape-flyers V5K0A1
./bin/stfg find-deals
```

---

## Project Structure

```
stfg/
├── main.go                 # Entry point → cmd.Execute()
├── docs/                  # Living project documentation
│   ├── ARCHITECTURE.md    # Code architecture: structure, diagrams, mermaidjs
│   ├── DECISIONS.md       # Important decisions / deviations / memory notes
│   ├── PROJECT_CONTEXT.md # Product vision & desired outcome (never edit)
│   ├── investigations/    # Research / transcripts (never edit after written)
│   └── plans/             # Archive of plan documents
├── cmd/                    # Cobra commands
│   ├── root.go            # Root command, config, logging setup
│   ├── groceries.go       # Parent command for grocery ops
│   ├── addGrocery.go      # Add item with embedding
│   ├── listGroceries.go   # List all items
│   ├── removeGrocery.go   # Remove item
│   ├── scrapeFlyers.go    # Scrape flyers via Flipp API
│   └── findDeals.go       # Match flyers to grocery list (semantic search)
├── internal/
│   ├── embedding/         # GroceryEmbedder: provider-backed vector generation
│   ├── storage/           # JSON file persistence
│   │   ├── db.go          # Low-level JSON save/load + cache dir
│   │   ├── jsonfilestorage.go  # High-level Grocery/Flyer CRUD
│   │   └── types.go       # Store, Flyer, FlyerItem, GroceryItem types
│   ├── flipp/             # Flipp API client
│   ├── reconciler/        # Flyer scraping & matching logic
│   ├── provider/          # LLM providers (OpenAI, Ollama, and OpenRouter)
│   └── promptwriter/      # Builds the match prompt and validates provider matches
├── models/                # Downloaded GGUF models (gitignored)
├── bin/                   # Built binary
├── lib/                   # Compiled llama.cpp static libs
├── res/llama-go/          # llama-go source + llama.cpp submodule
└── Makefile               # Build orchestration
```

---

## Key Commands

| Command | Description |
|---------|-------------|
| `make` | Full build: clone llama-go → build llama.cpp → build stfg |
| `make clean` | Remove res/, lib/, bin/ |
| `make run` | Run via `go run` (dev) |
| `./bin/stfg groceries add-grocery <item>` | Add item (creates embedding) |
| `./bin/stfg groceries list` | List all grocery items |
| `./bin/stfg groceries remove-grocery <item>` | Remove item |
| `./bin/stfg scrape-flyers <postal-code>` | Scrape flyers for area |
| `./bin/stfg find-deals` | Match grocery list to flyer deals |

---

## Build System Details

### Makefile Flow
1. Clones `tcpipuk/llama-go` with submodules
2. Runs `git submodule update --init --recursive` in llama-go
3. Builds `libbinding.a` with:
    - `-DLLAMA_DISABLE_LOGS=1` (suppresses llama.cpp logs)
    - `-I./llama.cpp/vendor/nlohmann` (nlohmann/json include path)
4. Copies `*.a` to `lib/`
5. `go build -o bin/stfg main.go` with `CGO_LDFLAGS="-Llib"`

### Critical Flags
- `LLAMA_DISABLE_LOGS=1`: Silences llama.cpp diagnostic output
- `LLAMA_LOG=none`: Set in `models/client.go` init() as fallback
- CGO needs `libggml`, `libllama`, `libbinding` from `lib/`

### Ephemeral Directories
- The `res/` and `lib/` directories are **ephemeral by design**
- These directories are deleted by `make clean`
- `res/` contains the llama-go source code and llama.cpp submodule
- `lib/` contains the compiled llama.cpp static libraries
- Do **not** make persistent changes to these directories as they will be deleted
- For persistent changes to llama-go/llama.cpp, fork the repository and modify the Makefile to use your fork
- The `models/` directory (for GGUF models) and `bin/` directory (for the built binary) are also cleaned by `make clean`

---

## Configuration

**Config file**: `~/.stfg.yaml` (or `--config` flag)

**Env vars** (prefix `STFG_`):
- `STFG_CONFIG` - config file path
- `STFG_LOG_FILE` - log output file
- `STFG_LOG_LEVEL` - log level (debug/info/warn/error)
- `STFG_QUIET` - suppress console output

**Flags**:
- `--config` - config file
- `--log-file` - log file path
- `--log-level` - log level
- `-q, --quiet` - mute console logs

---

## Data Flow

### Adding a Grocery Item
1. `addGrocery` command → `storage.NewJSONFileStorage()`
2. Checks `groceries.json` for duplicates (case-insensitive)
3. Calls `GroceryEmbedder.CreateGroceryEmbedding(item)`:
   - Resolves `api_key` from viper, falling back to the `--api-key` / `-a` flag
   - Constructs `provider.NewOpenRouterProvider(apiKey)`
   - Wraps the provider in `embedding.NewGroceryEmbedder(client, "openai/text-embedding-3-small")`
   - Calls `Provider.Embed(ctx, item, model)` → `[]float32`
4. Saves `GroceryItem{Name, Embedding}` to `groceries.json`

### Scraping Flyers
1. `scrape-flyers <postal-code>` → `flipp.NewClient()`
2. Queries Flipp API for valid flyers near postal code
3. Filters to whitelisted retailers (Superstore, Walmart, etc.)
4. Stores flyers + items in JSON cache

### Finding Deals
1. `find-deals` → loads groceries + flyers from storage
2. Calls `NewOpenRouterProvider`, passes the client and `openrouter/free` to `NewPromptWriter`
3. Matching still goes through `Provider.Send` and the decoded `[]GroceryFlyerMatch`; the writer validates matches against real flyer rows
4. Outputs matched deals

---

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config management |
| `go.uber.org/zap` | Structured logging |
| `github.com/OpenRouterTeam/go-sdk` | LLM API (OpenRouter; used by `find-deals` and `add-grocery`) |
| `github.com/openai/openai-go/v3` | LLM API (OpenAI; embeds `openai/text-embedding-3-small` via OpenRouter) |
| `github.com/liliang-cn/ollama-go` | LLM API (Ollama) |
| `github.com/h2non/gentleman` | HTTP client (Flipp API) |
| `github.com/kaptinlin/jsonrepair` | JSON repair |

---

## Common Issues & Fixes

### Build Fails: `nlohmann/json_fwd.hpp` not found
- Ensure Makefile uses `-I./llama.cpp/vendor/nlohmann` (not `3rdparty/json`)
- Run `make clean && make` to force fresh clone + submodule init

### llama.cpp Logs Spamming Output
- Makefile includes `-DLLAMA_DISABLE_LOGS=1` in CFLAGS/CXXFLAGS
- Use `--quiet` flag to also suppress stfg's own logs

> Note: `tcpipuk/llama-go` is no longer imported by Go code. `make` still
> builds llama.cpp because the Makefile is the documented build entrypoint;
> the artifacts are unused by the current Go packages.

---

## Documentation Index (`docs/`)

| File | Purpose |
|---|---|
| `ARCHITECTURE.md` | Code architecture: structure, diagrams, mermaidjs (ephemeral - may change) |
| `DECISIONS.md` | Important decisions / deviations / non-intuitive choices for future context |
| `PROJECT_CONTEXT.md` | Product vision & desired outcome (never edit - represents where we're heading) |
| `docs/investigations/` | Research / transcripts (never edit after written) |
| `docs/plans/` | Archive of plan documents (not always 100% code-sync)

---

## Development Notes

- **No tests yet** - add via `go test ./...`
- **Logger**: `zap.S()` for sugar, configured in `root.go:PersistentPreRunE`
- **Cache dir**: `internal.CacheDir()` (OS-specific: `~/.cache/stfg` on Linux)
- **Embeddings**: `GroceryEmbedder.CreateGroceryEmbedding` calls `Provider.Embed`; the model is fixed at `openai/text-embedding-3-small` for `add-grocery`. Vector length is whatever the model returns.

---

## File Locations Quick Reference

| Need | Location |
|------|----------|
| Add new CLI command | `cmd/newCommand.go` + register in `init()` |
| Change the embedding model or provider used by `add-grocery` | `cmd/addGrocery.go` (model string) |
| Change storage format | `internal/storage/jsonfilestorage.go` |
| Add retailer to whitelist | `cmd/scrapeFlyers.go:validFlyers` |
| Adjust llama.cpp build flags | `Makefile:15` (CFLAGS/CXXFLAGS) |
| Config/logging setup | `cmd/root.go` |
| Structured match types | `internal/provider/types.go` |
| Match schema and decoder | `internal/provider/utils.go` |
| Prompt writer type | `internal/promptwriter/prompt_writer.go` |
| Grocery embedding entry point | `internal/embedding/grocery_embedder.go` |