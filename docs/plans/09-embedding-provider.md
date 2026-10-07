# Plan 9 — Provider-backed grocery embeddings

**Status:** Ready to execute. This file is the spec.
**Repo root:** `/home/bsoer/Documents/PROJECTS/VSCodeProjects/stfg`
**Module:** `stfg`
**Go:** 1.26.5
**Remote:** `git@github.com:bensoer/stfg.git` (`origin`)
**SDKs already required:** `github.com/openai/openai-go/v3` v3.64.0, `github.com/liliang-cn/ollama-go` v0.2.1, `github.com/OpenRouterTeam/go-sdk` v0.5.18. Do not bump them.

Execute in a new worktree. Finish with commits, a push, and a pull request. Do not expand the product. Do not implement Plan 7 or Plan 8. Do not edit `docs/PROJECT_CONTEXT.md`, `README.md`, `Makefile`, flyer scraping, storage, or the reconciler. The only `cmd/` edit is `cmd/addGrocery.go`, specified below.

## Operator decision

Confirmed: wire `add-grocery` to OpenRouter. Hardcode model `openai/text-embedding-3-small` in that command only. Reuse the existing `api_key` / `--api-key` lookup. Do not add a provider switch, a model flag, or a second config key.

The Plan 8 `provider.Provider` fake will not compile once `Embed` exists. Add an `Embed` stub on that fake. Do not change its other behavior. No other files outside `internal/embedding`, `internal/provider`, `cmd/addGrocery.go`, docs, and `go.mod` / `go.sum`.

## Where to execute

Do not edit the primary checkout except to copy this file. Leave it after the copy.

```bash
git fetch origin
```

Pick the base. Plan 7 marker: `decodeGroceryFlyerMatches` in `internal/provider/types.go`. Plan 8 marker: `type PromptWriter struct` in `internal/promptwriter/prompt_writer.go`.

- If both markers are on `origin/main`, base is `origin/main`. PR base is `main`.
- Else if `origin/promptwriter-provider` has both markers, base is that branch. PR base is `promptwriter-provider`.
- Else if `origin/structured-provider-matches` has the Plan 7 marker, base is that branch. PR base is `structured-provider-matches`.
- Else stop. Do not branch from local `main`. Do not implement against `Send(...) (*string, error)`.

No ticket was provided. Do not ask. Branch: `embedding-provider`.

```bash
git worktree add -b embedding-provider /home/bsoer/Documents/PROJECTS/stfg-embedding <base>
mkdir -p /home/bsoer/Documents/PROJECTS/stfg-embedding/docs/plans
cp /home/bsoer/Documents/PROJECTS/VSCodeProjects/stfg/docs/plans/09-embedding-provider.md \
  /home/bsoer/Documents/PROJECTS/stfg-embedding/docs/plans/09-embedding-provider.md
```

If that path exists, stop. Do not reuse or delete it. All edits, tests, commits, and the PR happen in the worktree.

Git: never amend, never force-push, never rebase. A bad commit gets a new commit. Stay under 500 changed lines per commit. Do not commit `res/`, `lib/`, `bin/`, or `models/`.

## Outcome

`internal/models` is gone. `internal/embedding` has one type, `GroceryEmbedder`, that stores a `provider.Provider` and a model name and returns one `[]float32` for one grocery string.

`Provider` gains `Embed`. OpenAI, Ollama, and OpenRouter implement it with the SDKs already in `go.mod`. `Send` is unchanged.

## Rules

- No generics. No functional options. No provider factory or `switch`.
- Constructors return `(*Struct, error)`, not an interface.
- `context.Context` is the first parameter. Do not store a context. Do not nil-check it.
- Return the vector by value. Do not fill a caller pointer.
- Function bodies go on the line below the signature. Wrap new errors with `fmt.Errorf("...: %w", err)`.
- Do not log in new code. Do not log and return. Libraries do not panic or call `Fatal`.
- Do not restyle unrelated code. `gofmt` only files you touch. Do not reformat `Send`.
- Read each worktree file before editing. Keep every existing comment verbatim, including stale wording. Move comments with the deleted llama code as specified below. Do not reword them.
- Do not edit `internal/provider/errors.go`. Reuse `ValidationError` and `ModelResponseError`.
- Same-package tests. Hand-written fake only. No network, no API key, no llama.cpp, no SDK mock, no new test-only export.
- Do not import `embedding` from `provider`, or `storage` / `promptwriter` / `cmd` from `embedding`.

## Current state

`internal/models/client.go` exports `CreateGroceryEmbedding(grocery string) ([]float32, error)`. It downloads a GGUF, loads llama.cpp, and calls `ctx.GetEmbeddings`. It logs and returns. `resolver.go` and `types.go` exist only for that path. Nothing else in Go imports `stfg/internal/models` except `cmd/addGrocery.go`.

