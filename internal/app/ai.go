// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"strings"
	"thaw/internal/ai"
	"thaw/internal/config"
	"thaw/internal/fnmeta"
	"thaw/internal/logger"
	"thaw/internal/secrets"
)

// ListAIModels returns the models available for the given provider and API key.
// Returns nil (not an error) when the key is invalid or the request fails so
// the frontend can fall back to its static defaults.
// ollamaPort is the Ollama server port (0 = default 11434); ignored for other providers.
func (a *App) ListAIModels(provider, apiKey string, ollamaPort int) []string {
	models, err := ai.ListModels(provider, apiKey, ollamaPort)
	if err != nil {
		logger.L.Warn("failed to list AI models", "provider", provider, "err", err)
		return nil
	}
	return models
}

// TestAIModel makes a minimal one-token API call to verify that the given
// provider/key/model combination is valid and reachable.
// Returns an empty string on success or a human-readable error message.
// ollamaPort is the Ollama server port (0 = default 11434); ignored for other providers.
// ollamaNumCtx mirrors the configured context window so the test uses the same
// load path as real inference (important for large models like Gemma 4).
func (a *App) TestAIModel(provider, apiKey, model string, ollamaPort, ollamaNumCtx int) string {
	if err := ai.TestModel(provider, apiKey, model, ollamaPort, ollamaNumCtx); err != nil {
		return err.Error()
	}
	return ""
}

// GetAISuggestion calls the configured AI provider and returns an inline SQL
// completion for the given prefix text. Returns an empty string when AI is
// disabled, when no API key is set (non-Ollama), or when the provider returns an error.
func (a *App) GetAISuggestion(prefix string) string {
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	// The API key lives in the OS secure store, not config.json.
	apiKey, _ := secrets.Get(secrets.KeyAIAPIKey)
	if !cfg.AI.Enabled || (cfg.AI.Provider != "ollama" && apiKey == "") {
		return ""
	}

	prompt := "Complete this Snowflake SQL query. Return ONLY the completion text to insert at the cursor — no explanation, no markdown, no repetition of existing text. Keep it to 1–2 lines.\n\n" + prefix

	suggestion, err := ai.GetSuggestion(cfg.AI.Provider, apiKey, cfg.AI.Model, prompt, cfg.AI.OllamaPort, cfg.AI.OllamaNumCtx)
	if err != nil {
		logger.L.Debug("AI suggestion failed", "provider", cfg.AI.Provider, "err", err)
		return ""
	}
	return suggestion
}

// GetFunctionSuggestions returns up to 50 Snowflake functions whose name
// starts with prefix (case-insensitive). It reads the local SQLite cache so
// results are available instantly, even before a connection is established.
func (a *App) GetFunctionSuggestions(prefix string) ([]fnmeta.FunctionMeta, error) {
	if a.fnStore == nil {
		return nil, nil
	}
	return a.fnStore.Search(strings.ToUpper(prefix))
}

// GetAllFunctionNames returns every distinct function name and type in the
// local SQLite cache. Used by the editor to build its decoration/highlight set.
func (a *App) GetAllFunctionNames() ([]fnmeta.FunctionMeta, error) {
	if a.fnStore == nil {
		return nil, nil
	}
	return a.fnStore.GetAllNames()
}

// GetFunctionTooltip returns all overloads for the given Snowflake function
// name. The name is matched case-insensitively via an exact lookup in the
// local SQLite cache.
func (a *App) GetFunctionTooltip(name string) ([]fnmeta.FunctionMeta, error) {
	if a.fnStore == nil {
		return nil, nil
	}
	return a.fnStore.Lookup(strings.ToUpper(name))
}
