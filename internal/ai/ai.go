// SPDX-License-Identifier: GPL-3.0-or-later

package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 3 * time.Second}

var ollamaHttpClient = &http.Client{Timeout: 15 * time.Second}

// GetSuggestion requests an inline SQL completion from the configured provider.
// provider must be "openai", "google", or "ollama". Returns the completion text
// ready to insert at the cursor: every provider's reply goes through Sanitize,
// so prefix must be the text before the cursor that prompt was built from.
// ollamaPort is the port number for the local Ollama instance (0 = default 11434);
// ollamaNumCtx is the context window size sent to Ollama (0 = let Ollama decide);
// both are ignored for non-Ollama providers.
func GetSuggestion(provider, apiKey, model, prompt, prefix string, ollamaPort, ollamaNumCtx int) (string, error) {
	var (
		raw string
		err error
	)
	switch provider {
	case "openai":
		raw, err = openAISuggestion(apiKey, model, prompt)
	case "google":
		raw, err = googleSuggestion(apiKey, model, prompt)
	case "ollama":
		raw, err = ollamaSuggestion(ollamaBaseURL(ollamaPort), model, prompt, ollamaNumCtx)
	default:
		return "", fmt.Errorf("unknown AI provider: %s", provider)
	}
	if err != nil {
		return "", err
	}
	return Sanitize(prefix, raw), nil
}

// fencedBlock matches the first markdown code fence in a reply and captures its
// body, tolerating a missing closing fence (the reply can be cut off by the
// token limit mid-block).
var fencedBlock = regexp.MustCompile("(?s)```(?:[a-zA-Z]*\\n)?(.*?)(?:```|$)")

// Sanitize turns a chat-style reply into text that can be inserted at the cursor.
// Models answer the completion prompt as a question — a whole statement inside a
// ```sql fence — so it takes the first fenced block when there is one, then drops
// the longest suffix of prefix that the reply restates (case-insensitively).
// Both defects are independent of the provider, hence one sanitiser on the shared path.
func Sanitize(prefix, reply string) string {
	if m := fencedBlock.FindStringSubmatch(reply); m != nil {
		reply = m[1]
	}
	// Left-trim before matching, right-trim only after: the prefix ends in the
	// whitespace the user typed, so an echo of it must still match. This is why
	// the providers hand over their text untrimmed.
	body := strings.TrimLeft(reply, " \t\r\n")
	for i := range prefix {
		// Only a suffix starting at a token boundary is a repetition. Without
		// this, prefix "…from ac" + completion "count" reads as a repeated "c"
		// and inserts "acount".
		if i > 0 && isWordByte(prefix[i-1]) && isWordByte(prefix[i]) {
			continue
		}
		suffix := prefix[i:]
		// Byte-length compare: EqualFold folds per rune, so the handful of
		// characters that change UTF-8 length when folded (e.g. the Kelvin sign)
		// simply do not match. Identifiers that reach here are SQL text.
		if len(body) >= len(suffix) && strings.EqualFold(body[:len(suffix)], suffix) {
			return strings.TrimRight(body[len(suffix):], " \t\r\n")
		}
	}
	// No repetition: the model is continuing the prefix, so keep one separator
	// when it sent leading whitespace and the user's text does not end in any.
	if len(body) < len(reply) && prefix != "" && !isSpaceByte(prefix[len(prefix)-1]) {
		body = " " + body
	}
	return strings.TrimRight(body, " \t\r\n")
}

// isWordByte reports whether b can occur inside a SQL identifier.
func isWordByte(b byte) bool {
	return b == '_' || b == '$' ||
		b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

// ── OpenAI ────────────────────────────────────────────────────────────────────

// openAISuggestion requests an inline SQL completion from the OpenAI Chat
// Completions API. It returns the response text untrimmed (Sanitize owns
// trimming: it must see the whitespace the model sent) or an error.
func openAISuggestion(apiKey, model, prompt string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 150,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai: status %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", nil
	}
	return result.Choices[0].Message.Content, nil
}

// ── Model listing ─────────────────────────────────────────────────────────────

// ListModels returns the models available for the specified provider.
// provider must be "openai", "google", or "ollama".
// ollamaPort is the port number for the local Ollama instance (0 = default 11434);
// it is ignored for non-Ollama providers.
func ListModels(provider, apiKey string, ollamaPort int) ([]string, error) {
	switch provider {
	case "openai":
		return listOpenAIModels(apiKey)
	case "google":
		return listGoogleModels(apiKey)
	case "ollama":
		return listOllamaModels(ollamaBaseURL(ollamaPort))
	default:
		return nil, fmt.Errorf("unknown AI provider: %s", provider)
	}
}

// listOpenAIModels fetches the available models from the OpenAI /v1/models
// endpoint and returns only GPT chat-capable model IDs, sorted alphabetically.
func listOpenAIModels(apiKey string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: status %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Data {
		// Keep only chat-capable GPT models; exclude embeddings, TTS, etc.
		if strings.HasPrefix(m.ID, "gpt-") {
			models = append(models, m.ID)
		}
	}
	sort.Strings(models)
	return models, nil
}

