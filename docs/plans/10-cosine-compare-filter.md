# Plan 10 — Cosine prefilter for find-deals

**Status:** Ready to execute. This file is the spec. Do not expand it.
**Repo root:** `/home/bsoer/Documents/PROJECTS/VSCodeProjects/stfg`
**Module:** `stfg`
**Go:** `go 1.27.0` in `go.mod`
**Execute in this checkout.** Do not create a worktree, branch, commit, or pull request. Do not revert unrelated dirty files. The working tree already has uncommitted edits in `internal/bootstrap/providers.go`, `internal/commands/addGrocery.go`, `internal/commands/findDeals.go`, `internal/promptwriter/prompt_writer.go`, `internal/provider/openrouter.go`, and `internal/provider/types.go`. Leave those edits alone except for the `findDeals.go` change specified below.

## Operator decisions

Confirmed:

- Cosine similarity is a **prefilter**. Keep `PromptWriter.GetFlyerItemsOnGroceryList`. Do not replace the LLM match. Do not change prompt text, match schema, or provider `Send`.
- Embed each flyer item as `Brand + " " + Name`. There is no `Value` field. `storage.FlyerItem.Name` is that value. Do not include price, merchant, or display type.
- Similarity cutoff starts at **60%**, meaning cosine similarity `>= 0.60`. The package accepts the percentage at construction. The command passes 60. No CLI flag, no Viper key, no setter.
- Flyer embeddings are **memory only**. Do not write them to storage. Do not add a field for them. Do not cache them on disk. The only embeddings read from storage are the grocery embeddings already saved by `add-grocery`.

## In-memory filter, then the LLM

This is the `find-deals` order. Do not interleave it.

1. Fetch groceries from storage. Their embeddings are already stored. Use those vectors. Do not generate grocery embeddings again.
2. Fetch the flyers from storage, then fetch each flyer's items into an in-memory list. That list is the working copy. Storage is not updated.
3. For each flyer item still in that list, generate an embedding in a local variable from brand plus name. Compare that in-memory vector with every stored grocery embedding.
4. If the flyer item is not `>= 60%` similar to any grocery, remove that flyer item from the in-memory list. Then discard the flyer embedding. It is not kept on the item and not saved.
5. Only after that in-memory list has been filtered, continue to the existing LLM match. Pass the remaining items. If the list is empty, skip the LLM for that flyer.

## Outcome

`find-deals` still loads groceries and flyers, still calls the prompt writer, and still prints the same deal lines. The prompt writer never sees a flyer item that failed the in-memory 60% check. Flyer embeddings die with the comparison. They are not stored.

## Rules MiniMax must follow

Repository layout wins over generic Go layouts. Commands live in `internal/commands`, not `cmd/`. Do not move them. Do not edit project documentation during implementation. Another agent is replacing the documentation system. The files named below may not exist, or may not mean the same thing, when you finish the code.

- New feature package: `internal/cosinecompare/`. Package name `cosinecompare`. No generics. No functional options. No `init()`. No globals. No `sync.Once`.
- One behavior-owning struct per file. File name `cosine_compare.go`. Constructor returns `(*CosineCompare, error)`, not an interface.
- `context.Context` is never stored and is not needed on the comparer. Comparison is pure math.
- Do not fill caller pointers. Return the bool or the score.
- Function bodies go on the line below the signature. No clever one-liners.
- Wrap errors with `fmt.Errorf("...: %w", err)`. The library does not log, panic, `Fatal`, or `os.Exit`.
- Do not log and return. Command-edge warnings may use the existing `zap.S()` calls in `findDeals.go`. Do not introduce a new logger.
- Constructors are the only injection point. No `SetSimilarity`. No `WithSimilarity`.
- Do not import `storage`, `embedding`, `commands`, `promptwriter`, or `provider` from `cosinecompare`.
- Do not import `cosinecompare` from `embedding` or `provider`.
- Same-package tests. Standard `testing` only. No testify in our tests. No network. Do not export a helper only for tests.
- `gofmt` only files you touch. Do not restyle unrelated code. Do not reword or delete existing comments. The comment `// FindDealsCmd represents the find-deals command` stays verbatim.
- Do not edit `Makefile`, storage, flyer scraping, prompt writer, or provider.
- Do not edit `docs/`, `AGENTS.md`, `README.md`, or any other project documentation in the implementation step. Documentation is a later step and starts only after the operator confirms it.