`storage.GroceryItem.Embedding` is `[]float32`. Do not change that type.

Plan 7 makes `Send(ctx context.Context, prompt string, model string) ([]GroceryFlyerMatch, error)`. Confirm that signature in the worktree. If it differs, stop. Do not edit the `Send` line, its comment, `decodeGroceryFlyerMatches`, or the match schema.

## Provider contract

In `internal/provider/protocols.go`, keep the existing comments and the `Send` line. Add:

`Embed(ctx context.Context, text string, model string) ([]float32, error)`

New doc comment, copied onto the interface method and all three implementations: `Embed returns one float32 embedding for text from model.`

One text in, one vector out. No batch, no dimensions argument, no images.

Each implementation:

- Reject `text == ""` and `model == ""` with `ValidationError` (`Field` `text` or `model`, `Msg` `empty`). Do not trim.
- `ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)` then `defer cancel()`. Do not call `context.Background()`. Do not change `Send` if it still does.
- Transport failures: `fmt.Errorf("<name> embed: %w", err)`.
- Empty, wrong count, non-float, NaN, or Inf payload: `ModelResponseError` with `Provider` set to `openai`, `ollama`, or `openrouter` and `Raw` set to a short reason. Do not return a partial vector.
- Require exactly one vector. Do not silently take index 0 from a longer list.

Put `toFloat32Vector([]float64) ([]float32, error)` in `internal/provider/utils.go`. All three call it. Reject empty, NaN, and Inf.

### OpenAI

`op.Client.Embeddings.New`. `EmbeddingNewParams`: `Model` is the argument, `Input.OfString` is `openai.String(text)`, `EncodingFormat` is `EmbeddingNewParamsEncodingFormatFloat`. Read `res.Data`. Use `Data[0].Embedding` (`[]float64` in v3.64.0). Keep the existing client field. Do not add a constructor check.

### Ollama

Copy the `ollama.NewClient(WithHost, WithHTTPClient)` block from the worktree `Send` into `Embed`. Do not refactor `Send` to share it. Call `client.Embed(ctx, &ollama.EmbedRequest{Model: model, Input: text})`. Use `resp.Embeddings`, not the legacy `Embeddings` method and not package-level `ollama.Embed` (that ignores `Host`).

### OpenRouter

`op.client.Embeddings.Generate(ctx, operations.CreateEmbeddingsRequest{...})`. `Input` is `operations.CreateInputUnionStr(text)`. `EncodingFormat` is `operations.EncodingFormatFloat.ToPointer()`. Read `res.CreateEmbeddingsResponseBody.Data[0].Embedding`. Accept only `EmbeddingTypeArrayOfNumber` / `ArrayOfNumber`. A string union or base64 embedding is `ModelResponseError`. Do not decode base64.

Do not set `Dimensions`. Do not pull or download a model. A missing model is the wrapped SDK error.

## Embedding type

`internal/embedding/grocery_embedder.go`:

- `GroceryEmbedder` holds a named `provider.Provider` and a `model string`. Do not embed the provider.
- `NewGroceryEmbedder(p provider.Provider, model string) (*GroceryEmbedder, error)`.
  - `p == nil` → `fmt.Errorf("grocery embedder: provider is nil")`.
  - `model == ""` → `fmt.Errorf("grocery embedder: model is empty")`.
  - Do not trim. Do not use reflection to catch a typed nil. No network or file I/O.
- `CreateGroceryEmbedding(ctx context.Context, grocery string) ([]float32, error)`.
  - `grocery == ""` → `fmt.Errorf("grocery embedder: grocery is empty")`. Do not call `Embed`.
  - Otherwise `p.Embed(ctx, grocery, model)` and wrap a failure as `fmt.Errorf("create grocery embedding: %w", err)`.
- Doc comments on the exported type, constructor, and method. Those are new. Do not invent a second interface.

Delete `internal/models` after the comments below are pasted, verbatim and in this order, immediately above the `Embed` call:

```
// 32768 is Qwen's max, but since were only generating embeddings for couple words up to a sentence.
// 1024 is plenty. This also should save on users memory usage
// Create context with embedding support
//fmt.Printf("Model loaded successfully.\n")
//fmt.Printf("Getting embeddings for: %s\n", *text)
// Check if models/${modelName}.gguf exists
// return the absolute path
// Ensure destination directory exists
// Check if partial file exists
// Resume download
// If server ignored Range request, restart
```

Do not "fix" those comments. They are stale on purpose.

`go.mod`: after the llama import is gone, `go mod tidy`. Drop `github.com/tcpipuk/llama-go` if nothing else imports it. Do not remove the OpenAI, Ollama, or OpenRouter modules.

## add-grocery wiring

