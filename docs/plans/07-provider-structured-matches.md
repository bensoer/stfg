# Plan 7 — Structured grocery-flyer matches from providers

**Status:** Ready to execute. This file is the spec.
**Repo root:** `/home/bsoer/Documents/PROJECTS/VSCodeProjects/stfg`
**Module path:** `stfg`
**Go:** 1.26.5 (`go.mod`)
**Remote:** `git@github.com:bensoer/stfg.git` (`origin`)
**Base branch:** `main` (`origin/main`)

Execute this plan in its own git worktree. Finish with commits, a push, and a pull request. Do not expand the product change. Do not read other files in `docs/plans/` except to copy this file into the worktree. Do not edit `docs/PROJECT_CONTEXT.md` or `README.md`.

## Where to execute

Do not implement in the primary checkout at `/home/bsoer/Documents/PROJECTS/VSCodeProjects/stfg`. That checkout is `main`, and this plan is an untracked file there. Leave that working tree alone after you have copied this plan.

No ticket number was provided. Do not stop to ask. Use branch `structured-provider-matches`.

```bash
git fetch origin
git worktree add -b structured-provider-matches /home/bsoer/Documents/PROJECTS/stfg-structured-provider-matches origin/main
```

Then copy this spec into the worktree:

```bash
mkdir -p /home/bsoer/Documents/PROJECTS/stfg-structured-provider-matches/docs/plans
cp /home/bsoer/Documents/PROJECTS/VSCodeProjects/stfg/docs/plans/07-provider-structured-matches.md \
  /home/bsoer/Documents/PROJECTS/stfg-structured-provider-matches/docs/plans/07-provider-structured-matches.md
```

Do all edits, tests, commits, and the pull request from `/home/bsoer/Documents/PROJECTS/stfg-structured-provider-matches`. If that path already exists, stop and report it. Do not reuse or delete another worktree.

Git rules for the executor:

- Branch from the fetched `origin/main`. Do not branch from a dirty local `main`.
- Never `git commit --amend`. Never `git push --force` or `--force-with-lease`. Do not rebase.
- If a commit is wrong, add a new commit. Use `git revert` only to undo a commit you already pushed.
- Split commits as specified in [Commits and pull request](#commits-and-pull-request). Stay under 500 changed lines per commit.
- Do not commit `res/`, `lib/`, `bin/`, or `models/`.

## What you are changing

Today every provider returns the model text as `*string`. The only caller parses that text into grocery-to-flyer matches. Change the provider contract so `Send` returns those matches as a Go slice, and make OpenAI, Ollama, and OpenRouter request the same JSON schema.

Do not add `Send[T any]`. Do not take a schema argument. Do not return `any`. There is one response shape. A generic method is out of scope even if it looks more flexible.

## Rules that override local taste

These are already decided. Do not stop to ask about them.

- No generics. No functional options. No new provider factory or `switch` of providers.
- `context.Context` is the first parameter of `Send` and of `GetFlyerItemsOnGroceryList`. Do not store a context on a struct.
- Return `[]GroceryFlyerMatch` by value. Do not return `*GroceryFlyerMatches` and do not fill a caller pointer.
- Constructors stay as they are and still return `(*Struct, error)`, not the interface.
- Do not log inside provider methods. The command and the prompt writer already log.
- Do not restyle unrelated code. Do not reformat files you are not changing.
- Edit existing Go files in place. `internal/provider/types.go` is empty, so replacing that file is fine. `internal/provider/decode_test.go` is new.
- Before editing an existing file, read it and keep every existing comment verbatim. Do not reword, shorten, or "fix" comments, including comments that become stale. Placement rules for the comments this change disturbs are in [Comment placement](#comment-placement). Do not ask.
- Do not edit `internal/provider/errors.go`. `ModelResponseError` already exists there. Its package comment still documents the old `Send` signature. Leave that comment.
- Function bodies go on the line below the signature. Wrap new errors with `fmt.Errorf("...: %w", err)`.
- Same-package tests (`package provider`). No network. No SDK mocks. No new test-only export.

## Current state you need

### Files

| File | Role now | What to do |
|---|---|---|
| `internal/provider/protocols.go` | `Send(prompt, model string) (*string, error)` | Change the signature only |
| `internal/provider/types.go` | Empty package file | Add the response types, schema, and decoder |
| `internal/provider/openai.go` | Chat Completions, returns `choice.Message.Content` | Attach schema, decode |
| `internal/provider/ollama.go` | `client.Chat`, returns `resp.Message.Content` | Set `Format` to the schema, decode |
| `internal/provider/openrouter.go` | Does not compile. Returns `&prompt` | Attach schema, read the content union, decode |
| `internal/provider/errors.go` | `ModelResponseError` and friends | Do not edit |
| `internal/promptwriter/providerprompter.go` | Local consumer interface, same old `Send` | Change the signature only |
| `internal/promptwriter/openrouterfree.go` | Builds the prompt, repairs JSON, unmarshals, checks ids | Stop repairing and unmarshaling. Keep the id check |
| `cmd/findDeals.go` | Calls missing `provider.NewClient` | Wire `NewOpenRouterProvider` and pass `cmd.Context()` |
| `internal/provider/decode_test.go` | Does not exist | Add decoder tests |
| `internal/promptwriter/openrouterfree_test.go` | Does not exist | Add prompt-writer tests with a hand-written fake |
| `docs/ARCHITECTURE.md` | Says the provider package is OpenRouter only | Update the provider and find-deals sections |
| `docs/DECISIONS.md` | Says matching is OpenRouter text in, text out | Add the structured-match decision |
| `AGENTS.md` | Still lists `jsonrepair` and embedding-based find-deals | Update the provider, dependency, and find-deals notes |
| `docs/plans/07-provider-structured-matches.md` | This spec | Commit it with the docs commit |

No other Go file calls `Provider.Send`, `ProviderPrompter.Send`, or `GetFlyerItemsOnGroceryList`.

### SDKs already in `go.mod`

- `github.com/openai/openai-go/v3` v3.64.0
- `github.com/liliang-cn/ollama-go` v0.2.1
- `github.com/OpenRouterTeam/go-sdk` v0.5.18
- `github.com/kaptinlin/jsonrepair` v0.4.8 is only imported by the prompt writer. After that import is gone, run `go mod tidy`. Do not re-add it. The `AGENTS.md` row for that dependency is removed in the docs commit, not left behind because tidy does not edit markdown.

None of the three SDKs decode into a Go struct. They accept a JSON schema and still return a string. This package unmarshals that string.

### Why the wire format is an object

The prompt today asks for a bare JSON array. OpenAI strict structured outputs need a root object, `additionalProperties: false`, and every property listed in `required`. Use one envelope for all three SDKs:

```json
{
  "matches": [
    {
      "grocery_item": "milk",
      "flyer_item_id": 42,
      "flyer_item_name": "2% Milk 4L"
    }
  ]
}
```

Empty success is `{"matches":[]}`. Missing `matches` and `"matches": null` are decode errors. Callers receive the inner slice, not the envelope.

## Types, schema, and decode

Put all of this in `internal/provider/types.go`. Do not copy the schema into the provider files.

```go
package provider

import "encoding/json"

// GroceryFlyerMatch is one grocery list item matched to one flyer item.
// You are a helpful shopping assistant. Below is a store flyer and a list of groceries the user wants to buy.
//
// Decide which items from the grocery list appear (or have a close match/equivalent) in the flyer. For each match, report the grocery item, the matching flyer item id, and the matching flyer item name if available.
//
// Respond ONLY with a JSON array of objects, each with the keys "grocery_item", "flyer_item_id" and "flyer_item_name". If there are no matches, respond with an empty array. If a grocery does not have a match DO NOT create an entry with empty flyer_item_id and flyer_item_name values. Do not include it in the response instead
type GroceryFlyerMatch struct {
	GroceryItem   string `json:"grocery_item"`
	FlyerItemID   int64  `json:"flyer_item_id"`
	FlyerItemName string `json:"flyer_item_name"`
}

// GroceryFlyerMatches is the object the model is asked to return.
type GroceryFlyerMatches struct {
	Matches []GroceryFlyerMatch `json:"matches"`
}

func groceryFlyerMatchesSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"matches": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"grocery_item":    map[string]any{"type": "string"},
						"flyer_item_id":   map[string]any{"type": "integer"},
						"flyer_item_name": map[string]any{"type": "string"},
					},
					"required":             []string{"grocery_item", "flyer_item_id", "flyer_item_name"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"matches"},
		"additionalProperties": false,
	}
}

func decodeGroceryFlyerMatches(providerName, raw string) ([]GroceryFlyerMatch, error) {
	var envelope struct {
		Matches json.RawMessage `json:"matches"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil || len(envelope.Matches) == 0 || string(envelope.Matches) == "null" {
		return nil, &ModelResponseError{Provider: providerName, Raw: raw}
	}

	var matches []GroceryFlyerMatch
	if err := json.Unmarshal(envelope.Matches, &matches); err != nil {
		return nil, &ModelResponseError{Provider: providerName, Raw: raw}
	}
	if matches == nil {
		matches = []GroceryFlyerMatch{}
	}
	return matches, nil
}
```

The three comment lines between the new doc sentence and `type GroceryFlyerMatch` are an existing comment. Copy them verbatim, including the wording that still says "JSON array". Do not update that wording.

`encoding/json` cannot tell a missing field from a zero slice. The `json.RawMessage` check is required. Do not replace it with a direct unmarshal into `GroceryFlyerMatches`.

JSON keys stay `grocery_item`, `flyer_item_id`, `flyer_item_name`. The Go field is `FlyerItemID`, not the current `FlyerItemId`. The prompt writer field reads must change to `FlyerItemID`.

Schema name, where an SDK requires one: `grocery_flyer_matches`.

## Interface

`internal/provider/protocols.go` today:

```go
package provider

// Provider defines the contract for sending prompts to a model and receiving responses.
type Provider interface {
	// Send sends a prompt to the underlying model and returns the raw response as a string.
	Send(prompt string, model string) (*string, error)
}
```

Keep both comments verbatim. Change only the method line to:

```go
	Send(ctx context.Context, prompt string, model string) ([]GroceryFlyerMatch, error)
```

Add `"context"` to the imports.

Each provider method has this comment, which must stay verbatim above the new signature:

```go
// Send sends a prompt to the underlying model and returns the raw response as a string.
```

Inside every `Send`, keep the three-minute bound, but start from the caller context:

```go
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
```

Do not call `context.Background()`. A nil context is a caller bug. `cmd.Context()` is non-nil inside `Run`.

`prompt` stays a string. Prompt construction stays in the prompt writer.

## OpenAI

File: `internal/provider/openai.go`. Package import is `openai "github.com/openai/openai-go/v3"`. Add `github.com/openai/openai-go/v3/shared`.

Keep `OpenAIProvider`, `NewOpenAIProvider`, and the empty-choice / empty-content checks. Those checks still use `errors.New("empty response from openai")` and `errors.New("empty message content in openai response")`.

After the empty-content check, decode instead of returning `&choice.Message.Content`.

Request shape:

```go
	chatCompletion, err := op.Client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "grocery_flyer_matches",
					Schema: groceryFlyerMatchesSchema(),
					Strict: openai.Bool(true),
				},
			},
		},
	})