## Library choice

Do not write the cosine formula.

Checked and rejected:

- `gonum.org/v1/gonum/floats` v0.17 is the usual numeric library, but the current package has `Dot` and `Norm` only. It has no `CosineSimilarity`, and it is `[]float64`. Embeddings in this repo are `[]float32`.
- `github.com/golang-infrastructure/go-cosine-similarity` is a tiny unmaintained module (published 2022) aimed at number slices, not a documented float32 embedding API.
- Vector indexes (`kelindar/search` and similar) are the wrong tool. This task compares two slices. It does not search an index.

Use **`github.com/viterin/vek/vek32`**.

- Module: `github.com/viterin/vek`
- Pin: `v0.4.3` (latest tagged release as of this plan; MIT; pushed 2025-09-06; not archived)
- Call: `vek32.CosineSimilarity(x, y []float32) float32`
- Docs: https://pkg.go.dev/github.com/viterin/vek/vek32#CosineSimilarity
- It panics if `x` is empty or the lengths differ. Our package must check those cases and return an error **before** calling it. Never let that panic escape.
- A zero vector yields `NaN` from the pure-Go path (`dp / sqrt(0)`). Treat `NaN` and `Inf` as an error, not as similar.
- SIMD is on by default for amd64 AVX2 and falls back to pure Go. Do not call `SetAcceleration`. Do not add build tags.

Add the dependency with `go get github.com/viterin/vek@v0.4.3` from the repo root, then `go mod tidy`. Do not bump other modules.

## Package API

`internal/cosinecompare/cosine_compare.go`

`CosineCompare` holds one unexported `similarityPercent float64`. Named field, no embedding of another type.

Export a constant `DefaultSimilarityPercent = 60`. The command uses that constant. The constructor does not silently substitute it when the caller passes something else.

`NewCosineCompare(similarityPercent float64) (*CosineCompare, error)`

- Reject `NaN`, `Inf`, `< 0`, and `> 100` with `fmt.Errorf("cosine compare: similarity percent ...")`. Do not trim or clamp.
- `0` and `100` are valid.
- No I/O.

`CosineSimilarity(left, right []float32) (float32, error)`

- Doc comment: returns the cosine similarity of the two embeddings, in `[-1, 1]` for ordinary non-zero vectors.
- `nil` and empty are the same failure: `fmt.Errorf("cosine compare: embedding is empty")`. Do not call vek.
- Different lengths: `fmt.Errorf("cosine compare: embedding length mismatch")`. Do not call vek.
- Otherwise call `vek32.CosineSimilarity`. If the result is `NaN` or `Inf`, return `fmt.Errorf("cosine compare: undefined similarity")` and a zero score.
- This method does not apply the percentage. It does not copy the slices. vek's cosine function does not mutate them.

`AreCosineSimilar(left, right []float32) (bool, error)`

- Doc comment: reports whether the two embeddings are at least as similar as the configured percentage.
- Call `CosineSimilarity`. On error, return `false` and that error. Do not treat an error as "not similar" by swallowing it.
- Similar when `score >= float32(similarityPercent / 100)`. Use `>=`, so exactly 60% passes when the percent is 60.
- Negative scores are not similar at 60%.

No other exported methods. No batch API. No string comparison. No embedding.

## find-deals wiring

File: `internal/commands/findDeals.go`. Read it before editing. Current shape, including the dirty working tree:

- `FindDealsCmd` takes `*container.ContainerRegistry`.
- It loads storage key `"json"` and provider key `"openrouter"`.
- Empty flyers or empty groceries print a line and return nil.
- Prompt writer model in the working tree is `openai/gpt-4.1-nano`. Do not change that string.
- It lists flyer items per flyer, calls `promptWriter.GetFlyerItemsOnGroceryList`, and prints `flyerItem.Brand`, `flyerItem.Name`, `flyerItem.Price`.

Keep that print format and the empty-list messages.

After groceries are loaded and non-empty, and after the provider is in hand:

1. Read the model string from `internal/commands/addGrocery.go` at edit time and pass that same literal to `embedding.NewGroceryEmbedder`. Do not invent a second model. Do not "correct" it to `openai/text-embedding-3-small` if the file says something else. At plan-write time the working tree says `nvidia/nemotron-3-embed-1b:free` and git HEAD says `openai/text-embedding-3-small`. The file on disk wins. Do not edit `addGrocery.go`.
2. Construct `cosinecompare.NewCosineCompare(cosinecompare.DefaultSimilarityPercent)` once. A constructor error is returned from `RunE`.
3. Reuse the provider already fetched. Do not call `GetProvider` again.

Inside the existing flyer loop, after `ListFlyerItems` and the empty-item `continue`:

The `[]storage.FlyerItem` from `ListFlyerItems` is the in-memory list for that flyer. Filter that list to completion before any prompt-writer call for that flyer.

- Allocate a new slice for the items that stay. Do not write the filtered list back to storage. Do not delete flyer rows from the database or JSON cache.
- A flyer embedding is a local `[]float32` used for that item's comparisons, then dropped. Do not put it on `FlyerItem`. Do not add a storage method. Do not save it beside the groceries.
- Pass only the filtered in-memory slice to `GetFlyerItemsOnGroceryList`. That call is the next step after filtering, not part of filtering.
- If the filtered slice is empty, do not call the prompt writer. Use the existing debug log style for no matches (`zap.S().Debugf`, keep the current wording for the LLM no-match path). A separate debug line for "all items filtered" is fine. Do not print a user-facing line per skipped flyer.

Prefilter rule for one flyer item:

- Text is `strings.TrimSpace(item.Brand + " " + item.Name)`. One space between the fields before trim. Do not lowercase. Do not collapse internal spaces beyond `TrimSpace`.
- If that text is empty, drop the item. Do not embed it. Do not keep it.
- Otherwise `CreateGroceryEmbedding(ctx, text)`. One embed per flyer item, not per grocery. On error, return `fmt.Errorf("find deals: embed flyer item %d: %w", item.ID, err)` from `RunE`. Do not continue with a partial filter. A failed embed must not look like "no deals".
- Compare that vector to each grocery's stored `Embedding`. Do not re-embed grocery names.
- Keep the flyer item in the in-memory list only if `AreCosineSimilar` is true for **any** grocery that has a non-empty embedding. Otherwise remove it from that list. Below 60% is removal, not a softer match.
- A compare error (length mismatch, empty vector, NaN) is returned from `RunE`, wrapped with the grocery name and flyer item id. Do not skip it. Different vector lengths mean the stored grocery embedding was made by a different model. Failing is the signal to re-add groceries. Do not re-embed groceries in this task.
- Groceries with `nil` or empty `Embedding` are skipped. Log one `zap.S().Warnf` per such grocery, not per flyer item. If every grocery embedding is empty, return an error telling the user to add the groceries again so embeddings exist. Do not call the prompt writer in that case.
- Do not persist flyer embeddings. Do not add storage fields. Do not cache them across process runs. Grocery embeddings stay where `add-grocery` put them. Flyer embeddings do not join them.

