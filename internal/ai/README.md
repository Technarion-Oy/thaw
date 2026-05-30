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
| `ai.go` | All exported API surface: `GetSuggestion`, `ListModels`, `TestModel`, plus internal provider implementations |
| `doc.go` | Package doc and `// thaw:domain: AI Tooling` annotation |

## Key types & functions

```go
// ai.go:33
func GetSuggestion(provider, apiKey, model, prompt string, ollamaPort, ollamaNumCtx int) (string, error)

// ai.go:105
func ListModels(provider, apiKey string, ollamaPort int) ([]string, error)

// ai.go:230
func TestModel(provider, apiKey, model string, ollamaPort, ollamaNumCtx int) error
```

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

- This package has no connection to `internal/snowflake`. It is a pure HTTP client layer — no Wails context, no `*App` receiver.
- The Ollama suggestion path uses `ollamaHttpClient` (15 s) while listing uses the shorter `httpClient` (3 s); do not swap them or listing will hang on cold model loads.
- `listOpenAIModels` filters by the `"gpt-"` prefix. Newer OpenAI model families (e.g. `o1-*`) are intentionally excluded unless that filter is updated.
