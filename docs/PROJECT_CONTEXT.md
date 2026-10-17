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
| List groceries | `stfg groceries list-groceries` | Shows all items in the grocery list. |
| Remove grocery item | `stfg groceries remove-grocery <item>` | Removes an item by name (case-insensitive). |
| Scrape flyers | `stfg scrape-flyers <postal-code>` | Fetches flyers from Flipp API for the given postal code, filtered to a whitelist of retailers. |
| Find deals | `stfg find-deals` | Matches grocery list items against scraped flyer items using an LLM (OpenRouter) and prints the best deals. |

---

## Technical Stack

| Layer | Technology |
|-------|------------|
| Language | Go 1.27 |
| CLI Framework | Cobra |
| Logging | Zap (JSON file + console stderr) |
| Configuration | Viper (YAML, env vars `STFG_*`, CLI flags) |
| Storage | JSON files in the OS cache dir. SQLite and Bolt implement the same storage contract but are not selected by the commands. |
| Embeddings | Hosted model through `provider.Provider.Embed` |
| Flyer Source | Flipp API |
| Deal matching | LLM prompt through OpenRouter, not a vector search |

---

## Configuration

**Config file:** `~/.stfg.yaml` (or `--config` flag)

**Flags:** `--config`, `--log-file`, `--log-level`, `--quiet`. Viper maps those flags and reads `STFG_*` env vars. Nested keys use underscores (`STFG_PROVIDERS_OPENROUTER_API_KEY`, `STFG_FLY_FINDER_WHITELIST`).

Retailer filtering defaults to `fly_finder.whitelist`. Model names are hardcoded in the commands, not in the config file.

---

## Data Model

- **GroceryItem** — `Name`, `Embedding` (float32 vector)
- **Flyer** — `ID`, `Merchant`, `Name`, `ValidFrom`, `ValidTo`, `Stores`
- **FlyerItem** — `ID`, `FlyerID`, `Name`, `Brand`, `Price`, `ImageURL`, `VideoURL`, `DisplayType`
- **Store** — address fields for a flyer, including `PostalCode`

---

## Build & Run

```bash
make
./bin/stfg groceries add-grocery milk
./bin/stfg groceries list-groceries
./bin/stfg scrape-flyers V5K0A1
./bin/stfg find-deals
```

`make` compiles `cmd/stfg/main.go` to `./bin/stfg`. It does not build llama.cpp.

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