Edit only `cmd/addGrocery.go`. Read the worktree `cmd/findDeals.go` first and copy its OpenRouter construction. After Plan 7 that is `provider.NewOpenRouterProvider`, not `provider.NewClient`. If the worktree still calls `NewClient`, stop.

- Keep the existing log lines verbatim: `Getting Embedding Value For Grocery` and `Failed To Create Grocery Embedding Data`.
- Key lookup matches find-deals: `viper.GetString("api_key")`, then `cmd.Flags().GetString("api-key")` when that string is empty.
- Register the same `--api-key` / `-a` flag on `addGroceryCmd` and `viper.BindPFlag("api_key", ...)`. Copy the find-deals flag help text verbatim. Do not add the flag to any other command. Do not rewrite other help text.
- Do not add an empty-key check that find-deals does not have.
- On constructor error, `zap.S().Error` and return. Do not continue.
- `embedding.NewGroceryEmbedder(client, "openai/text-embedding-3-small")`, then `CreateGroceryEmbedding(cmd.Context(), item)`.
- The model string stays in the command. Do not put it in `internal/embedding`.
- Do not import llama-go. Do not change success printing, duplicate checks, or `init()` beyond the flag.

## Edge cases

- Nil context is a caller bug.
- Whitespace-only grocery or model is sent as-is.
- Vector length is whatever the model returns. Do not require 1024. Do not compare vectors in this package.
- Existing stored Qwen vectors will not match a new model's vectors. No migration.
- float64 narrows to float32. That is intentional; storage stays `[]float32`.
- Duplicate calls reload nothing locally. Each `CreateGroceryEmbedding` is one provider call. No cache.
- Ollama may not have the model installed. Do not auto-pull.
- OpenRouter and OpenAI still do nothing in the constructor when the API key is empty. Failure stays on the call.
- `find-deals` does not gain an embedding call.

## Tests

`internal/embedding/grocery_embedder_test.go`, `package embedding`. Fake implements every method on the worktree `Provider`. `Send` must not be called; return an error if it is. Script `Embed`.

1. Nil provider and empty model. `Embed` not called.
2. Empty grocery. `Embed` not called.
3. Success. Recorded text, model, and ctx match the arguments. Returned slice equals the scripted `[]float32`. `Send` not called.
4. `Embed` error is wrapped with `create grocery embedding`. Called once.

Pass `context.Background()`. Do not assert on zap.

`internal/provider/utils_test.go`: empty input, one finite value, NaN, Inf. No SDK.

`internal/provider/providers_test.go`: compile-time assertions that `*OpenAIProvider`, `*OllamaProvider`, and `*OpenRouterProvider` implement `Provider`, each with a comment that the line exists so a missing `Embed` fails here.

Prompt-writer fake: add `Embed` returning `nil, fmt.Errorf("embed not used")` or the worktree's existing error style. Do not call a network. Do not edit production prompt-writer files.

## Documentation

Read the worktree docs after Plan 7 and Plan 8. Edit only the embedding facts. Do not rewrite the structured-match or prompt-writer sections.

### `docs/ARCHITECTURE.md`

- Grocery embeddings come from `GroceryEmbedder` through `Provider.Embed`, not llama.cpp.
- Data-flow step 1: `add-grocery` constructs `NewOpenRouterProvider` from `api_key` / `--api-key` and passes that provider plus `openai/text-embedding-3-small` to `NewGroceryEmbedder`. It does not select among providers.
- Directory tree: `internal/embedding/`, not `internal/models/` as the embedding implementation. The tree comment that calls `models/` the storage DTOs is already wrong; point storage DTOs at `internal/storage` and embeddings at `internal/embedding`.
- Provider bullet: the three providers also implement `Embed`. Do not remove the Plan 7 `Send` description.

### `docs/DECISIONS.md`

Do not rewrite `Embedding Model Choice`. Append a section: grocery embeddings now come from `Provider.Embed`; the llama.cpp / Qwen GGUF path is removed from Go code; stored vectors are not migrated and will not match `openai/text-embedding-3-small`; `add-grocery` hardcodes that model and OpenRouter; `Makefile` still builds llama.cpp and that build is now unused by this package.

### `AGENTS.md`

- Tree, add-grocery data flow, dependency row, file-location row, and the GGUF troubleshooting notes no longer describe `internal/models` as the embedding implementation.
- State that `CreateGroceryEmbedding` lives on `GroceryEmbedder` and that `add-grocery` calls it through OpenRouter with `openai/text-embedding-3-small`.
- Leave the Makefile / `res/` / `lib/` build notes. They are still true for `make`.

## Out of scope