// listGoogleModels fetches Gemini models from the Google Generative Language
// API and returns only those that support generateContent, sorted alphabetically.
func listGoogleModels(apiKey string) ([]string, error) {
	url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + apiKey

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: status %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Models []struct {
			Name                       string   `json:"name"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Models {
		// Only include models that support generateContent.
		supportsGenerate := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsGenerate = true
				break
			}
		}
		if !supportsGenerate {
			continue
		}
		// Strip the "models/" prefix; the API URL uses just the model ID.
		id := strings.TrimPrefix(m.Name, "models/")
		if strings.Contains(id, "gemini") {
			models = append(models, id)
		}
	}
	sort.Strings(models)
	return models, nil
}

// ── Model testing ─────────────────────────────────────────────────────────────

// testHttpClient has a 10-second timeout — long enough for a real API response
// but short enough to give quick feedback in the settings dialog.
var testHttpClient = &http.Client{Timeout: 10 * time.Second}

// TestModel sends a minimal one-token request to verify that the given
// provider / API key / model combination is reachable and valid.
// Returns nil on success or a user-readable error.
// ollamaPort is the port number for the local Ollama instance (0 = default 11434);
// ollamaNumCtx is the context window size sent to Ollama (0 = let Ollama decide);
// both are ignored for non-Ollama providers.
func TestModel(provider, apiKey, model string, ollamaPort, ollamaNumCtx int) error {
	switch provider {
	case "openai":
		return testOpenAIModel(apiKey, model)
	case "google":
		return testGoogleModel(apiKey, model)
	case "ollama":
		return testOllamaModel(ollamaBaseURL(ollamaPort), model, ollamaNumCtx)
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}
}

// testOpenAIModel sends a minimal one-token request to the OpenAI Chat
// Completions API to verify that the key and model are valid.
func testOpenAIModel(apiKey, model string) error {
	body, err := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
		"max_tokens": 1,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testHttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &errResp) == nil && errResp.Error.Message != "" {
			return fmt.Errorf("%s", errResp.Error.Message)
		}
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// testGoogleModel sends a minimal one-token request to the Google Gemini
// generateContent API to verify that the key and model are valid.
func testGoogleModel(apiKey, model string) error {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, apiKey,
	)
	body, err := json.Marshal(map[string]any{
		"contents":         []map[string]any{{"parts": []map[string]string{{"text": "hi"}}}},
		"generationConfig": map[string]any{"maxOutputTokens": 1},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := testHttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &errResp) == nil && errResp.Error.Message != "" {
			return fmt.Errorf("%s", errResp.Error.Message)
		}
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// ── Ollama (local) ────────────────────────────────────────────────────────────

// ollamaBaseURL returns the Ollama API base URL for the configured port.
// If port is 0 or negative the default port 11434 is used.
func ollamaBaseURL(port int) string {
	if port <= 0 {
		return "http://localhost:11434"
	}
	return fmt.Sprintf("http://localhost:%d", port)
}

// listOllamaModels fetches locally available models from the Ollama /api/tags endpoint.
func listOllamaModels(base string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, base+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama not reachable: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, raw)
	}
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, m.Name)
	}
	sort.Strings(models)
	return models, nil
}

// testOllamaModel sends a minimal generate request to verify the model is available.
// numCtx mirrors the context-window setting the user configured so the test
// exercises the same load path as real inference (important for Gemma 4 etc.).
func testOllamaModel(base, model string, numCtx int) error {
	payload := map[string]any{
		"model":       model,
		"prompt":      "hi",
		"stream":      false,
		"num_predict": 1,
	}
	if numCtx > 0 {
		payload["options"] = map[string]any{"num_ctx": numCtx, "keep_alive": 300}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := testHttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama not reachable: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("%s", errResp.Error)
		}
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// ollamaSuggestion requests an inline SQL completion from the local Ollama
// /api/generate endpoint. The response text is returned untrimmed (see openAISuggestion).
// numCtx sets the context window size; 0 means use Ollama's default.
func ollamaSuggestion(base, model, prompt string, numCtx int) (string, error) {
	payload := map[string]any{
		"model":       model,
		"prompt":      prompt,
		"stream":      false,
		"num_predict": 150,
	}
	if numCtx > 0 {
		payload["options"] = map[string]any{"num_ctx": numCtx}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ollamaHttpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama not reachable: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama: status %d: %s", resp.StatusCode, raw)
	}
	var result struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	return result.Response, nil
}

// ── Google AI Studios (Gemini) ────────────────────────────────────────────────

// googleSuggestion requests an inline SQL completion from the Google Gemini
// generateContent API. It returns the response text untrimmed (see openAISuggestion).
func googleSuggestion(apiKey, model, prompt string) (string, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, apiKey,
	)

	body, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": prompt}}},
		},
		"generationConfig": map[string]any{
			"maxOutputTokens": 150,
			"temperature":     0.2,
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google: status %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", nil
	}
	return result.Candidates[0].Content.Parts[0].Text, nil
}
