# AGENTS.md — stfg

Index for working in this repository. Current code shape lives in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). Non-obvious choices live in [docs/DECISIONS.md](docs/DECISIONS.md). Product goal lives in [docs/PROJECT_CONTEXT.md](docs/PROJECT_CONTEXT.md) — do not rewrite that vision for a refactor. [docs/investigations/](docs/investigations/) and [docs/plans/](docs/plans/) are historical. Do not edit an investigation after it is written. Plans are not authoritative.

## What this is

`stfg` is a Go CLI that stores a grocery list, scrapes Flipp flyers for a postal code, and asks an LLM which flyer rows match the list.

- Language: Go 1.27 (`go.mod`)
- CLI: Cobra, wired in `internal/bootstrap` (no command `init()`)
- Config: Viper
- Logging: Zap
- Storage: `storage.StorageProvider` with JSON, SQLite, and Bolt backends. Commands look up `"json"`.
- LLM: `provider.Provider` (OpenRouter, OpenAI, Ollama). Commands look up `"openrouter"`.
- Flyers: `flyerfinder.FlyerFinder`, implemented by `internal/flyerfinder/flipp`

## Layout

```
cmd/stfg/main.go                 # composition root: bootstrap, register, Execute
internal/bootstrap/              # Cobra root, config, logging, provider registration, subcommand wiring
internal/commands/               # one Cobra command constructor per file
internal/container/              # string-keyed loader registry
internal/storage/                # StorageProvider, DTOs, cache dir
internal/storage/json|sqlite|bolt/
internal/provider/               # Provider, match schema, OpenRouter/OpenAI/Ollama
internal/embedding/              # GroceryEmbedder over Provider.Embed
internal/cosinecompare/          # CosineCompare over vek32.CosineSimilarity for the in-memory prefilter
internal/promptwriter/           # match prompt, Send, validate returned rows
internal/flyerfinder/            # FlyerFinder protocol + flipp client
internal/reconciler/scrape/      # whitelist filter and flyer persistence
docs/                            # living docs, investigations, plans
```

There is no root `main.go`, no `cmd/*.go` command package, and no `internal/flipp` or `internal/models` package.

## Build and run

```bash
make                  # go build -o ./bin/stfg cmd/stfg/main.go
go test ./...
./bin/stfg groceries add-grocery milk
./bin/stfg groceries list-groceries
./bin/stfg groceries remove-grocery milk
./bin/stfg scrape-flyers V5K0A1
./bin/stfg find-deals
```

Do not use `make run`. It still executes `go run main.go`, which no longer exists. Llama.cpp targets in the Makefile are commented out and are not part of the build.

## Configuration

Config file: `$HOME/.stfg.yaml`, or `--config`. Viper prefix is `STFG`. Dots and dashes in keys become underscores in env vars (`providers.openrouter.api_key` → `STFG_PROVIDERS_OPENROUTER_API_KEY`).

Persistent flags, bound into Viper before config load: `--config`, `--log-file`, `--log-level` (default `info`), `--quiet`.

Defaults set in `internal/bootstrap/config.go`:

| Key | Default |
|-----|---------|
| `fly_finder.whitelist` | Superstore, Thrifty Foods, Quality Foods, Buy-Low Foods, Country Grocer, No Frills, Pharmasave, Shoppers Drug Mart, Walmart, Nesters Market, Rexall |
| `providers.openrouter.api_key` | empty; the OpenRouter loader fails if it stays empty |
| `providers.openai.api_key` | empty |
| `providers.ollama.host` | `http://localhost:11434` |
| `storage.sqlite.sqlite_file_name` | `stfg.sqlite.db` |
| `storage.bolt.bolt_file_name` | `stfg.bolt.db` |

Empty `storage.sqlite.cache_dir` and `storage.bolt.cache_dir` fall back to `storage.CacheDir()` (`os.UserCacheDir()/stfg`, mode `0700`). JSON always uses that cache dir. Files: `groceries.json`, `flyers_index.json`, `flyer_<id>.json`.

Logs default to `stfg.log` in the process working directory (JSON). Console logs use a console encoder on the command's stderr unless `--quiet` is set. `SilenceErrors` and `SilenceUsage` are on. `main` prints one `error:` line to stderr and exits 1.