```

`Schema` is `any`. `openai.Bool` is `param.Opt[bool]`, which is what `Strict` wants.

Stay on Chat Completions. Do not move this call to the Responses API.

Decode:

```go
	matches, err := decodeGroceryFlyerMatches("openai", choice.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("openai decode matches: %w", err)
	}
	return matches, nil
```

## Ollama

File: `internal/provider/ollama.go`. Import alias is `ollama "github.com/liliang-cn/ollama-go"`.

Keep `OllamaProvider`, `NewOllamaProvider`, and the client construction inside `Send`:

```go
	client, err := ollama.NewClient(
		ollama.WithHost(op.Host),
		ollama.WithHTTPClient(op.client),
	)
```

Keep `errors.New("nil response from ollama")` for a nil response.

`ChatRequest.Format` is `interface{}`. Assign the schema map. Do not set `Format` to the string `"json"`. Do not switch to the package-level `ollama.Chat` helper.

```go
	resp, err := client.Chat(ctx, &ollama.ChatRequest{
		Model: model,
		Messages: []ollama.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Format: groceryFlyerMatchesSchema(),
	})
```

Decode `resp.Message.Content` with provider name `"ollama"` and the same `fmt.Errorf("ollama decode matches: %w", err)` wrap.

Ollama enforcement depends on the model. Do not add an Ollama-only repair parser. A bad body is a decode error.

## OpenRouter

File: `internal/provider/openrouter.go`. This file does not compile today.

Confirmed against `go-sdk` v0.5.18:

- `components.ChatRequest.ResponseFormat` is `*components.ResponseFormat`.
- `components.CreateResponseFormatJSONSchema` sets `Type` to `json_schema`. Do not set that string yourself.
- `components.ChatChoice.Message` is a `ChatAssistantMessage` value, not a pointer. `choice.Message == nil` is a compile error. Delete that comparison.
- `ChatAssistantMessage.Content` is `optionalnullable.OptionalNullable[ChatAssistantMessageContent]`, not a pointer. Use `Get()`.
- The string arm is `content.Str`. The other arms are `ArrayOfChatContentItems` and `Any`.
- `openrouter.Pointer` exists. `optionalnullable.From` lives in `github.com/OpenRouterTeam/go-sdk/optionalnullable`.

Keep the struct, constructor, and every existing comment. The comments currently say the message is a `ChatMessages` and that the method returns the prompt. Leave that text. Put the comments on the surviving code as specified below.

Build the format before `Chat.Send`:

```go
	format := components.CreateResponseFormatJSONSchema(components.ChatFormatJSONSchemaConfig{
		JSONSchema: components.ChatJSONSchemaConfig{
			Name:        "grocery_flyer_matches",
			Description: openrouter.Pointer("Grocery items matched to flyer items"),
			Schema:      groceryFlyerMatchesSchema(),
			Strict:      optionalnullable.From(openrouter.Pointer(true)),
		},
	})
