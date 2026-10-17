# stfg — Shop The Flyers Grocer

CLI for keeping a grocery list, scraping retailer flyers, and matching that list to flyer deals.

## Prerequisites

- Go 1.27 (see `go.mod`)
- An OpenRouter API key for `add-grocery` and `find-deals`
- `make`

## Setup

```bash
git clone git@github.com:bensoer/stfg.git
cd stfg
make
```

`make` builds `./bin/stfg` from `cmd/stfg/main.go`. It does not clone llama.cpp or link CGO libraries. `make run` still calls a removed root `main.go` and is not a working entrypoint. `make clean` deletes `bin/`, `res/`, and `lib/`.

Copy or create `~/.stfg.yaml` and set the OpenRouter key:

```yaml
providers:
  openrouter:
    api_key: sk-or-...
```

The same key can be set with `STFG_PROVIDERS_OPENROUTER_API_KEY`.

## Commands

```bash
./bin/stfg groceries add-grocery milk
./bin/stfg groceries list-groceries
./bin/stfg groceries remove-grocery milk
./bin/stfg scrape-flyers V5K0A1
./bin/stfg find-deals
```

Command results go to stdout. Logs go to `stfg.log` in the working directory and, unless `--quiet` is set, to stderr.

Grocery and flyer data for the JSON backend (the backend the commands use) is stored under the OS cache directory, `~/.cache/stfg` on Linux.

## Tests

```bash
go test ./...
```

## Docs

Start with [AGENTS.md](AGENTS.md) for repository navigation. [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) is the current code snapshot. [docs/PROJECT_CONTEXT.md](docs/PROJECT_CONTEXT.md) is the product goal.