## How a run is wired

1. `bootstrap.BootstrapCobraRoot()` builds the root command and `PersistentPreRunE` (config, then logger, then `zap.ReplaceGlobals`).
2. `bootstrap.RegisterProviders` builds a `container.ContainerRegistry` and registers storage keys `json`, `sqlite`, `bolt` and provider keys `openrouter`, `ollama`, `openai`. Loaders run on lookup, not at registration. Constructors return concrete types; the registry methods return the consumer interfaces.
3. `bootstrap.RegisterSubCommands` passes that registry into command constructors. The registry is not stored on the Cobra context.
4. Each command calls `GetStorage(ctx, "json")` and, when it needs a model, `GetProvider(ctx, "openrouter")`.

## Command behavior

`add-grocery` rejects a case-insensitive duplicate, embeds the name, and stores `GroceryItem`. The embedder model string is the package-level constant `groceryEmbeddingModel` in `internal/commands/addGrocery.go` (`nvidia/nemotron-3-embed-1b:free`).

`scrape-flyers` looks up `"openrouter"` and the JSON store, builds `embedding.NewGroceryEmbedder` with `groceryEmbeddingModel`, and passes it on `scrape.ScrapeReconcilerOptions.Embedder`. A flyer is kept when `merchant + " " + name` contains a whitelist entry. Expired flyers are pruned first. The reconciler embeds each new or changed flyer item at scrape time: `text = strings.TrimSpace(Brand + " " + Name)`, embed when the item is new, the stored embedding is empty, or the text differs from what is stored. Empty text is not embedded and is persisted as a nil embedding. Existing flyer IDs are not re-inserted, but their items are still reconciled (the old `HasFlyer` skip is gone). Embed errors abort the scrape and leave already-written rows on disk; the next scrape backfills them. The reconciler logs one info line per flyer with the count of items it actually embedded.

`find-deals` does not embed. It loads JSON groceries and flyers, runs the 60% cosine prefilter on stored `FlyerItem.Embedding` vs. stored `GroceryItem.Embedding` through `cosinecompare.CosineCompare` (`vek32.CosineSimilarity`), and sends the survivors to `promptwriter.NewPromptWriter` with `openai/gpt-4.1-nano`. Matching is an LLM prompt, not a vector search. The prompt sends flyer `id`, `brand`, `name`, and `displayType` only. The writer retries up to three times when a returned match cannot be tied to a real flyer row and grocery name. A grocery with no embedding is an error that tells the user to re-add it with `groceries add-grocery`; a flyer item whose text is non-empty but whose stored embedding is nil is an error that tells the user to re-run `scrape-flyers`. Empty embedding text is skipped silently.

## Where to change things

| Need | Location |
|------|----------|
| Add a command | `internal/commands/`, then `internal/bootstrap/commands.go` |
| Register a storage or LLM backend | `internal/bootstrap/providers.go` |
| Select which backend a command uses | the `GetStorage` / `GetProvider` key in that command |
| Embedding model string (grocery and flyer) | `internal/commands/addGrocery.go` (`groceryEmbeddingModel`) |
| Match model string | `internal/commands/findDeals.go` (`openai/gpt-4.1-nano`) |
| Cosine prefilter percent and library | `internal/cosinecompare/cosine_compare.go`, plus the wiring in `internal/commands/findDeals.go` |
| Match schema and decode | `internal/provider/types.go`, `internal/provider/utils.go` |
| Prompt text and match checks | `internal/promptwriter/prompt_writer.go` |
| Retailer whitelist default | `internal/bootstrap/config.go` (`fly_finder.whitelist`) |
| Flipp HTTP calls | `internal/flyerfinder/flipp/` |
| Cache directory | `internal/storage/utils.go` |

## Docs map

| Path | Role |
|------|------|
| `docs/ARCHITECTURE.md` | Current structure and flow. Update when the code moves. |
| `docs/DECISIONS.md` | Choices that are hard to see from the code. |
| `docs/PROJECT_CONTEXT.md` | Product goal. Do not turn it into a changelog. |
| `docs/investigations/` | Past research. Leave it after it is written. |
| `docs/plans/` | Archived plans. Not a guarantee of what shipped. |
