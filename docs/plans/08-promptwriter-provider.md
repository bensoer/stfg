# Plan 8 — Provider-agnostic prompt writer

**Status:** Execute only after Plan 7's structured `Send` is on the base commit.
**Repo root:** `/home/bsoer/Documents/PROJECTS/VSCodeProjects/stfg`
**Module:** `stfg`
**Remote:** `git@github.com:bensoer/stfg.git` (`origin`)
**Spec file:** this file. It is untracked in the primary checkout until you copy it.

Do not expand the product. Do not re-implement Plan 7. Do not read other files in `docs/plans/` except to copy this file. Do not edit `docs/PROJECT_CONTEXT.md`, `README.md`, `internal/provider/`, `internal/storage/`, flyer scraping, embeddings, or the reconciler.

## Prerequisite

Plan 7 changes `Provider.Send` to:

`Send(ctx context.Context, prompt string, model string) ([]GroceryFlyerMatch, error)`

and adds `decodeGroceryFlyerMatches` in `internal/provider/types.go`. The prompt writer after that plan still has a local `ProviderPrompter` and `OpenRouterFreePromptWriter` with the model hardcoded to `openrouter/free`.

Before creating the worktree:

```bash
git fetch origin
```

Pick the base:

- If `git grep -n decodeGroceryFlyerMatches origin/main -- internal/provider/types.go` matches, base is `origin/main`. Pull request base is `main`.
- Else if `origin/structured-provider-matches` exists and that same grep matches there, base is `origin/structured-provider-matches`. Pull request base is `structured-provider-matches`.
- Else stop. Report that Plan 7 is not on a remote branch yet. Do not branch from local `main`. Do not implement against `*string` `Send`.

No ticket number was provided. Do not ask. Branch name: `promptwriter-provider`.

```bash
git worktree add -b promptwriter-provider /home/bsoer/Documents/PROJECTS/stfg-promptwriter <base>
mkdir -p /home/bsoer/Documents/PROJECTS/stfg-promptwriter/docs/plans
cp /home/bsoer/Documents/PROJECTS/VSCodeProjects/stfg/docs/plans/08-promptwriter-provider.md \
  /home/bsoer/Documents/PROJECTS/stfg-promptwriter/docs/plans/08-promptwriter-provider.md
```

If that path already exists, stop. Do not reuse or delete it. Do all edits, tests, commits, and the pull request from the worktree. Leave the primary checkout alone after the copy.

## Outcome

`internal/promptwriter` has one type, `PromptWriter`, in `prompt_writer.go`. It accepts any `provider.Provider` plus the model name, builds the existing flyer/grocery prompt, calls `Send`, and maps schema-valid matches back onto the real `storage.FlyerItem` rows.

`find-deals` still uses OpenRouter and the model `openrouter/free`. It does not gain a provider flag, a provider switch, or calls to `NewOpenAIProvider` / `NewOllamaProvider`.

Deleted after the move: `providerprompter.go`, `openrouterfree.go`, `openrouterfree_test.go`. No second interface. No `OpenRouterFreePromptWriter`. No local `GroceryFlyerMatch`.

## Rules

- No generics. No functional options. No provider factory or `switch`.
- Constructor returns `(*PromptWriter, error)`, not an interface.
- `context.Context` is the first parameter of `GetFlyerItemsOnGroceryList`. Do not store a context on the struct. Do not nil-check it.
- Return the map by value. Do not fill a caller pointer.
- Function bodies go on the line below the signature. Wrap new errors with `fmt.Errorf("...: %w", err)`.
- Do not log inside new code. Keep the existing `zap` calls that Plan 7 left in the method. Do not "fix" the log-and-return.
- Do not restyle unrelated code. `gofmt` only files you touch.
- Read each existing file before editing. Keep every existing comment verbatim, including stale wording. Move a comment with the code it annotates. Do not reword it.
- The shopping-assistant comments now live on `provider.GroceryFlyerMatch` in `internal/provider/types.go`. Leave them there. Do not copy them back.
- Same-package tests. Hand-written fake only. No network, no SDK client, no live model, no new test-only export.
- Never `git commit --amend`, force-push, or rebase. A bad commit gets a new commit. Stay under 500 changed lines per commit. Do not commit `res/`, `lib/`, `bin/`, or `models/`.

