# AGENTS.md - stfg Project Guide

## Project Overview

**stfg (Shop The Flyers Grocer)** - CLI tool for searching grocery deals from flyers and stores.

- **Language**: Go 1.26+
- **Framework**: Cobra CLI
- **Logging**: Zap (structured JSON + console)
- **Config**: Viper (YAML, env vars, flags)
- **Storage**: JSON files in OS cache dir
- **ML**: llama.cpp via `tcpipuk/llama-go` for embeddings
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
├── cmd/                    # Cobra commands
│   ├── root.go            # Root command, config, logging setup
│   ├── groceries.go       # Parent command for grocery ops
│   ├── addGrocery.go      # Add item with embedding
│   ├── listGroceries.go   # List all items
│   ├── removeGrocery.go   # Remove item
│   ├── scrapeFlyers.go    # Scrape flyers via Flipp API
│   └── findDeals.go       # Match flyers to grocery list (semantic search)
├── internal/
│   ├── models/            # Embedding model handling
│   │   ├── client.go      # llama-go integration, CreateGroceryEmbedding()
│   │   ├── resolver.go    # Download/resolve GGUF models from HF
│   │   └── types.go       # Model constants (Qwen3-Embedding-0.6B)
│   ├── storage/           # JSON file persistence
│   │   ├── db.go          # Low-level JSON save/load + cache dir
│   │   ├── jsonfilestorage.go  # High-level Grocery/Flyer CRUD
│   │   └── types.go       # Store, Flyer, FlyerItem, GroceryItem types
│   ├── flipp/             # Flipp API client
│   ├── reconciler/        # Flyer scraping & matching logic
│   ├── provider/          # LLM providers (OpenRouter)
│   └── promptwriter/      # Prompt templates
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
3. Calls `models.CreateGroceryEmbedding(item)`:
   - Resolves/downloads `Qwen3-Embedding-0.6B-Q8_0.gguf` to `models/`
   - Loads model with llama-go (GPU layers=-1, mmap, silent loading)
   - Creates context (1024 tokens, embeddings enabled)
   - Runs `ctx.GetEmbeddings(grocery)` → `[]float32`
4. Saves `GroceryItem{Name, Embedding}` to `groceries.json`

### Scraping Flyers
1. `scrape-flyers <postal-code>` → `flipp.NewClient()`
2. Queries Flipp API for valid flyers near postal code
3. Filters to whitelisted retailers (Superstore, Walmart, etc.)
4. Stores flyers + items in JSON cache

### Finding Deals
1. `find-deals` → loads groceries + flyers from storage
2. Uses embeddings for semantic matching
3. Outputs matched deals

---

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config management |
| `go.uber.org/zap` | Structured logging |
| `github.com/tcpipuk/llama-go` | llama.cpp Go bindings |
| `github.com/OpenRouterTeam/go-sdk` | LLM API (OpenRouter) |
| `github.com/h2non/gentleman` | HTTP client (Flipp API) |
| `github.com/kaptinlin/jsonrepair` | JSON repair |

---

## Common Issues & Fixes

### Build Fails: `nlohmann/json_fwd.hpp` not found
- Ensure Makefile uses `-I./llama.cpp/vendor/nlohmann` (not `3rdparty/json`)
- Run `make clean && make` to force fresh clone + submodule init

### llama.cpp Logs Spamming Output
- Makefile includes `-DLLAMA_DISABLE_LOGS=1` in CFLAGS/CXXFLAGS
- `models/client.go` sets `LLAMA_LOG=none` at init
- Use `--quiet` flag to also suppress stfg's own logs

### Model Download Fails
- Check network access to HuggingFace
- Model: `Qwen/Qwen3-Embedding-0.6B-GGUF` (Q8_0 quantization)
- Stored in `models/Qwen3-Embedding-0.6B-Q8_0.gguf`

---

## Development Notes

- **No tests yet** - add via `go test ./...`
- **Logger**: `zap.S()` for sugar, configured in `root.go:PersistentPreRunE`
- **Cache dir**: `internal.CacheDir()` (OS-specific: `~/.cache/stfg` on Linux)
- **Embeddings**: 1024 context size (Qwen supports 32768 but trimmed for memory)
- **Silent loading**: `llama.WithSilentLoading()` + `LLAMA_DISABLE_LOGS=1`

---

## File Locations Quick Reference

| Need | Location |
|------|----------|
| Add new CLI command | `cmd/newCommand.go` + register in `init()` |
| Modify embedding model | `internal/models/types.go` |
| Change storage format | `internal/storage/jsonfilestorage.go` |
| Add retailer to whitelist | `cmd/scrapeFlyers.go:validFlyers` |
| Adjust llama.cpp build flags | `Makefile:15` (CFLAGS/CXXFLAGS) |
| Config/logging setup | `cmd/root.go` |