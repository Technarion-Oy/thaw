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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 3 * time.Second}

var chatHttpClient = &http.Client{Timeout: 60 * time.Second}

var suggestHttpClient = &http.Client{Timeout: 15 * time.Second}

// UIToolCall holds one tool invocation and its result, for the frontend.
type UIToolCall struct {
	Name    string `json:"name"`
	Input   string `json:"input"`  // raw JSON the AI sent
	Output  string `json:"output"` // formatted result or error
	IsError bool   `json:"isError"`
}

// UIMessage is the display-facing message format shared between Go and the frontend.
type UIMessage struct {
	Role      string       `json:"role"` // "user" | "assistant"
	Text      string       `json:"text"`
	ToolCalls []UIToolCall `json:"toolCalls,omitempty"`
}

// ToolExecutor is called by Chat to run a tool by name with its JSON input.
// Returns (output text, isError).
type ToolExecutor func(name, inputJSON string) (string, bool)

// chatTools is the tool definitions sent to both providers.
var chatTools = []map[string]any{
	{
		"name":        "get_session_context",
		"description": "Return the current Snowflake session context: active role, warehouse, database, and schema. Call this first whenever you need to know where you are before listing schemas or tables.",
	},
	{
		"name":        "list_databases",
		"description": "List all databases the current role can access.",
	},
	{
		"name":        "list_schemas",
		"description": "List all schemas in a database.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"database": map[string]any{"type": "string", "description": "Database name"},
			},
			"required": []string{"database"},
		},
	},
	{
		"name":        "list_tables",
		"description": "List all tables and views in the given database.schema.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"database": map[string]any{"type": "string", "description": "Database name"},
				"schema":   map[string]any{"type": "string", "description": "Schema name"},
			},
			"required": []string{"database", "schema"},
		},
	},
	{
		"name":        "describe_table",
		"description": "Return each column's name and data type for a table or view.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"database": map[string]any{"type": "string", "description": "Database name"},
				"schema":   map[string]any{"type": "string", "description": "Schema name"},
				"table":    map[string]any{"type": "string", "description": "Table or view name"},
			},
			"required": []string{"database", "schema", "table"},
		},
	},
	{
		"name":        "run_sql",
		"description": "Execute a SQL query. Returns up to 50 rows as a plain-text table.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "The SQL query to execute"},
			},
			"required": []string{"query"},
		},
	},
	{
		"name":        "list_directory",
		"description": "List files and subdirectories in a directory on the local file system. Use \".\" to list the project root.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Directory path relative to the project root, or an absolute path."},
			},
			"required": []string{"path"},
		},
	},
	{
		"name":        "read_file",
		"description": "Read the text content of a local file. Use this to inspect SQL files, configurations, and other text files in the project.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "File path relative to the project root, or an absolute path."},
			},
			"required": []string{"path"},
		},
	},
	{
		"name":        "run_command",
		"description": "Run a shell command in the project working directory. Use this to run scripts, git commands, build tools, etc. Returns combined stdout and stderr.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "The shell command to execute, e.g. \"git log --oneline -10\" or \"ls -la\"."},
			},
			"required": []string{"command"},
		},
	},
}

// Chat runs one chat turn.
// When agentMode is true the AI may invoke tools (explore schema, run SQL) over
// up to 8 iterations. When false a single plain-text response is returned with
// no tool access.
// ollamaPort is the port number for the local Ollama instance (0 = default 11434);
// ollamaNumCtx is the context window size sent to Ollama (0 = let Ollama decide);
// both are ignored for non-Ollama providers.
func Chat(ctx context.Context, provider, apiKey, model string, ollamaPort, ollamaNumCtx int, history []UIMessage, userText, currentSQL, lastResultSummary string, agentMode bool, workDir string, exec ToolExecutor) (UIMessage, error) {
	switch provider {
	case "openai":
		return openAIChat(ctx, apiKey, model, history, userText, currentSQL, lastResultSummary, agentMode, workDir, exec)
	case "google":
		return googleChat(ctx, apiKey, model, history, userText, currentSQL, lastResultSummary, agentMode, workDir, exec)
	case "ollama":
		return ollamaChat(ctx, ollamaBaseURL(ollamaPort), model, ollamaNumCtx, history, userText, currentSQL, lastResultSummary, agentMode, workDir, exec)
	default:
		return UIMessage{}, fmt.Errorf("unknown AI provider: %s", provider)
	}
}