```

Pass `ResponseFormat: &format` on the existing `components.ChatRequest`. Keep the existing message construction.

Target body, with the existing comments kept:

```go
// Send sends a prompt to the underlying model and returns the raw response as a string.
func (op *OpenRouterProvider) Send(ctx context.Context, prompt string, model string) ([]GroceryFlyerMatch, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	format := components.CreateResponseFormatJSONSchema(components.ChatFormatJSONSchemaConfig{
		JSONSchema: components.ChatJSONSchemaConfig{
			Name:        "grocery_flyer_matches",
			Description: openrouter.Pointer("Grocery items matched to flyer items"),
			Schema:      groceryFlyerMatchesSchema(),
			Strict:      optionalnullable.From(openrouter.Pointer(true)),
		},
	})

	// The OpenRouter SDK's Chat.Send method returns a ChatResult
	res, err := op.client.Chat.Send(ctx, components.ChatRequest{
		Model: openrouter.Pointer(model),
		Messages: []components.ChatMessages{
			components.CreateChatMessagesUser(
				components.ChatUserMessage{
					Content: components.CreateChatUserMessageContentStr(prompt),
					Role:    components.ChatUserMessageRoleUser,
				},
			),
		},
		ResponseFormat: &format,
	}, nil)
	if err != nil {
		return nil, err
	}
	if res == nil || res.ChatResult == nil {
		return nil, errors.New("empty response from openrouter")
	}

	// The ChatResult contains Choices array with ChatChoice objects
	if len(res.ChatResult.Choices) == 0 {
		return nil, errors.New("no choices in openrouter response")
	}

	choice := res.ChatResult.Choices[0]
	// Each choice has a Message field which is a ChatMessages
	// Message.Content contains a Union type that we need to extract
	content, ok := choice.Message.GetContent().Get()
	if !ok || content == nil {
		return nil, errors.New("empty message content in openrouter response")
	}
	if content.Str == nil {
		encoded, marshalErr := json.Marshal(content)
		raw := ""
		if marshalErr == nil {
			raw = string(encoded)
		}
		return nil, fmt.Errorf("openrouter content is not a string: %w", &ModelResponseError{Provider: "openrouter", Raw: raw})
	}

	// Since we can't easily determine the type, we'll convert to string
	// The actual implementation depends on the OpenRouter SDK's structure
	// For now, return the prompt as a placeholder
	matches, err := decodeGroceryFlyerMatches("openrouter", *content.Str)
	if err != nil {
		return nil, fmt.Errorf("openrouter decode matches: %w", err)
	}
	return matches, nil
}
```

`fmt` is already imported and currently unused. This uses it. Add `encoding/json` and `optionalnullable`.

Do not call a live OpenRouter model to see if `openrouter/free` accepts `response_format`. If you are not making a live call, leave `Strict: true` in place. Do not add a fallback that drops the schema.

## Prompt writer

### Consumer interface

`internal/promptwriter/providerprompter.go` today:

```go
package promptwriter

type ProviderPrompter interface {
	// Sends a prompt to a model and returns response as a map[string]any
	Send(prompt, model string) (*string, error)
}
```

Keep the comment verbatim. Change only the method line, and add the imports `context` and `stfg/internal/provider`:

```go
	Send(ctx context.Context, prompt string, model string) ([]provider.GroceryFlyerMatch, error)