## What to build

Read the post-Plan-7 `openrouterfree.go` in the worktree. That file is the behavior source. Move it; do not redesign the match loop or the prompt text.

`prompt_writer.go`:

- `PromptWriter` holds a named `provider.Provider` field and a `model string`. Do not embed the provider.
- `NewPromptWriter(p provider.Provider, model string) (*PromptWriter, error)`.
  - Nil interface: `p == nil` → `fmt.Errorf("prompt writer: provider is nil")`.
  - `model == ""` → `fmt.Errorf("prompt writer: model is empty")`.
  - Do not trim the model. Do not use reflection to catch a typed nil behind a non-nil interface. Callers pass the struct pointer from `NewOpenRouterProvider`.
  - No network or file I/O in the constructor.
- Add a one-line doc comment on the exported type, constructor, and method. These are new comments, not rewrites.
- `GetFlyerItemsOnGroceryList(ctx context.Context, flyerItems []storage.FlyerItem, groceryList []storage.GroceryItem) (map[string][]storage.FlyerItem, error)`.
- Keep the prompt string from the post-Plan-7 file byte for byte, including the typo `ALWAYS respond in with` and the `{"matches":[...]}` example. Do not restore the bare JSON array example.
- Call `p.Send(ctx, prompt, model)` using the stored model. Do not hardcode `openrouter/free` inside the package.
- Keep the 3-iteration retry only for a match whose flyer id, flyer name, and grocery name are not all present in the inputs. A `Send` error still aborts immediately, wrapped as `error sending prompt`. An empty `[]GroceryFlyerMatch` is success and returns an empty map. Do not retry it.
- Lookup stays exact: `flyerItem.ID == gfm.FlyerItemID`, `flyerItem.Name == gfm.FlyerItemName`, and `internal.Contains(groceryNames, gfm.GroceryItem)`. `Contains` is case-sensitive equality. Do not fuzzy-match.
- On a hit, store the input `storage.FlyerItem`, not a struct rebuilt from the match. Price and brand exist only on that row. First matching row wins (`break`). Later matches for the same grocery append.
- One bad match rejects the whole response and retries. Do not return a partial map.
- After 3 failed attempts, return the existing string unchanged: `Failed To Get Valid Response From LLM. Unable To Verify Grocery Items Are On Flyer`.
- Keep the `Debugf` format string unchanged. Keep `zap.S().Debug("Repairs were fine, on to checking if things match")`.

Comment move when `providerprompter.go` is deleted:

`// Sends a prompt to a model and returns response as a map[string]any`

Place that line immediately above the `Send` call. Every other comment in the post-Plan-7 method moves with its branch into `prompt_writer.go`.

`cmd/findDeals.go`, after Plan 7's `NewOpenRouterProvider` error handling:

- Replace `NewOpenRouterFreePromptWriter(client)` with `NewPromptWriter(client, "openrouter/free")`.
- If the constructor returns an error, log it with `zap.S().Errorf` and return. Do not continue the flyer loop.
- Keep `cmd.Context()` at the match call. Do not change flags, help text, `init()`, or printing.

## Edge cases to leave alone

- A nil context is a caller bug.
- Duplicate flyer ids: first row wins.
- Grocery names are matched exactly. Do not normalize case or whitespace.
- The prompt still sends only flyer `id`, `brand`, `name`, `displayType`, `imgURL`, and grocery names. Do not add price.
- `find-deals` still loops flyers itself. Do not batch flyers inside the prompt writer.
- Do not import `promptwriter` from `provider`.
- `internal/storage/storage_test.go` counts `type FlyerItem struct` and `type GroceryItem struct` in the storage package. Do not add copies of those structs.
- The `DisplayType` comment in `internal/storage/types.go` mentions OpenRouter. Do not edit it.

## Tests