// buildSystemPrompt returns the system-level instruction block injected at the
// start of every chat request. It adapts based on whether agent mode (tool
// access) is active and includes the current SQL and last result for context.
func buildSystemPrompt(currentSQL, lastResultSummary string, agentMode bool, workDir string) string {
	var sb strings.Builder
	if agentMode {
		sb.WriteString("You are a helpful SQL assistant connected to a live Snowflake database.\n")
		sb.WriteString("You have tools to explore the database, run SQL, and access the local file system. Follow these rules:\n")
		sb.WriteString("1. Never guess database names, schema names, table names, or column names. Always look them up first.\n")
		sb.WriteString("2. When you need to know where you are, call get_session_context first.\n")
		sb.WriteString("3. When you need tables in a schema, call list_tables. If you don't know the schema, call list_schemas first.\n")
		sb.WriteString("4. Before writing any SELECT, call describe_table to confirm the real column names and types.\n")
		sb.WriteString("5. Only call run_sql once you have verified the table and column names through the other tools.\n")
		sb.WriteString("6. You can access the local file system with list_directory, read_file, and run_command. Use \".\" as the path to list the project root.\n")
		if workDir != "" {
			sb.WriteString("Project working directory: ")
			sb.WriteString(workDir)
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("You are a helpful SQL assistant. You can see the user's current SQL query and their most recent query result for context, but you cannot access the database directly. Help the user understand their data, improve their queries, and answer questions about SQL and Snowflake.\n")
		if workDir != "" {
			sb.WriteString("Project working directory: ")
			sb.WriteString(workDir)
			sb.WriteString("\n")
		}
	}
	if currentSQL != "" {
		sb.WriteString("\nCurrent SQL in the editor:\n```sql\n")
		sb.WriteString(currentSQL)
		sb.WriteString("\n```\n")
	}
	if lastResultSummary != "" {
		sb.WriteString("\nMost recent query result:\n")
		sb.WriteString(lastResultSummary)
		sb.WriteString("\n")
	}
	return sb.String()
}

// ── OpenAI chat with tool-calling ─────────────────────────────────────────────

// openAIChat handles a single chat turn using the OpenAI Chat Completions API.
// In agent mode it runs a tool-calling loop (up to 8 iterations); otherwise
// it performs a single round-trip and returns the assistant response.
func openAIChat(ctx context.Context, apiKey, model string, history []UIMessage, userText, currentSQL, lastResultSummary string, agentMode bool, workDir string, exec ToolExecutor) (UIMessage, error) {
	systemPrompt := buildSystemPrompt(currentSQL, lastResultSummary, agentMode, workDir)

	// Build initial messages
	messages := []map[string]any{
		{"role": "system", "content": systemPrompt},
	}
	for _, m := range history {
		messages = append(messages, map[string]any{"role": m.Role, "content": m.Text})
	}
	messages = append(messages, map[string]any{"role": "user", "content": userText})

	// Chat mode: single round-trip, no tools.
	if !agentMode {
		body, err := json.Marshal(map[string]any{
			"model":    model,
			"messages": messages,
		})
		if err != nil {
			return UIMessage{}, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			return UIMessage{}, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := chatHttpClient.Do(req)
		if err != nil {
			return UIMessage{}, err
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		if err != nil {
			return UIMessage{}, err
		}
		if resp.StatusCode != http.StatusOK {
			return UIMessage{}, fmt.Errorf("openai: status %d: %s", resp.StatusCode, raw)
		}
		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return UIMessage{}, err
		}
		if len(result.Choices) == 0 {
			return UIMessage{}, fmt.Errorf("openai: no choices returned")
		}
		return UIMessage{Role: "assistant", Text: strings.TrimSpace(result.Choices[0].Message.Content)}, nil
	}

	// Agent mode: tool-calling loop.

	// Convert tools to OpenAI format
	openAITools := make([]map[string]any, len(chatTools))
	for i, t := range chatTools {
		openAITools[i] = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t["name"],
				"description": t["description"],
				"parameters":  t["parameters"],
			},
		}
	}

	var accumulated []UIToolCall
	const maxIter = 8

	for iter := 0; iter < maxIter; iter++ {
		body, err := json.Marshal(map[string]any{
			"model":    model,
			"messages": messages,
			"tools":    openAITools,
		})
		if err != nil {
			return UIMessage{}, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			return UIMessage{}, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := chatHttpClient.Do(req)
		if err != nil {
			return UIMessage{}, err
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		if err != nil {
			return UIMessage{}, err
		}
		if resp.StatusCode != http.StatusOK {
			return UIMessage{}, fmt.Errorf("openai: status %d: %s", resp.StatusCode, raw)
		}

		var result struct {
			Choices []struct {
				FinishReason string `json:"finish_reason"`
				Message      struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return UIMessage{}, err
		}
		if len(result.Choices) == 0 {
			return UIMessage{}, fmt.Errorf("openai: no choices returned")
		}

		choice := result.Choices[0]

		if choice.FinishReason != "tool_calls" {
			return UIMessage{
				Role:      "assistant",
				Text:      strings.TrimSpace(choice.Message.Content),
				ToolCalls: accumulated,
			}, nil
		}

		// Append assistant message with tool_calls
		assistantMsg := map[string]any{
			"role":    "assistant",
			"content": choice.Message.Content,
		}
		toolCallsForMsg := make([]map[string]any, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			toolCallsForMsg[i] = map[string]any{
				"id":   tc.ID,
				"type": "function",
				"function": map[string]any{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			}
		}
		assistantMsg["tool_calls"] = toolCallsForMsg
		messages = append(messages, assistantMsg)

		// Execute each tool and append results
		for _, tc := range choice.Message.ToolCalls {
			output, isErr := exec(tc.Function.Name, tc.Function.Arguments)
			accumulated = append(accumulated, UIToolCall{
				Name:    tc.Function.Name,
				Input:   tc.Function.Arguments,
				Output:  output,
				IsError: isErr,
			})
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": tc.ID,
				"content":      output,
			})
		}
	}

	return UIMessage{}, fmt.Errorf("openai: exceeded max tool-calling iterations")
}

// ── Google Gemini chat with function-calling ───────────────────────────────────

// googleChat handles a single chat turn using the Google Gemini generateContent
// API. In agent mode it runs a function-calling loop (up to 8 iterations);
// otherwise it performs a single round-trip and returns the model response.
func googleChat(ctx context.Context, apiKey, model string, history []UIMessage, userText, currentSQL, lastResultSummary string, agentMode bool, workDir string, exec ToolExecutor) (UIMessage, error) {
	systemPrompt := buildSystemPrompt(currentSQL, lastResultSummary, agentMode, workDir)

	// Build contents from history.
	contents := []map[string]any{}
	for _, m := range history {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]any{{"text": m.Text}},
		})
	}
	contents = append(contents, map[string]any{
		"role":  "user",
		"parts": []map[string]any{{"text": userText}},
	})

	apiURL := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, apiKey,
	)

	// Chat mode: single round-trip, no tools.
	if !agentMode {
		body, err := json.Marshal(map[string]any{
			"system_instruction": map[string]any{
				"parts": []map[string]any{{"text": systemPrompt}},
			},
			"contents": contents,
		})
		if err != nil {
			return UIMessage{}, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
		if err != nil {
			return UIMessage{}, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := chatHttpClient.Do(req)
		if err != nil {
			return UIMessage{}, err
		}
		rawResp, err := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		if err != nil {
			return UIMessage{}, err
		}
		if resp.StatusCode != http.StatusOK {
			return UIMessage{}, fmt.Errorf("google: status %d: %s", resp.StatusCode, rawResp)
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
		if err := json.Unmarshal(rawResp, &result); err != nil {
			return UIMessage{}, err
		}
		if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
			return UIMessage{}, fmt.Errorf("google: no candidates returned")
		}
		var parts []string
		for _, p := range result.Candidates[0].Content.Parts {
			if p.Text != "" {
				parts = append(parts, p.Text)
			}
		}
		return UIMessage{Role: "assistant", Text: strings.TrimSpace(strings.Join(parts, ""))}, nil
	}

	// Agent mode: tool-calling loop.

	// Build Google function declarations (omit parameters for no-arg tools).
	functionDecls := make([]map[string]any, 0, len(chatTools))
	for _, t := range chatTools {
		decl := map[string]any{
			"name":        t["name"],
			"description": t["description"],
		}
		if params, ok := t["parameters"]; ok {
			decl["parameters"] = params
		}
		functionDecls = append(functionDecls, decl)
	}

	var accumulated []UIToolCall
	const maxIter = 8

	// partEnvelope is used only to inspect a part — never to reconstruct it.
	type partEnvelope struct {
		Text         string `json:"text"`
		FunctionCall *struct {
			Name string         `json:"name"`
			Args map[string]any `json:"args"`
		} `json:"functionCall"`
	}

	for iter := 0; iter < maxIter; iter++ {
		body, err := json.Marshal(map[string]any{
			"system_instruction": map[string]any{
				"parts": []map[string]any{{"text": systemPrompt}},
			},
			"contents": contents,
			"tools": []map[string]any{
				{"function_declarations": functionDecls},
			},
			"tool_config": map[string]any{
				"function_calling_config": map[string]any{"mode": "AUTO"},
			},
		})
		if err != nil {
			return UIMessage{}, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
		if err != nil {
			return UIMessage{}, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := chatHttpClient.Do(req)
		if err != nil {
			return UIMessage{}, err
		}
		rawResp, err := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		if err != nil {
			return UIMessage{}, err
		}
		if resp.StatusCode != http.StatusOK {
			return UIMessage{}, fmt.Errorf("google: status %d: %s", resp.StatusCode, rawResp)
		}

		// Parse the candidate content with raw parts so we never lose fields
		// like thought_signature that thinking models attach to functionCall parts.
		var result struct {
			Candidates []struct {
				Content struct {
					Parts []json.RawMessage `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(rawResp, &result); err != nil {
			return UIMessage{}, err
		}
		if len(result.Candidates) == 0 {
			return UIMessage{}, fmt.Errorf("google: no candidates returned")
		}

		rawParts := result.Candidates[0].Content.Parts

		// Inspect each part to find function calls, but keep the raw bytes.
		var functionCalls []partEnvelope
		var textParts []string
		hasFunctionCall := false
		for _, rp := range rawParts {
			var p partEnvelope
			if err := json.Unmarshal(rp, &p); err != nil {
				continue
			}
			if p.FunctionCall != nil {
				hasFunctionCall = true
				functionCalls = append(functionCalls, p)
			} else if p.Text != "" {
				textParts = append(textParts, p.Text)
			}
		}

		if !hasFunctionCall {
			return UIMessage{
				Role:      "assistant",
				Text:      strings.TrimSpace(strings.Join(textParts, "")),
				ToolCalls: accumulated,
			}, nil
		}

		// Echo the model turn back verbatim — raw parts preserve thought_signature
		// and any other fields that thinking models include.
		contents = append(contents, map[string]any{
			"role":  "model",
			"parts": rawParts,
		})

		// Execute each tool and build the functionResponse turn.
		var responseParts []map[string]any
		for _, fc := range functionCalls {
			argsJSON, _ := json.Marshal(fc.FunctionCall.Args)
			output, isErr := exec(fc.FunctionCall.Name, string(argsJSON))
			accumulated = append(accumulated, UIToolCall{
				Name:    fc.FunctionCall.Name,
				Input:   string(argsJSON),
				Output:  output,
				IsError: isErr,
			})
			responseParts = append(responseParts, map[string]any{
				"functionResponse": map[string]any{
					"name": fc.FunctionCall.Name,
					"response": map[string]any{
						"output": output,
					},
				},
			})
		}
		contents = append(contents, map[string]any{
			"role":  "user",
			"parts": responseParts,
		})
	}

	return UIMessage{}, fmt.Errorf("google: exceeded max tool-calling iterations")
}

// GetSuggestion requests an inline SQL completion from the configured provider.
// provider must be "openai", "google", or "ollama". Returns the trimmed completion text.
// ollamaPort is the port number for the local Ollama instance (0 = default 11434);
// ollamaNumCtx is the context window size sent to Ollama (0 = let Ollama decide);
// both are ignored for non-Ollama providers.
func GetSuggestion(provider, apiKey, model, prompt string, ollamaPort, ollamaNumCtx int) (string, error) {
	switch provider {
	case "openai":
		return openAISuggestion(apiKey, model, prompt)
	case "google":
		return googleSuggestion(apiKey, model, prompt)
	case "ollama":
		return ollamaSuggestion(ollamaBaseURL(ollamaPort), model, prompt, ollamaNumCtx)
	default:
		return "", fmt.Errorf("unknown AI provider: %s", provider)
	}
}

// SuggestFormatOptions analyses the provided file sample and returns a clean
// JSON string containing suggested Snowflake COPY INTO format options.
// format should be "CSV" or "JSON". Code fences are stripped and the result
// is validated before being returned; an error is returned if the AI response
// cannot be parsed as JSON.
// ollamaPort is the port number for the local Ollama instance (0 = default 11434);
// ollamaNumCtx is the context window size sent to Ollama (0 = let Ollama decide);
// both are ignored for non-Ollama providers.
func SuggestFormatOptions(provider, apiKey, model, format, sampleContent string, ollamaPort, ollamaNumCtx int) (string, error) {
	// Run the local CSV sniffer to give the AI reliable anchors and reduce
	// hallucination.  We pass only a few lines so the sniffer is fast; the
	// full sample is still embedded in the prompt for the AI to verify.
	var hints []string
	if format == "CSV" {
		if det := inspectCSVContent(sampleContent, 20); det != nil {
			hints = append(hints, fmt.Sprintf("  fieldDelimiter: %q", det.FieldDelimiter))
			if det.SkipHeader > 0 {
				hints = append(hints, "  parseHeader: true")
			} else {
				hints = append(hints, "  parseHeader: false")
			}
			if det.FieldOptionallyEnclosed != "" {
				hints = append(hints, fmt.Sprintf("  fieldOptionallyEnclosedBy: %q", det.FieldOptionallyEnclosed))
			}
		}
	}

	prompt := buildFormatSuggestionPrompt(format, sampleContent, hints)
	var raw string
	var err error
	switch provider {
	case "openai":
		raw, err = openAISuggestFormat(apiKey, model, prompt)
	case "google":
		raw, err = googleSuggestFormat(apiKey, model, prompt)
	case "ollama":
		raw, err = ollamaSuggestFormat(ollamaBaseURL(ollamaPort), model, prompt, ollamaNumCtx)
	default:
		return "", fmt.Errorf("unknown AI provider: %s", provider)
	}
	if err != nil {
		return "", err
	}
	return extractFormatJSON(raw)
}

// extractFormatJSON strips markdown code fences from an AI response, extracts
// the first JSON object, and validates it.
func extractFormatJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)

	// Strip opening code fence line (``` or ```json or ```JSON etc.)
	if strings.HasPrefix(raw, "```") {
		if nl := strings.Index(raw, "\n"); nl != -1 {
			raw = raw[nl+1:]
		} else {
			raw = raw[3:]
		}
		// Strip closing fence
		if idx := strings.LastIndex(raw, "```"); idx != -1 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}

	// Find the outermost JSON object
	start := strings.Index(raw, "{")
	if start == -1 {
		return "", fmt.Errorf("AI returned no JSON object: %.120s", raw)
	}
	end := strings.LastIndex(raw, "}")
	if end > start {
		raw = raw[start : end+1]
	} else {
		// No closing brace — response was likely truncated; take from '{' to end.
		raw = raw[start:]
	}

	// Defensively remove the "explanation" key: the AI sometimes includes it
	// with unescaped double quotes in the value, which invalidates the JSON.
	raw = stripJSONStringKey(raw, "explanation")

	if !json.Valid([]byte(raw)) {
		// Attempt to recover from a truncated response (e.g. Gemini cuts off
		// before the closing '}' when the response hits the token limit).
		if rec := recoverTruncatedJSON(raw); rec != "" {
			raw = rec
		}
	}

	if !json.Valid([]byte(raw)) {
		return "", fmt.Errorf("AI returned invalid JSON: %.120s", raw)
	}
	return raw, nil
}

// recoverTruncatedJSON tries to salvage a JSON object whose closing '}' was
// cut off by a token limit. It attempts two strategies:
//  1. Append '}' directly — works when only the brace is missing.
//  2. Trim back to the last top-level ',' — discards the incomplete final
//     key-value pair — then append '}'. Works when truncation happened
//     mid-key or mid-value.
func recoverTruncatedJSON(s string) string {
	trimmed := strings.TrimRight(s, " \t\r\n")

	// Strategy 1: just close the object.
	if candidate := trimmed + "}"; json.Valid([]byte(candidate)) {
		return candidate
	}

	// Strategy 2: drop the incomplete trailing field and close.
	// Using strings.LastIndex is safe for our constrained, flat JSON output:
	// the last ',' in the string is overwhelmingly the separator between the
	// last two complete key-value pairs (not a ',' inside a field value).
	if i := strings.LastIndex(trimmed, ","); i != -1 {
		candidate := strings.TrimRight(trimmed[:i], " \t\r\n") + "\n}"
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}
	return ""
}

// stripJSONStringKey removes a string-valued key-value pair from a raw JSON
// object. It handles values that contain unescaped double quotes by treating
// the first `"` followed (after optional whitespace) by `,` or `}` as the
// end of the value — a heuristic that is safe for our constrained output.
func stripJSONStringKey(s, key string) string {
	keyPos := strings.Index(s, `"`+key+`"`)
	if keyPos == -1 {
		return s
	}

	// Advance past the key to the colon.
	p := keyPos + len(key) + 2 // +2 for the surrounding quotes
	for p < len(s) && isJSONWS(s[p]) {
		p++
	}
	if p >= len(s) || s[p] != ':' {
		return s
	}
	p++

	// Advance to the opening quote of the value.
	for p < len(s) && isJSONWS(s[p]) {
		p++
	}
	if p >= len(s) || s[p] != '"' {
		return s // not a string value; bail safely
	}
	p++ // skip opening quote

	// Scan for the closing quote using the heuristic described above.
	for p < len(s) {
		if s[p] == '"' {
			rest := p + 1
			for rest < len(s) && isJSONWS(s[rest]) {
				rest++
			}
			if rest >= len(s) || s[rest] == ',' || s[rest] == '}' {
				return spliceJSONPair(s, keyPos, p+1)
			}
		}
		p++
	}
	return s
}

// spliceJSONPair removes the characters [pairStart, valueEnd) from s and
// eliminates exactly one surrounding comma to keep the JSON valid.
func spliceJSONPair(s string, pairStart, valueEnd int) string {
	// Prefer removing the leading comma.
	check := pairStart - 1
	for check >= 0 && isJSONWS(s[check]) {
		check--
	}
	if check >= 0 && s[check] == ',' {
		return strings.TrimRight(s[:check], " \t\r\n") + s[valueEnd:]
	}
	// No leading comma: try trailing.
	tail := valueEnd
	for tail < len(s) && isJSONWS(s[tail]) {
		tail++
	}
	if tail < len(s) && s[tail] == ',' {
		return s[:pairStart] + s[tail+1:]
	}
	// Only field: just remove it.
	return s[:pairStart] + s[valueEnd:]
}

func isJSONWS(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

func buildFormatSuggestionPrompt(format, sample string, hints []string) string {
	// Keep only ~600 bytes, trimmed to a whole line, to limit token usage.
	const maxSample = 600
	if len(sample) > maxSample {
		trimmed := sample[:maxSample]
		if nl := strings.LastIndex(trimmed, "\n"); nl > 0 {
			trimmed = trimmed[:nl]
		}
		sample = trimmed + "\n...(truncated)"
	}

	var fieldHint string
	if format == "CSV" {
		fieldHint = `- fieldDelimiter: string (e.g. "," or "\\t" or "|")
- parseHeader: boolean (true if first row contains column names)
- fieldOptionallyEnclosedBy: string (e.g. "\"" or "NONE")
- encoding: string (e.g. "UTF8", "ISO8859_1" — omit if clearly UTF-8)
- compression: string (e.g. "AUTO", "GZIP", "NONE")
- recordDelimiter: string (e.g. "\\n", "\\r\\n" — omit if standard \n)
`
	} else {
		fieldHint = `- multiLine: boolean (true if each record spans multiple lines)
- stripOuterArray: boolean (true if root element is an array of objects)
- compression: string (e.g. "AUTO", "GZIP", "NONE")
`
	}

	var hintSection string
	if len(hints) > 0 {
		hintSection = "\nLocal analysis detected these probable values (verify against the sample below):\n" +
			strings.Join(hints, "\n") + "\n"
	}

	return fmt.Sprintf(`Analyze this %s file sample and return Snowflake COPY INTO format option suggestions.
%s
File sample:
---
%s
---

Return ONLY a compact JSON object with no explanation or commentary. Include only fields you are confident about:
%s
Output the JSON object only. No prose, no markdown fences, no extra keys.`, format, hintSection, sample, fieldHint)
}

func openAISuggestFormat(apiKey, model, prompt string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 600,
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

	resp, err := suggestHttpClient.Do(req)
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
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func googleSuggestFormat(apiKey, model, prompt string) (string, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, apiKey,
	)

	body, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": prompt}}},
		},
		"generationConfig": map[string]any{
			"maxOutputTokens": 2000,
			"temperature":     0.1,
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

	resp, err := suggestHttpClient.Do(req)
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
	return strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text), nil
}

// ── OpenAI ────────────────────────────────────────────────────────────────────

// openAISuggestion requests an inline SQL completion from the OpenAI Chat
// Completions API. It returns the trimmed response text or an error.
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
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
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

// ollamaSuggestion requests an inline SQL completion from the local Ollama /api/generate endpoint.
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
	resp, err := suggestHttpClient.Do(req)
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
	return strings.TrimSpace(result.Response), nil
}

// ollamaSuggestFormat requests format option suggestions from the local Ollama /api/generate endpoint.
// numCtx sets the context window size; 0 means use Ollama's default.
func ollamaSuggestFormat(base, model, prompt string, numCtx int) (string, error) {
	payload := map[string]any{
		"model":       model,
		"prompt":      prompt,
		"stream":      false,
		"num_predict": 600,
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
	resp, err := suggestHttpClient.Do(req)
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
	return strings.TrimSpace(result.Response), nil
}

// ollamaChat handles a single chat turn using the local Ollama /api/chat endpoint.
// In agent mode it runs a tool-calling loop (up to 8 iterations) using Ollama's
// OpenAI-compatible tool format; otherwise it performs a single round-trip.
// numCtx sets the context window size; 0 means use Ollama's default.
func ollamaChat(ctx context.Context, base, model string, numCtx int, history []UIMessage, userText, currentSQL, lastResultSummary string, agentMode bool, workDir string, exec ToolExecutor) (UIMessage, error) {
	systemPrompt := buildSystemPrompt(currentSQL, lastResultSummary, agentMode, workDir)

	messages := []map[string]any{
		{"role": "system", "content": systemPrompt},
	}
	for _, m := range history {
		messages = append(messages, map[string]any{"role": m.Role, "content": m.Text})
	}
	messages = append(messages, map[string]any{"role": "user", "content": userText})

	// Chat mode: single round-trip, no tools.
	if !agentMode {
		chatPayload := map[string]any{
			"model":    model,
			"messages": messages,
			"stream":   false,
		}
		if numCtx > 0 {
			chatPayload["options"] = map[string]any{"num_ctx": numCtx}
		}
		body, err := json.Marshal(chatPayload)
		if err != nil {
			return UIMessage{}, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/chat", bytes.NewReader(body))
		if err != nil {
			return UIMessage{}, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := chatHttpClient.Do(req)
		if err != nil {
			return UIMessage{}, fmt.Errorf("ollama not reachable: %w", err)
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		if err != nil {
			return UIMessage{}, err
		}
		if resp.StatusCode != http.StatusOK {
			return UIMessage{}, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, raw)
		}
		var result struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return UIMessage{}, err
		}
		return UIMessage{Role: "assistant", Text: strings.TrimSpace(result.Message.Content)}, nil
	}

	// Agent mode: tool-calling loop.
	ollamaTools := make([]map[string]any, len(chatTools))
	for i, t := range chatTools {
		ollamaTools[i] = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t["name"],
				"description": t["description"],
				"parameters":  t["parameters"],
			},
		}
	}

	var accumulated []UIToolCall
	const maxIter = 8

	for iter := 0; iter < maxIter; iter++ {
		agentPayload := map[string]any{
			"model":    model,
			"messages": messages,
			"tools":    ollamaTools,
			"stream":   false,
		}
		if numCtx > 0 {
			agentPayload["options"] = map[string]any{"num_ctx": numCtx}
		}
		body, err := json.Marshal(agentPayload)
		if err != nil {
			return UIMessage{}, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/chat", bytes.NewReader(body))
		if err != nil {
			return UIMessage{}, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := chatHttpClient.Do(req)
		if err != nil {
			return UIMessage{}, fmt.Errorf("ollama not reachable: %w", err)
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		if err != nil {
			return UIMessage{}, err
		}
		if resp.StatusCode != http.StatusOK {
			return UIMessage{}, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, raw)
		}

		var result struct {
			Message struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string         `json:"name"`
						Arguments map[string]any `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return UIMessage{}, err
		}

		if len(result.Message.ToolCalls) == 0 {
			return UIMessage{
				Role:      "assistant",
				Text:      strings.TrimSpace(result.Message.Content),
				ToolCalls: accumulated,
			}, nil
		}

		// Echo the assistant message back with its tool_calls.
		toolCallMaps := make([]map[string]any, len(result.Message.ToolCalls))
		for i, tc := range result.Message.ToolCalls {
			toolCallMaps[i] = map[string]any{
				"function": map[string]any{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			}
		}
		messages = append(messages, map[string]any{
			"role":       result.Message.Role,
			"content":    result.Message.Content,
			"tool_calls": toolCallMaps,
		})

		// Execute each tool and append results.
		for _, tc := range result.Message.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Function.Arguments)
			output, isErr := exec(tc.Function.Name, string(argsJSON))
			accumulated = append(accumulated, UIToolCall{
				Name:    tc.Function.Name,
				Input:   string(argsJSON),
				Output:  output,
				IsError: isErr,
			})
			messages = append(messages, map[string]any{
				"role":    "tool",
				"content": output,
			})
		}
	}

	return UIMessage{}, fmt.Errorf("ollama: exceeded max tool-calling iterations")
}

// ── Google AI Studios (Gemini) ────────────────────────────────────────────────

// googleSuggestion requests an inline SQL completion from the Google Gemini
// generateContent API. It returns the trimmed response text or an error.
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
	return strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text), nil
}