```

Do not delete this interface and do not make the prompt writer depend on `*OpenAIProvider`, `*OllamaProvider`, or `*OpenRouterProvider`.

### Prompt text

In `GetFlyerItemsOnGroceryList`, change only the response-structure example. Leave every rule and the typo "ALWAYS respond in with" alone.

Replace this example:

```text
[
  {
    "grocery_item": string,
	"flyer_item_id": number,
	"flyer_item_name": string
  }
]
```

with:

```text
{
  "matches": [
    {
      "grocery_item": string,
      "flyer_item_id": number,
      "flyer_item_name": string
    }
  ]
}
```

Do not rewrite the rest of the prompt.

### Method signature and loop

Change:

```go
func (o *OpenRouterFreePromptWriter) GetFlyerItemsOnGroceryList(flyerItems []storage.FlyerItem, groceryList []storage.GroceryItem) (map[string][]storage.FlyerItem, error)
```

to:

```go
func (o *OpenRouterFreePromptWriter) GetFlyerItemsOnGroceryList(ctx context.Context, flyerItems []storage.FlyerItem, groceryList []storage.FlyerItem) (map[string][]storage.FlyerItem, error)
```

Keep building `flyerJSON` and `groceryJSON` with `encoding/json`. That import stays. `stfg/internal` stays, because `internal.Contains` stays. `errors` and `zap` stay.

Remove the local `GroceryFlyerMatch` struct after its three comment lines have been copied into `types.go`. Remove the `jsonrepair` import. Remove the repair call, the unmarshal into `[]GroceryFlyerMatch`, and the two debug logs that print the raw and repaired JSON.

`Send` now returns the slice. A `Send` error, including a decode error, still aborts the whole method. Do not retry decode failures here. The existing comment on that branch says to abort right away.

The retry loop stays for one case: a schema-valid match whose id, name, and grocery name are not all present in the inputs. A schema cannot prove the model invented a real flyer row.

Target loop, comments included:

```go
RetryLoop:
	for range 3 {
		gfms, err := o.providerPrompter.Send(ctx, prompt, o.model)
		if err != nil {
			// Something bizarre happened, we should abort right away
			zap.S().Errorf("Error sending prompt: %v", err)
			return nil, fmt.Errorf("error sending prompt: %w", err)
		}

		zap.S().Debug("Repairs were fine, on to checking if things match")

		// Next check that they all map

		flyerItemsMatchingGroceries := map[string][]storage.FlyerItem{}
		for _, gfm := range gfms {

			matchFound := false
			for _, flyerItem := range flyerItems {
				if flyerItem.ID == gfm.FlyerItemID &&
					flyerItem.Name == gfm.FlyerItemName &&
					internal.Contains(groceryNames, gfm.GroceryItem) {

					// then this item is indeed a match!

					value, ok := flyerItemsMatchingGroceries[gfm.GroceryItem]
					if ok {
						value = append(value, flyerItem)
						flyerItemsMatchingGroceries[gfm.GroceryItem] = value
					} else {
						flyerItemsMatchingGroceries[gfm.GroceryItem] = []storage.FlyerItem{
							flyerItem,
						}
					}

					matchFound = true
					break
				}
			}

			if !matchFound {
				// This means there is a response item that doesn't belong to anything! We got illogical mappings!
				// Repairing was not possible, response was invalid. We should try again
				// Parsing the returned object was not possible. Response is invalid. We should try again
				zap.S().Debugf("No Match For: FlyerItemName %s |  FlyerItemId %d | GroceryItem %s", gfm.FlyerItemName, gfm.FlyerItemID, gfm.GroceryItem)
				// fix any bizarreness in the response
				continue RetryLoop
			}

		}

		// If we get this far then everything worked. Return the contents
		return flyerItemsMatchingGroceries, nil

	}

	// If we got here that means the retry loop ran out. We couldn't get anything valuable back from the LLM
	return nil, errors.New("Failed To Get Valid Response From LLM. Unable To Verify Grocery Items Are On Flyer")
```

Leave the final `errors.New` string exactly as it is. Leave the `Debugf` format string exactly as it is. Only the field selector changes from `FlyerItemId` to `FlyerItemID`.

The three comments that used to sit on the repair and unmarshal branches move onto the logical-mismatch `continue`. That is intentional. Do not delete them and do not rewrite them.

## find-deals wiring

`cmd/findDeals.go` calls `provider.NewClient(apiKey)`. That function does not exist. Do not create it. Do not add a registry.

This command is the OpenRouter free prompt writer. Construct the existing OpenRouter provider:

```go
		client, err := provider.NewOpenRouterProvider(apiKey)
		if err != nil {
			zap.S().Errorf("Error creating OpenRouter provider: %v", err)
			return
		}
		promptWriter := promptwriter.NewOpenRouterFreePromptWriter(client)
