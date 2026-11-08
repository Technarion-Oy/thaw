// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func callText(t *testing.T, cs *mcpsdk.ClientSession, name string, args any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res.Content[0].(*mcpsdk.TextContent).Text
}

func TestOpenFormTools(t *testing.T) {
	store := NewOpenFormStateStore()
	var emitted []FillOpenFormPayload
	emit := func(name string, p any) {
		if name == "mcp:fill-form" {
			emitted = append(emitted, p.(FillOpenFormPayload))
		}
	}
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "t", Version: "v1"}, nil)
	registerOpenFormTools(srv, emit, store)
	httpSrv := httptest.NewServer(mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil))
	t.Cleanup(httpSrv.Close)
	cs, err := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "c", Version: "v1"}, nil).
		Connect(context.Background(), &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	if got := callText(t, cs, "get_open_form", map[string]any{}); !strings.Contains(got, "No form") {
		t.Errorf("closed get = %q", got)
	}
	if got := callText(t, cs, "fill_open_form", map[string]any{"fields": map[string]any{"name": "X"}}); !strings.Contains(got, "No form") || len(emitted) != 0 {
		t.Errorf("closed fill = %q, emitted %d", got, len(emitted))
	}

	store.Set(&OpenFormState{Form: "create-integration", Kind: "API", Fields: map[string]any{
		"name": "OLD", "apiKey": "hunter2", "oauthClientSecret": "", "enabled": true,
	}})
	got := callText(t, cs, "get_open_form", map[string]any{})
	if strings.Contains(got, "hunter2") || !strings.Contains(got, "OLD") {
		t.Errorf("get should mask apiKey and show name: %s", got)
	}

	got = callText(t, cs, "fill_open_form", map[string]any{"fields": map[string]any{
		"name": "NEW", "enabled": true, "apiKey": "s3cret",
	}})
	if len(emitted) != 1 || emitted[0].Form != "create-integration" || emitted[0].Fields["name"] != "NEW" {
		t.Fatalf("emitted = %+v", emitted)
	}
	if !strings.Contains(got, "name: OLD → NEW") || strings.Contains(got, "enabled") || strings.Contains(got, "s3cret") || strings.Contains(got, "hunter2") {
		t.Errorf("delta = %q", got)
	}
	if s := store.Get(); s.Fields["name"] != "NEW" || s.Fields["apiKey"] != "s3cret" {
		t.Errorf("store not merged: %+v", s.Fields)
	}

	store.Clear()
	if store.Get() != nil {
		t.Error("Clear should empty the store")
	}
}