Replace `openrouterfree_test.go` with `prompt_writer_test.go` in `package promptwriter`. The fake implements `provider.Provider` by recording `prompt` and `model` and returning a scripted slice or error. It does not parse JSON.

Carry the three Plan 7 cases forward under the new type:

1. One match whose grocery name, flyer id, and flyer name exist in the inputs. Result maps that grocery to that `storage.FlyerItem`. `Send` called once. Prompt contains `"matches"` and does not use a bare JSON array as the response example. Recorded model equals the model passed to `NewPromptWriter`, and that model is not hardcoded by omitting the argument.
2. Match with a `FlyerItemID` absent from the flyer slice, returned three times. `Send` called three times. Error string is exactly `Failed To Get Valid Response From LLM. Unable To Verify Grocery Items Are On Flyer`.
3. First `Send` returns an error. Method returns immediately, wrapped with `error sending prompt`. `Send` called once.

Add two constructor tests: nil provider, and empty model. Neither calls `Send`.

Pass `context.Background()`. Do not assert on zap. Build the smallest `FlyerItem` (`ID`, `Name`) and `GroceryItem` (`Name`) the cases need.

## Documentation

Update living docs so a later agent does not revive `ProviderPrompter`, `OpenRouterFreePromptWriter`, or a hardcoded model inside the prompt writer. Read the post-Plan-7 text first. Plan 7 already rewrote the provider sections. Edit only the prompt-writer facts.

### `docs/ARCHITECTURE.md`

- PromptWriter bullet: one `PromptWriter` takes a `provider.Provider` and a model name, builds the flyer/grocery prompt, and checks returned matches against the real flyer rows.
- Data Flow step 3: `find-deals` still constructs `NewOpenRouterProvider` and passes that provider plus `openrouter/free` to `NewPromptWriter`. Do not say it selects among providers.
- Directory-tree promptwriter comment: prompt writer over any `Provider`.
- Do not redraw storage, embeddings, or the three-provider section Plan 7 added.

### `docs/DECISIONS.md`

Do not rewrite `Structured Provider Matches` or the older LLM-matching section. Append:

```markdown
## Provider-Agnostic Prompt Writer

**Decision:** `PromptWriter` depends on `provider.Provider` directly. The model name is a constructor argument. `find-deals` passes `NewOpenRouterProvider` and `openrouter/free`. There is no local prompter interface and no OpenRouter-specific writer type.

**Reason:** After `Send` returned `[]GroceryFlyerMatch`, the local interface duplicated `provider.Provider` and the writer hardcoded one model. Keeping provider choice at the command leaves OpenAI and Ollama unwired until a caller needs them, without a factory.
```

### `AGENTS.md`

- Tree comment for `promptwriter/`: builds the match prompt and validates provider matches.
- Finding Deals: load groceries and flyers, call `NewOpenRouterProvider`, pass it and `openrouter/free` to `NewPromptWriter`. Matching still goes through `Provider.Send` and decoded `[]GroceryFlyerMatch`.
- File Locations: add that `internal/promptwriter/prompt_writer.go` is the only prompt-writer type. Do not remove Plan 7's `internal/provider/types.go` row.

## Out of scope

- Wiring OpenAI or Ollama into `find-deals`, or a `--provider` / `--model` flag.
- Changing prompt wording, retry count, or match equality.
- A schema argument, generic `Send`, streaming, or JSON repair.
- Editing `internal/provider`, including its comments and tests.
- `make`, `go test ./...`, or anything that builds llama.cpp.
- Live model calls.

## Order