```

`NewOpenRouterProvider` currently returns `nil` error. Still handle the error. Do not validate the API key in this change. Do not rewrite the command's help text, flags, or `init()`.

Pass context at the only call site:

```go
			matches, err := promptWriter.GetFlyerItemsOnGroceryList(cmd.Context(), flyerItems, groceries)
```

Do not move matching logic into `cmd/`. The command already prints. Leave that printing alone.

## Comment placement

Existing comments and where they go:

| Comment | Action |
|---|---|
| Three lines above `GroceryFlyerMatch` in `openrouterfree.go`, starting `You are a helpful shopping assistant.` | Copy verbatim into `types.go` under the new one-line doc comment. Then remove them from the prompt writer with the struct. |
| `// Sends a prompt to a model and returns response as a map[string]any` | Leave in `providerprompter.go` |
| `// Send sends a prompt to the underlying model and returns the raw response as a string.` | Leave on the interface and on all three methods |
| `// Provider defines the contract...` | Leave in `protocols.go` |
| OpenRouter comments inside `Send`, including `// For now, return the prompt as a placeholder` | Keep verbatim in the target body above |
| `// Something bizarre happened, we should abort right away` | Stays on the `Send` error return |
| `// fix any bizarreness in the response` | Move to the logical-mismatch `continue` |
| `// Repairing was not possible, response was invalid. We should try again` | Move to that same `continue` |
| `// Parsing the returned object was not possible. Response is invalid. We should try again` | Move to that same `continue` |
| `// Next check that they all map` | Stays |
| `// then this item is indeed a match!` | Stays |
| `// This means there is a response item that doesn't belong to anything! We got illogical mappings!` | Stays |
| `// If we get this far then everything worked. Return the contents` | Stays |
| `// If we got here that means the retry loop ran out...` | Stays |
| `errors.go` package comment | Do not touch the file |

`zap.S().Debug("Repairs were fine, on to checking if things match")` is not a comment. Keep that call so the existing debug line does not disappear with the repair block.

## Tests

Tests are required, not optional. Use the standard `testing` package. Same-package tests. Hand-written fakes only. Do not export a helper so a test can call it. Do not start an HTTP server. Do not construct the OpenAI, Ollama, or OpenRouter SDK clients. Do not call a live model.

### Decoder

Create `internal/provider/decode_test.go` in `package provider`. Three direct tests, not a table:

1. `{"matches":[{"grocery_item":"milk","flyer_item_id":42,"flyer_item_name":"2% Milk"}]}` returns one element with those values. Assert `FlyerItemID`, not a field named `FlyerItemId`.
2. `{"matches":[]}` returns a non-nil empty slice.
3. `{`, `{}`, and `{"matches":null}` each return an error that `errors.As` matches to `*ModelResponseError`.

Call `decodeGroceryFlyerMatches("openai", raw)`.

### Prompt writer

Create `internal/promptwriter/openrouterfree_test.go` in `package promptwriter`. A fake in that file implements `ProviderPrompter`. It records the prompt and returns scripted slices or an error. It does not parse JSON.

`FlyerItem` lives in `stfg/internal/storage` and has `ID int64` and `Name string`. `GroceryItem` has `Name string`. Build the smallest values those tests need.

Three direct tests:

1. The fake returns one `provider.GroceryFlyerMatch` whose grocery name, flyer id, and flyer name all exist in the inputs. `GetFlyerItemsOnGroceryList` returns that grocery key mapped to that flyer item. The fake was called once. The prompt contains `"matches"` and does not ask for a bare JSON array as the response example.
2. The fake returns a match whose `FlyerItemID` is not in the flyer slice, three times. The method calls `Send` three times, then returns the existing error string `Failed To Get Valid Response From LLM. Unable To Verify Grocery Items Are On Flyer`.
3. The fake returns an error on the first `Send`. The method returns immediately, wrapped with `error sending prompt`. The fake was called once.

Pass `context.Background()` from the tests. Do not assert on zap output.

`internal/storage/storage_test.go` counts `type FlyerItem struct` and `type GroceryItem struct` inside the storage package. Do not add another copy of those structs.

## Documentation

Update the living docs so a later agent does not revive `*string` responses, `jsonrepair`, or `provider.NewClient`. Do not edit `docs/PROJECT_CONTEXT.md`. It still correctly says find-deals uses an LLM. Do not edit `README.md`.