A private function in `findDeals.go` is allowed if it keeps `RunE` readable. It is command orchestration, not a new package. It must not be exported. Pass the embedder, comparer, groceries, and items in. Return the kept items and an error. Do not put this function in `cosinecompare`.

Append one sentence to the existing `Long` string: flyer items are embedded from brand and name and dropped unless they are at least 60% cosine-similar to a grocery embedding, and the remaining items are still matched by the model. Do not rewrite `Use` or `Short`. Do not add `Example` or flags.

The info log `Searching for deals on %d grocery items across %d flyers...` stays. Do not log vectors or API keys.

## Docs — stop and wait

This is a separate step. Do not start it while implementing or testing. Do not edit documentation "while you are there."

After the code and tests pass, stop. Tell the operator the code step is done and that documentation has not been touched. Wait for an explicit confirmation to document the change. Silence, a passing test run, or this plan's existence is not that confirmation.

When confirmation is given:

- Read the documentation as it exists then. Another agent is replacing this project's documentation. Do not assume `docs/DECISIONS.md`, `docs/ARCHITECTURE.md`, or `AGENTS.md` are still the system, still at those paths, or still use the wording in this plan.
- Follow the new system's own rules for where a decision, an architecture note, and an agent guide entry go. If those rules are unclear, stop and ask. Do not invent a parallel doc.
- Record only this change: `find-deals` loads flyers into memory, embeds each flyer item as brand plus name with the same `GroceryEmbedder` model string as `add-grocery`, and compares that in-memory vector with the grocery embeddings already in storage. A flyer item below 60% cosine similarity (`internal/cosinecompare`, `vek32.CosineSimilarity`) is removed from the in-memory list. Flyer vectors are not written to storage. Only the remaining items go to the prompt writer. Stored grocery vectors must already exist and must come from the same model. The LLM match remains.
- Do not rewrite unrelated documentation. Do not restore old file layouts or old model strings just because this plan mentioned them.

## Tests

`internal/cosinecompare/cosine_compare_test.go`, package `cosinecompare`.

Direct named tests, not a table, unless the cases are truly the same assertion.

- Constructor rejects `-1`, `101`, and `math.NaN()`. Accepts `0`, `60`, and `100`.
- Identical non-zero vectors: `CosineSimilarity` is about `1` (tolerance `1e-5`). `AreCosineSimilar` at 60 is true. At 100 it is true.
- Orthogonal `[1, 0]` and `[0, 1]`: score about `0`. Not similar at 60. Similar at 0, because `>= 0`.
- Opposite `[1, 0]` and `[-1, 0]`: score about `-1`. Not similar at 60 or at 0.
- Empty, nil, and length mismatch return an error and do not panic. Use a recover only if you need to prove vek was not reached; prefer not calling vek by checking the error text contains `cosine compare:`.
- Zero vector `[0, 0]` against `[1, 0]` returns the undefined-similarity error, and `AreCosineSimilar` is false with that error.
- A score just under the cutoff is not similar. Build that with threshold `0` vs orthogonal for the boundary, plus one case at threshold `100` where a non-identical vector is false. Do not depend on a hand-computed `0.6` vector unless it is exact and commented with the inputs.

No `findDeals` test. There is no command test harness, and do not add one. Do not reshape production code so a test can call the filter.

## Verification

From the repo root:

- `gofmt -w` the files you edited
- `go test ./internal/cosinecompare/`
- `go test ./internal/commands/ ./internal/embedding/ ./internal/promptwriter/` if those packages still compile. Do not run `make`. Do not call OpenRouter. Do not scrape flyers.

Stop if `vek32.CosineSimilarity` is missing at v0.4.3. Do not fall back to a handwritten formula.

## Out of scope

- Re-embedding existing groceries
- Persisting flyer embeddings
- A similarity flag or config key
- Changing the prompt-writer model, prompt, or match validation
- Replacing the LLM match
- Storage migrations
- Commits, branches, and pull requests
- Documentation edits before the operator confirms the docs step