1. Confirm the base, create the worktree, copy this spec, `cd` there.
2. Read `openrouterfree.go`, `providerprompter.go`, `cmd/findDeals.go`, and `openrouterfree_test.go` in the worktree. List their comments before editing.
3. Add `prompt_writer.go` with the moved method and comments. Update the `find-deals` constructor call.
4. Delete `providerprompter.go` and `openrouterfree.go` only after their comments are in `prompt_writer.go`.
5. Add `prompt_writer_test.go` and delete `openrouterfree_test.go`.
6. Update `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, and `AGENTS.md`.
7. `gofmt` the Go files you touched.
8. Run the checks. Commit only after they pass.

## Checks

From the worktree:

```bash
go test ./internal/promptwriter/ ./internal/provider/
go build -o /dev/null ./internal/promptwriter/ ./internal/provider/ ./cmd/
```

Confirm `rg -n "ProviderPrompter|OpenRouterFreePromptWriter|openrouterfree" --glob '!docs/plans/**'` finds nothing under the worktree. `rg` of `openrouter/free` should hit `cmd/findDeals.go` only, not `internal/promptwriter`.

## Commits and pull request

Three commits, in this order. Tests and docs never share a commit with source.

1. Source only. No `*_test.go`. No `docs/`. No `AGENTS.md`.

```text
refactor(promptwriter): take any provider and a model name

OpenRouterFreePromptWriter duplicated provider.Provider and hardcoded
openrouter/free. PromptWriter now accepts the provider interface and the
model, and find-deals passes the OpenRouter client plus that same model.
```

Stage `internal/promptwriter/prompt_writer.go`, the deleted prompt-writer Go files, and `cmd/findDeals.go`.

2. Tests only.

```text
test(promptwriter): cover provider injection and match retries

The writer no longer has its own prompter interface. These tests use a
Provider fake to lock constructor checks, model forwarding, and the
existing retry rules without calling a model.
```

Stage `prompt_writer_test.go` and the `openrouterfree_test.go` deletion.

3. Docs only, including this plan.

```text
docs: record the provider-agnostic prompt writer

ARCHITECTURE, DECISIONS, and AGENTS still described an OpenRouter-only
writer and a local prompter interface. Point them at PromptWriter and
the find-deals call that still supplies openrouter/free.
```

Stage `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, `AGENTS.md`, and `docs/plans/08-promptwriter-provider.md`.

If a commit would exceed 500 changed lines, split that group by file and keep the same type prefix. Do not move a test into the refactor commit.

```bash
git push -u origin HEAD
gh pr create --base <pr-base> --title "refactor, test, docs: take any provider in the prompt writer" --body "$(cat <<'EOF'
### Added
- `PromptWriter` in `internal/promptwriter/prompt_writer.go`, taking `provider.Provider` and a model name
- Constructor checks for a nil provider and an empty model
- Tests for model forwarding, constructor failure, and the existing match retry rules
- Decision note that provider choice stays at the caller

### Removed
- None

### Updated
- `find-deals` now calls `NewPromptWriter(client, "openrouter/free")`
- Prompt-writer tests target `provider.Provider` instead of a local interface
- `docs/ARCHITECTURE.md` and `AGENTS.md` describe the single prompt-writer type

### Deleted
- `ProviderPrompter` and `providerprompter.go`
- `OpenRouterFreePromptWriter`, `openrouterfree.go`, and `openrouterfree_test.go`
EOF
)"
```

`<pr-base>` is the base chosen above. Title lists `refactor`, `test`, `docs` in that order. If a later commit adds another type, `gh pr edit` the title. Do not force-push.

If `gh` is not authenticated, stop after the push and report the branch name. Do not invent a token.

## Done when

- Worktree is `/home/bsoer/Documents/PROJECTS/stfg-promptwriter` on `promptwriter-provider`, branched from the Plan 7 base, not from a dirty primary `main`.
- `PromptWriter` is the only type in `internal/promptwriter`. It stores `provider.Provider` and the model. It does not mention `openrouter/free`.
- `find-deals` still uses `NewOpenRouterProvider`, passes `openrouter/free`, and passes `cmd.Context()`.
- Match retry, prompt text, and error strings match the post-Plan-7 writer.
- The deleted interface comment sits above `Send`. No other existing comment was reworded or dropped.
- `go test ./internal/promptwriter/ ./internal/provider/` and the `go build` check pass.
- Three commits: source, tests, docs. Branch is pushed. A pull request is open against the chosen base, or `gh` auth failure is reported with the branch name.
- No amend, force push, or rebase.