### `docs/ARCHITECTURE.md`

This file is the current-code snapshot. Update only these spots:

- Under Models, change the PromptWriter bullet so it says the prompt writer builds the flyer and grocery prompt, sends it through a `Provider`, and checks returned matches against the real flyer rows.
- Replace the Providers section so it lists OpenAI, Ollama, and OpenRouter as implementations of `internal/provider.Provider`. Say `Send` returns `[]GroceryFlyerMatch` decoded from a shared `{"matches":[...]}` schema. Keep the Flipp API bullet. Flipp is not an LLM provider; leave it listed where it already is rather than moving the whole section.
- In Data Flow step 3, say `find-deals` constructs `NewOpenRouterProvider` and asks the prompt writer to match flyer items to the grocery list. Do not say it uses embeddings for that match. Do not say it calls `provider.NewClient`.
- In the directory tree, change the provider comment from `OpenRouter LLM provider` to `OpenAI, Ollama, and OpenRouter LLM providers`.
- In Technology Stack, add the three LLM SDKs next to External Services. Remove any implication that JSON repair is part of the current path. Do not mention `jsonrepair`.

Do not redraw unrelated storage or embedding sections.

### `docs/DECISIONS.md`

Add one new section. Do not rewrite the existing LLM-Based Matching section; that decision still explains why an LLM is used. Append:

```markdown
## Structured Provider Matches

**Decision:** `Provider.Send` returns `[]GroceryFlyerMatch` for every backend. The model is asked for one JSON object, `{"matches":[...]}`, not a bare array and not free text. OpenAI, Ollama, and OpenRouter all receive that same schema. The prompt writer no longer repairs or unmarshals model text. It still rejects a match that is not present in the flyer and grocery inputs. `find-deals` calls `NewOpenRouterProvider` directly.

**Reason:** The only model call in the program has one response shape. A generic `Send[T]` was rejected. OpenAI strict structured outputs need a root object, so the wire format is an envelope even though callers receive the inner slice. Decoding lives next to the schema so the three SDKs cannot drift. `jsonrepair` was removed because providers now decode, and a repair step was hiding malformed responses instead of returning `ModelResponseError`.
```

### `AGENTS.md`

Update the existing rows. Do not add a new architecture essay.

- In the project tree, change the provider comment to `OpenAI, Ollama, and OpenRouter`.
- In Finding Deals, replace the embedding-matching step with: load groceries and flyers, call `NewOpenRouterProvider`, and let the prompt writer match through `Provider.Send`, which returns decoded `[]GroceryFlyerMatch`.
- In Key Dependencies, add `github.com/openai/openai-go/v3` and `github.com/liliang-cn/ollama-go`. Delete the `github.com/kaptinlin/jsonrepair` row.
- In File Locations, add a row: structured match type and schema live in `internal/provider/types.go`.

Leave the llama.cpp build notes alone.

## Out of scope

- A second structured call, a schema parameter, or a generic send method.
- A JSON-schema code generator or any new dependency.
- Flyer scraping, storage, embeddings, or reconciler changes.
- `provider.NewClient`, a provider registry, or wiring Ollama and OpenAI into `find-deals`.
- Retries inside provider methods. The prompt writer already retries bad matches.
- Streaming.
- Live calls to OpenAI, OpenRouter, or Ollama.
- `make`, or anything that builds `llama.cpp`.
- Editing `docs/PROJECT_CONTEXT.md`, `README.md`, or plans other than committing this spec.
- Implementing in the primary checkout.

## Order

1. Create the worktree and copy this plan into it, as specified above. `cd` there before editing.
2. Write `internal/provider/types.go`.
3. Change `protocols.go` and the three `Send` methods together.
4. Update `providerprompter.go` and `openrouterfree.go` together, including the comment moves.
5. Update the two call sites in `cmd/findDeals.go`.
6. Add `decode_test.go` and `openrouterfree_test.go`.
7. Update `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, and `AGENTS.md`.
8. Run `gofmt` on every Go file you touched.
9. Run `go mod tidy` because `jsonrepair` is no longer imported.
10. Run the checks below from the worktree.
11. Commit, push, and open the pull request. Do this only after the checks pass.

## Checks

From the worktree:

```bash
go test ./internal/provider/ ./internal/promptwriter/
go build -o /dev/null ./internal/provider/ ./internal/promptwriter/ ./cmd/
```

Do not run `make`. Do not run `go test ./...` if it starts the llama.cpp build. These two commands are the gate.

## Commits and pull request

Three commits, in this order. Tests and docs are never in the source commit.

1. Source only. No `*_test.go`. No `docs/`. No `AGENTS.md`.

```text
feat(provider): return structured grocery-flyer matches

