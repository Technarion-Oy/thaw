# internal/ai

> HTTP clients for LLM providers (OpenAI, Google Gemini, Ollama) used to serve inline SQL completions and model management from the Thaw backend.

## Responsibility

- Request inline SQL completions from a configured AI provider.
- List available models for each provider (filtered to chat-capable ones).
- Test that a given provider / API key / model combination is reachable and valid.
- Route all three operations through a single `provider` string so callers never need to import provider-specific logic.

## Key files

| File | Purpose |
|------|---------|
| `ai.go` | All exported API surface: `GetSuggestion`, `Sanitize`, `ListModels`, `TestModel`, plus internal provider implementations |
| `schema.go` | `SchemaContext` types, `BuildPrompt`, and `SchemaBlock` / `SchemaCharBudget` — the schema block prepended to completion prompts (#924) |
| `schema_test.go` | Formatting, quoting/qualification, per-table column truncation, budget cutoff, and that `BuildPrompt` sends nothing from the catalog when schema context is off |
| `sanitize_test.go` | `Sanitize`: fence stripping (closed and unclosed) and prefix-repetition removal (#926) |
| `doc.go` | Package doc and `// thaw:domain: AI Tooling` annotation |

## Key types & functions

```go
// ai.go:33
func GetSuggestion(provider, apiKey, model, prompt, prefix string, ollamaPort, ollamaNumCtx int) (string, error)
func Sanitize(prefix, reply string) string

// ai.go:105
func ListModels(provider, apiKey string, ollamaPort int) ([]string, error)

// ai.go:230
func TestModel(provider, apiKey, model string, ollamaPort, ollamaNumCtx int) error

// schema.go
func BuildPrompt(prefix string, ctx SchemaContext, numCtx int, includeSchema bool) string
func SchemaBlock(ctx SchemaContext, charBudget int) string
func SchemaCharBudget(numCtx int) int
```

**Reply sanitising (#926):** models answer the completion prompt as a *question* and reply
chat-style — the whole statement, prefix included, inside a ```` ```sql ```` fence. `GetSuggestion`
therefore runs every provider's reply through `Sanitize`: take the first fenced block when there
is one (tolerating a fence cut off by the 150-token limit), then drop the longest suffix of the
prefix that the reply restates, case-insensitively. It is one sanitiser on the shared path
because both defects are provider-independent — a per-provider fix would be three of them.
Hence `GetSuggestion` takes `prefix` as well as `prompt`.

**Prompt assembly:** `BuildPrompt` is the whole inline-completion prompt — schema block,
standing instruction, then the text before the cursor. `includeSchema` is an explicit
parameter rather than something inferred from an empty context: whether the catalog may be
sent is a privacy decision, so it is one testable flag rather than an emergent property.

**Schema block:** `SchemaContext` is what the editor already knows about the statement under
the cursor — the resolved table refs with their cached columns (name + data type) and foreign
keys. `SchemaBlock` renders it as DDL-like lines that every model completes well against:

```
-- Schema context
ORDERS(ID NUMBER, CUSTOMER_ID NUMBER)
-- ORDERS.CUSTOMER_ID -> CUSTOMERS.ID
CUSTOMERS(ID NUMBER, EMAIL VARCHAR)
```

Tables arrive last-referenced first and are emitted until `charBudget` would be exceeded
(by the trailing blank line, at most one byte); a table past `maxColumnsPerTable` (40) columns
is truncated with a `…(+k more)` marker, and a table with no cached columns is skipped entirely
(its bare name tells the model nothing the prefix doesn't). Identifiers that Snowflake would not
fold to themselves keep their double quotes, so a case-sensitive name (#922) reaches the model
spelled the way it must be typed; a name appearing in two schemas is qualified, and only then.
`SchemaCharBudget` derives the budget from the Ollama context window (~4 chars/token, so
`numCtx` chars ≈ a quarter of the window), defaulting to 4096 for hosted providers.

`SchemaTable` carries no object kind: views, dynamic, external and iceberg tables all render
identically, so nothing downstream would read it.

**Provider routing:** each exported function switches on `provider` ("openai" | "google" | "ollama") and calls a private implementation.

**HTTP clients:**
- `httpClient` — 3 s timeout, used for completions and model listing.
- `ollamaHttpClient` — 15 s timeout, used for Ollama completions (local inference is slower).
- `testHttpClient` — 10 s timeout, used for `TestModel` calls (quick enough for UI feedback).

**Model filtering:**
- OpenAI: retains only IDs starting with `"gpt-"`.
- Google: retains models that support `generateContent` and contain `"gemini"` in the name.
- Ollama: returns all names from `/api/tags`, sorted.

## Patterns & integration

- Called exclusively from `internal/app/ai.go` thin delegator methods (`GetAISuggestion`, `ListAIModels`, `TestAIModel`), which read provider config from `config.AIConfig` and forward it here.
- The `ollamaPort` and `ollamaNumCtx` parameters are ignored for non-Ollama providers; 0 means use Ollama defaults (port 11434, model-default context window).
- Completions request a maximum of 150 output tokens; test requests use 1 token for fast round-trips.
- Google Gemini requests set `temperature: 0.2` for deterministic completions.

## Gotchas

- This package has no connection to `internal/snowflake`. It is a pure HTTP client layer — no Wails context, no `*App` receiver. `schema.go` is pure formatting: it never fetches the schema it renders, the caller passes what it already has.
- The schema-context switch is a **tri-state** in config (`config.AIConfig.SchemaContext *bool`); `SchemaContextEnabled()` owns the unset case — off for an install that already had a hosted provider (upgrading must not start sending column names to OpenAI/Google unasked), on for Ollama. `GetAIConfig` resolves it before the modal sees it, so saving records an explicit choice.
- The Ollama suggestion path uses `ollamaHttpClient` (15 s) while listing uses the shorter `httpClient` (3 s); do not swap them or listing will hang on cold model loads.
- `Sanitize` matches the prefix before right-trimming the reply: the prefix ends in whatever whitespace the user typed, so trimming first would stop a pure echo (`"select "` → `"select "`) from being recognised as a repetition. The prompt hardening (fences forbidden, prefix labelled `-- SQL before the cursor`) is a hint, not a guarantee — small local models ignore it, which is why the sanitiser is the actual fix.
- `listOpenAIModels` filters by the `"gpt-"` prefix. Newer OpenAI model families (e.g. `o1-*`) are intentionally excluded unless that filter is updated.
