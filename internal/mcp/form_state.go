// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/logger"
)

// OpenFormState is a snapshot of the create dialog the user has open, pushed
// from the frontend (useMcpFormSync) via IPC. Field names are the Ant Design
// form field names, which equal the json tags of the matching backend params
// struct (e.g. integrations.StorageIntegrationParams).
type OpenFormState struct {
	Form     string         `json:"form"`               // dialog id, e.g. "create-integration"
	Kind     string         `json:"kind,omitempty"`     // e.g. "STORAGE"
	Provider string         `json:"provider,omitempty"` // e.g. "S3"
	Fields   map[string]any `json:"fields"`
}

// OpenFormStateStore is a concurrency-safe cache of the currently open form.
// Only one form is tracked at a time — the most recently pushed one.
type OpenFormStateStore struct {
	mu    sync.RWMutex
	state *OpenFormState
}

// NewOpenFormStateStore returns an empty store (no form open).
func NewOpenFormStateStore() *OpenFormStateStore { return &OpenFormStateStore{} }

// Set replaces the current form state.
func (s *OpenFormStateStore) Set(state *OpenFormState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

// Clear marks no form as open.
func (s *OpenFormStateStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = nil
}

// Get returns a copy of the current state (Fields map copied), or nil.
func (s *OpenFormStateStore) Get() *OpenFormState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state == nil {
		return nil
	}
	out := *s.state
	out.Fields = maps.Clone(s.state.Fields)
	return &out
}

// merge writes fields into the open form's state in place. Returns false when
// no form is open.
func (s *OpenFormStateStore) merge(fields map[string]any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		return false
	}
	if s.state.Fields == nil {
		s.state.Fields = map[string]any{}
	}
	maps.Copy(s.state.Fields, fields)
	return true
}

// isSensitiveField reports whether a field's value must be masked before it is
// shown to an MCP client.
func isSensitiveField(name string) bool {
	n := strings.ToLower(name)
	for _, k := range []string{"secret", "password", "apikey", "token"} {
		if strings.Contains(n, k) {
			return true
		}
	}
	return false
}

// maskValue hides a sensitive field's non-empty value; empty values pass
// through so the client can still tell the field is unset.
func maskValue(name string, v any) any {
	if !isSensitiveField(name) || v == nil || v == "" {
		return v
	}
	return "********"
}

// fillOpenFormInput is the tool input for fill_open_form.
type fillOpenFormInput struct {
	Fields map[string]any `json:"fields" jsonschema:"form field name to value; names are the json tags of the matching build_*_sql params (e.g. awsRoleArn, allowedLocations). Booleans for switches/checkboxes, arrays of strings for multi-selects"`
}

// FillOpenFormPayload is the Wails event payload for "mcp:fill-form".
type FillOpenFormPayload struct {
	Form   string         `json:"form"`
	Fields map[string]any `json:"fields"`
}

// registerOpenFormTools wires get_open_form / fill_open_form onto srv. They
// execute no SQL, so they are registered in every execution mode; both are
// emit-gated (no Wails runtime → no dialog to sync with).
func registerOpenFormTools(srv *mcpsdk.Server, emit func(string, any), store *OpenFormStateStore) {
	if emit == nil || store == nil {
		return
	}

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_open_form",
		Description: "Get the create dialog the user currently has open in Thaw (e.g. Create Storage Integration): " +
			"its form id, kind, provider and current field values. Secret-like values (secret/password/apiKey/token) are masked. " +
			"Returns a message if no form is open. Use with fill_open_form to fill the dialog instead of asking the user to copy values.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
		state := store.Get()
		if state == nil {
			return textResult("No form is currently open."), nil, nil
		}
		for k, v := range state.Fields {
			state.Fields[k] = maskValue(k, v)
		}
		return jsonResult(state), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "fill_open_form",
		Description: "Fill fields of the create dialog the user has open (see get_open_form). " +
			"Field names equal the json params of the matching build_*_sql tool (build_storage_integration_sql, " +
			"build_api_integration_sql, …) — call that tool first with the same values to self-check the generated SQL. " +
			"Setting provider/source/subtype/secType switches the dialog's sub-form. Nothing is executed: the user reviews and clicks Create. " +
			"Returns the fields that changed.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in fillOpenFormInput) (*mcpsdk.CallToolResult, any, error) {
		if len(in.Fields) == 0 {
			return nil, nil, fmt.Errorf("fields must contain at least one field")
		}
		before := store.Get()
		if before == nil {
			return textResult("No form is currently open. Ask the user to open the create dialog first."), nil, nil
		}

		var emitFailed bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.L.Error("mcp fill_open_form: emit panicked", "err", r)
					emitFailed = true
				}
			}()
			emit("mcp:fill-form", FillOpenFormPayload{Form: before.Form, Fields: in.Fields})
		}()
		if emitFailed {
			return textResult("Failed to fill form: internal error"), nil, nil
		}
		// Keep the cache consistent inside the frontend's 300 ms debounce window;
		// the frontend's next push overwrites it with the authoritative values.
		store.merge(in.Fields)

		var b strings.Builder
		for _, k := range slices.Sorted(maps.Keys(in.Fields)) {
			old, had := before.Fields[k]
			if had && reflect.DeepEqual(old, in.Fields[k]) {
				continue
			}
			fmt.Fprintf(&b, "\n- %s: %v → %v", k, maskValue(k, old), maskValue(k, in.Fields[k]))
		}
		if b.Len() == 0 {
			return textResult(fmt.Sprintf("No changes: all %d field(s) already had those values in %q.", len(in.Fields), before.Form)), nil, nil
		}
		return textResult(fmt.Sprintf("Filled %q. The user reviews and clicks Create. Changed:%s", before.Form, b.String())), nil, nil
	})
}