Send used to return raw model text, and the prompt writer repaired it
with jsonrepair. Each provider now requests the same matches schema and
decodes the content itself. find-deals calls NewOpenRouterProvider
because provider.NewClient does not exist and a provider switch is out
of scope.
```

Stage `internal/provider/protocols.go`, `internal/provider/types.go`, `internal/provider/openai.go`, `internal/provider/ollama.go`, `internal/provider/openrouter.go`, `internal/promptwriter/providerprompter.go`, `internal/promptwriter/openrouterfree.go`, `cmd/findDeals.go`, and the `go.mod` / `go.sum` tidy result if `jsonrepair` was removed.

2. Tests only.

```text
test(provider): cover match decoding and prompt-writer retries

The decoder is the only shared parse path, and the prompt writer still
has to reject ids the schema cannot prove. These tests lock both
without calling a model.
```

Stage `internal/provider/decode_test.go` and `internal/promptwriter/openrouterfree_test.go`.

3. Docs only, including this plan.

```text
docs: record structured provider matches

ARCHITECTURE, DECISIONS, and AGENTS still described a raw OpenRouter
string and jsonrepair. Update them to the concrete match slice and the
OpenRouter wiring find-deals actually uses.
```

Stage `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, `AGENTS.md`, and `docs/plans/07-provider-structured-matches.md`.

If a commit would exceed 500 changed lines, split that group by file and keep the same type prefix. Do not move a test file into the feat commit to balance the count.

Push without force:

```bash
git push -u origin HEAD
```

Open the pull request against `main`:

```bash
gh pr create --base main --title "feat, test, docs: return structured grocery-flyer matches" --body "$(cat <<'EOF'
### Added
- `GroceryFlyerMatch` and `GroceryFlyerMatches` in `internal/provider/types.go`
- Shared JSON schema and `decodeGroceryFlyerMatches`
- Structured-output requests for OpenAI, Ollama, and OpenRouter
- Decoder tests and prompt-writer tests with a hand-written fake
- Decision note for the concrete match type and the `matches` envelope

### Removed
- `jsonrepair` from the prompt writer and from `go.mod` when tidy drops it
- Raw `*string` provider responses
- The call to missing `provider.NewClient`

### Updated
- `Provider.Send` now returns `[]GroceryFlyerMatch` and takes `context.Context`
- Prompt writer validates decoded matches instead of repairing JSON
- `find-deals` uses `NewOpenRouterProvider` and `cmd.Context()`
- `docs/ARCHITECTURE.md` and `AGENTS.md` describe the three providers and the decoded match slice

### Deleted
- None
EOF
)"
```

If `gh` is not authenticated, stop after the push and report the branch URL. Do not print or invent a token.

The pull request title lists every commit type in the required order: `feat`, `test`, `docs`. If you add another commit type later, update the title. Do not force-push to fix the title; `gh pr edit` is enough.

## Done when

- The worktree is `/home/bsoer/Documents/PROJECTS/stfg-structured-provider-matches` on branch `structured-provider-matches`.
- The primary checkout was not used for the implementation commits.
- `go test ./internal/provider/ ./internal/promptwriter/` passes in the worktree.
- `go build -o /dev/null ./internal/provider/ ./internal/promptwriter/ ./cmd/` passes in the worktree.
- `Provider.Send` returns `[]GroceryFlyerMatch`.
- All three SDK requests carry `groceryFlyerMatchesSchema()`.
- OpenRouter no longer returns `&prompt`, and `choice.Message == nil` is gone.
- The prompt writer no longer imports `jsonrepair` and no longer calls `json.Unmarshal` on a model response.
- The prompt writer still rejects a match that is not in the flyer items and grocery names.
- `cmd/findDeals.go` uses `NewOpenRouterProvider` and passes `cmd.Context()`.
- `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, and `AGENTS.md` match the Documentation section.
- Three commits exist, separated into source, tests, and docs.
- The branch is pushed to `origin` and a pull request is open against `main`, unless `gh` auth failed and that failure is reported with the branch name.
- No generic type parameter was added.
- No existing comment text was reworded or deleted. Moved comments still match the table above.
- No amend, force push, or rebase was used.
