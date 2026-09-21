# Project Context — stfg (Shop The Flyers Grocer)

**Product Vision:** Build a command-line tool that helps users find the best grocery deals in their area by matching their personal grocery list against flyers scraped from local retailers.

**Desired Outcome:** A user can:
1. Maintain a personal grocery list (add/remove/list items).
2. Scrape flyers from retailers near their postal code.
3. Run a semantic search to find which flyer items match their grocery list, surfacing the best prices.

---

## Target User

Anyone who wants to save money on groceries by comparing weekly flyer deals against their shopping list — without manually browsing multiple store flyers.

---

## Core Features

| Feature | Command | Description |
|---------|---------|-------------|
| Add grocery item | `stfg groceries add-grocery <item>` | Adds an item with a generated embedding. Duplicate detection is case-insensitive. |
| List groceries | `stfg groceries list` | Shows all items in the grocery list. |
| Remove grocery item | `stfg groceries remove-grocery <item>` | Removes an item by name (case-insensitive). |
| Scrape flyers | `stfg scrape-flyers <postal-code>` | Fetches flyers from Flipp API for the given postal code, filtered to a whitelist of retailers. |
| Find deals | `stfg find-deals` | Matches grocery list items against scraped flyer items using an LLM (OpenRouter) and prints the best deals. |

---

## Technical Stack

| Layer | Technology |
|-------|------------|
| Language | Go 1.26+ |
| CLI Framework | Cobra |
| Logging | Zap (structured JSON + console) |
| Configuration | Viper (YAML, env vars `STFG_*`, CLI flags) |
| Storage | JSON files (OS cache dir), optional SQLite, BoltDB |
| Embeddings | llama.cpp via `tcpipuk/llama-go` (Qwen3-Embedding-0.6B-GGUF, Q8_0) |
| Flyer Source | Flipp API |
| LLM Provider | OpenRouter (configurable model) |

---

## Configuration

**Config file:** `~/.stfg.yaml` (or `--config` flag)

**Environment variables (prefix `STFG_`):**
- `STFG_CONFIG` — config file path
- `STFG_LOG_FILE` — log output file
- `STFG_LOG_LEVEL` — log level (debug/info/warn/error)
- `STFG_QUIET` — suppress console output

**Flags:**
- `--config` — config file path
- `--log-file` — log file path
- `--log-level` — log level
- `-q, --quiet` — mute console logs

---

## Data Model

- **GroceryItem** — `Name`, `Embedding` (float32 vector)
- **Flyer** — `ID`, `Merchant`, `Name`, `ValidFrom`, `ValidTo`, `PostalCode`
- **FlyerItem** — `FlyerID`, `Name`, `Brand`, `Price`, `Category`

---

## Build & Run

```bash
# Full build (clones llama-go, builds llama.cpp, compiles stfg)
make

# Run commands
./bin/stfg groceries add-grocery milk
./bin/stfg groceries list
./bin/stfg scrape-flyers V5K0A1
./bin/stfg find-deals
```

---

## Roadmap / Future Enhancements

- Web UI for managing the grocery list and viewing deals
- Support for more flyer sources (e.g., Reebee, store-specific APIs)
- Notification system (email, push) when deals match
- Price history tracking and trend analysis
- Integration with shopping list apps

---

## Non-Goals

- No mobile app (CLI-first)
- No direct purchasing integration
- No user accounts / cloud sync (local-only for now)