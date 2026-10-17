# Decision Log

Choices that are easy to miss from a quick read of the code.

## Composition root, not `init()`

**Decision:** `cmd/stfg/main.go` builds the Cobra root, registers loaders, and attaches subcommands. Command files export constructors. They do not register themselves in `init()`.

**Reason:** The previous `init()` tree hid wiring and made the binary entrypoint a side effect of imports. The container is created once in `main` and passed into constructors. It is not stuffed into the Cobra context.

## Registry keys, not a factory switch

**Decision:** `container.ContainerRegistry` maps a string key to a loader. Callers use `GetStorage` and `GetProvider`. Constructors return concrete types (`*json.FileStorage`, `*provider.OpenRouterProvider`). Registry methods return `storage.StorageProvider` and `provider.Provider`.

**Reason:** New backends are added by registration in `internal/bootstrap/providers.go`, not by growing a switch. Commands still hardcode `"json"` and `"openrouter"`. SQLite, Bolt, OpenAI, and Ollama are registered so they can be selected later without a new abstraction.

## Storage contract

**Decision:** One `StorageProvider` in `internal/storage/protocols.go` covers groceries, flyers, and flyer items. There is no `internal/storage/storage.go`. JSON is the backend commands use. Cache files live in `os.UserCacheDir()/stfg`.

**Reason:** Flyer and grocery access used to be split. One interface lets the scrape reconciler and the commands share a backend. JSON stays the default because the files are inspectable. SQLite and Bolt exist for the same contract and are covered by the parity tests.

**Decision:** Case-insensitive grocery identity uses `strings.ToLower` via `storage.NormalizeName`, shared by all three backends.

**Reason:** `strings.EqualFold` treats some Unicode pairs as equal that `ToLower` does not. The parity tests pin the `ToLower` behavior.

## Hosted embeddings, llama.cpp removed from the build

**Decision:** Grocery vectors come from `Provider.Embed`. The local Qwen GGUF / llama.cpp path is gone. `make` is `go build -o ./bin/stfg cmd/stfg/main.go`. Commented llama.cpp rules in the Makefile are leftovers, not the build. `make run` still points at a deleted root `main.go` and should not be used.

**Reason:** llama-go was only used to embed grocery names. The provider interface already had `Embed`. Stored vectors from the old model are not migrated; add the items again before comparing them to a new model.

**Decision:** `add-grocery` hardcodes `nvidia/nemotron-3-embed-1b:free` in the command, not in `internal/embedding` and not in Viper.

**Reason:** The embedder should not pick a vendor or a model. The command that already chose OpenRouter owns the model string.

## Deal matching is a prompt, not a vector search

**Decision:** `find-deals` does not read `GroceryItem.Embedding`. `PromptWriter` takes any `provider.Provider` plus a model name. The command passes OpenRouter and `openai/gpt-4.1-nano`.

**Reason:** After `Send` returned `[]GroceryFlyerMatch`, a local prompter interface duplicated `provider.Provider`. Leaving provider and model selection in the command avoids a factory. Vector search is not what this command does, even though embeddings are stored.

**Decision:** The prompt includes flyer `id`, `brand`, `name`, and `displayType`. It does not send image URLs. The writer accepts a match only when flyer id, flyer name, and grocery name all exist on the inputs, and it retries that check up to three times.

**Reason:** The model must not invent rows. Image URLs do not help that check and inflate the prompt.

## Structured provider matches

**Decision:** Every `Send` implementation asks for one JSON object, `{"matches":[...]}`, and returns `[]GroceryFlyerMatch`. Decoding sits next to the schema in `internal/provider/utils.go`.

**Reason:** OpenAI strict structured outputs need a root object. Callers still receive the inner slice. A generic `Send[T]` was rejected because this program has one response shape.

## OpenRouter client

**Decision:** The OpenRouter loader requires `providers.openrouter.api_key` (env `STFG_PROVIDERS_OPENROUTER_API_KEY`). The client timeout is 3 minutes, set in `RegisterProviders`. `Send` sets provider `RequireParameters`. The API key is not logged.

**Reason:** A missing key should fail at lookup instead of sending an empty credential. The timeout is a client default, not a per-call context deadline. `RequireParameters` tells OpenRouter to reject a route that cannot satisfy the request.

## Flipp whitelist

**Decision:** Preferred retailers are the Viper default `fly_finder.whitelist`, overridable from `~/.stfg.yaml`. `scrape.Reconcile` keeps a flyer when that list's entry is a substring of `merchant + " " + name`.

**Reason:** The whitelist is configuration, not a slice in the command file. Substring matching tolerates Flipp names that include extra words around the retailer.

## Logging and stdout

**Decision:** Zap JSON goes to `stfg.log` unless `--log-file` is set. Console Zap goes to the command stderr. Results go to stdout. Cobra usage and error printing are silenced. `main` logs the `Execute` error once on stderr.

**Reason:** Stdout has to stay pipeable. Logging the same failure in a library and again at the boundary double-prints it.

## Configuration shape

**Decision:** There is no top-level `api_key` and no `--api-key` flag. Provider and storage settings are nested Viper keys under `providers.*` and `storage.*`, with `STFG_` env overrides.

**Reason:** More than one provider and more than one backend need their own settings. A single `api_key` flag could not name which provider it belonged to.
