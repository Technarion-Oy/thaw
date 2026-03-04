// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 5 * time.Second}

// GetSuggestion requests an inline SQL completion from the configured provider.
// provider must be "openai" or "google". Returns the trimmed completion text.
func GetSuggestion(provider, apiKey, model, prompt string) (string, error) {
	switch provider {
	case "openai":
		return openAISuggestion(apiKey, model, prompt)
	case "google":
		return googleSuggestion(apiKey, model, prompt)
	default:
		return "", fmt.Errorf("unknown AI provider: %s", provider)
	}
}

// ── OpenAI ────────────────────────────────────────────────────────────────────

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
	defer resp.Body.Close()

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
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

// ── Model listing ─────────────────────────────────────────────────────────────

// ListModels returns the models available to the given API key for the
// specified provider. provider must be "openai" or "google".
func ListModels(provider, apiKey string) ([]string, error) {
	switch provider {
	case "openai":
		return listOpenAIModels(apiKey)
	case "google":
		return listGoogleModels(apiKey)
	default:
		return nil, fmt.Errorf("unknown AI provider: %s", provider)
	}
}

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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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

// ── Google AI Studios (Gemini) ────────────────────────────────────────────────

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
	defer resp.Body.Close()

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
	return strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text), nil
}