- Any `cmd/` file except `cmd/addGrocery.go`. No `--provider` or `--model` flag. No edit to `find-deals`.
- Reading API keys inside `internal/embedding` or `internal/provider`. The command owns `viper`.
- `Makefile`, llama.cpp flags, deleting `res/` or `lib/`.
- Storage schema, similarity search, re-embedding saved groceries.
- Changing `Send`, match JSON, prompt text, or `find-deals`.
- Batch embeddings, dimensions, base64 decode, retries, caching, streaming.
- `make`, live model calls, or `go test ./...` if that pulls llama.cpp. Use the checks below.
- Editing `internal/provider/errors.go` or other plan files.

## Order

1. Confirm the base, create the worktree, copy this spec, `cd` there.
2. Read `protocols.go`, the three provider files, `errors.go`, and the prompt-writer test fake. List comments before editing.
3. Add `Embed` and `toFloat32Vector`. Do not touch `Send` bodies.
4. Add `grocery_embedder.go` with the moved comments. Delete `internal/models`.
5. `go mod tidy` if llama-go is unused.
6. Add the tests and the fake `Embed` stub.
7. Update `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, and `AGENTS.md`.
8. `gofmt` touched Go files. Run the checks. Commit only after they pass.

## Checks

```bash
go test ./internal/embedding/ ./internal/provider/ ./internal/promptwriter/
go build -o /dev/null ./internal/embedding/ ./internal/provider/ ./cmd/
```

`rg -n "stfg/internal/models|tcpipuk/llama-go" --glob '!docs/plans/**' -g '*.go'` must be empty. `openai/text-embedding-3-small` must appear in `cmd/addGrocery.go` and not in `internal/embedding`.

## Commits and pull request

Three commits. Tests and docs never share a commit with source.

1. Source only. No `*_test.go`. No `docs/`. No `AGENTS.md`.

```text
refactor(embedding): generate grocery embeddings through providers

CreateGroceryEmbedding loaded a local GGUF inside internal/models and
logged the same errors it returned. GroceryEmbedder now takes a Provider
and model, and OpenAI, Ollama, and OpenRouter implement Embed. add-grocery
calls OpenRouter with openai/text-embedding-3-small.
```

Stage `internal/embedding/grocery_embedder.go`, the deleted `internal/models` files, provider production files, `cmd/addGrocery.go`, and `go.mod` / `go.sum` if tidy changed them.

2. Tests only.

```text
test(embedding): cover embedder forwarding and vector conversion

The llama loader cannot run in unit tests. These tests lock constructor
checks, one-vector conversion, and that the embedder calls Embed rather
than Send.
```

Stage `*_test.go` only, including the prompt-writer fake stub.

3. Docs only, including this plan.

```text
docs: record provider-backed grocery embeddings

ARCHITECTURE, DECISIONS, and AGENTS still described llama.cpp and
internal/models. Point them at GroceryEmbedder, Provider.Embed, and the
add-grocery call that hardcodes OpenRouter plus text-embedding-3-small.
```

Stage `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, `AGENTS.md`, and `docs/plans/09-embedding-provider.md`.

```bash
git push -u origin HEAD
gh pr create --base <pr-base> --title "refactor, test, docs: generate grocery embeddings through providers" --body "$(cat <<'EOF'
### Added
- `GroceryEmbedder` in `internal/embedding`
- `Provider.Embed` on OpenAI, Ollama, and OpenRouter
- `add-grocery` call to `NewOpenRouterProvider` and `NewGroceryEmbedder` with `openai/text-embedding-3-small`
- `--api-key` on `add-grocery`, same viper key as `find-deals`
- Unit tests for constructor checks, forwarding, and float conversion
- Decision note that llama.cpp embeddings are no longer called from Go

### Removed
- None

### Updated
- Grocery embedding generation goes through `provider.Provider` instead of llama.cpp
- `add-grocery` stores the vector returned by OpenRouter
- `go.mod` drops `tcpipuk/llama-go` if tidy removed it
- Living docs describe the new package and the hardcoded model

### Deleted
- `internal/models` (`client.go`, `resolver.go`, `types.go`)
EOF
)"
```

Title lists `refactor`, `test`, `docs`. If `gh` is not authenticated, stop after the push and report the branch. Do not invent a token.

## Done when

- Worktree is `/home/bsoer/Documents/PROJECTS/stfg-embedding` on `embedding-provider`, branched from the selected remote base.
- `internal/models` is gone. `GroceryEmbedder` stores `provider.Provider` and the model, returns `[]float32`, and does not log.
- All three providers implement `Embed` using the pinned SDKs. `Send` is unchanged.
- The moved llama comments sit above the `Embed` call, verbatim.
- Checks above pass, including `go build ./cmd`.
- `add-grocery` is the only command that embeds, and it uses OpenRouter plus `openai/text-embedding-3-small`.
- Three commits, pushed. PR open against the chosen base, or `gh` auth failure reported with the branch name.
- No amend, force-push, or rebase.
