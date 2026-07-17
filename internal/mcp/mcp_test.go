// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/snowflake"
)

// expectedTools is the full tool set the server exposes in metadata mode
// (alphabetically sorted). Editor context tools (get_current_editor_sql,
// get_query_history) are NOT listed because the test uses a nil store.
// Workspace tools (git_*, list_directory, read_file, search_files) are NOT
// listed because the test uses an empty SessionConfig (no WorkspaceRoot).
var expectedTools = []string{
	"analyze_migration",
	"build_alter_stage_sql",
	"build_api_integration_sql",
	"build_call_statement",
	"build_catalog_integration_sql",
	"build_create_file_format_sql",
	"build_create_pipe_sql",
	"build_create_secret_sql",
	"build_create_stage_sql",
	"build_external_access_integration_sql",
	"build_function_select",
	"build_notification_integration_sql",
	"build_refresh_pipe_sql",
	"build_security_integration_sql",
	"build_storage_integration_sql",
	"describe_table",
	"explain_query",
	"format_sql",
	"generate_migration_script",
	"get_all_data_types",
	"get_data_retention",
	"get_database_cross_deps",
	"get_database_ddl",
	"get_ddl",
	"get_er_model",
	"get_explain_diagnostics",
	"get_function_info",
	"get_function_tooltip",
	"get_object_lineage",
	"get_pipe_copy_history",
	"get_pipe_status",
	"get_procedure_params",
	"get_role_ddl",
	"get_schema_cross_deps",
	"get_schema_foreign_keys",
	"get_session_context",
	"get_snowflake_keywords",
	"get_table_foreign_keys",
	"get_task_dependencies",
	"get_task_run_history",
	"get_warehouse_ddl",
	"list_available_roles",
	"list_databases",
	"list_dropped_databases",
	"list_dropped_schemas",
	"list_dropped_tables",
	"list_file_formats",
	"list_integrations",
	"list_objects",
	"list_roles",
	"list_schemas",
	"list_secrets",
	"list_stage_files",
	"list_tasks",
	"list_warehouses",
	"search_functions",
	"search_objects",
	"suggest_join_conditions",
	"validate_data_type",
	"validate_sql",
}

// TestServerExposesToolsOverSSE verifies that an external MCP client can
// connect to the server over the SSE transport and discover the proof-of-life
// tools. A nil client is sufficient because tool *listing* does not invoke the
// handlers (which would call into Snowflake).
func TestServerExposesToolsOverSSE(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)

	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := client.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("client connect over SSE failed: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	var got []string
	for _, tool := range res.Tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got)

	if len(got) != len(expectedTools) {
		t.Fatalf("tool count = %d, want %d (%v)", len(got), len(expectedTools), got)
	}
	for i, name := range expectedTools {
		if got[i] != name {
			t.Errorf("tool[%d] = %q, want %q", i, got[i], name)
		}
	}
}

// TestManagerPortAllocation verifies auto-assignment starts at basePort and
// that an explicit port already claimed by a session is rejected.
func TestManagerPortAllocation(t *testing.T) {
	m := NewManager(nil)

	ln, err := m.allocatePortLocked(0)
	if err != nil {
		t.Fatalf("allocatePortLocked(0) failed: %v", err)
	}
	defer func() { _ = ln.Close() }()

	port := ln.Addr().(*net.TCPAddr).Port
	if port < basePort {
		t.Errorf("auto-assigned port %d below basePort %d", port, basePort)
	}

	// Register a session occupying that port and assert an explicit request
	// for the same port is rejected (the inUse path in allocatePortLocked).
	m.sessions["occupied"] = &session{label: "occupied", port: port}
	if _, err := m.allocatePortLocked(port); err == nil {
		t.Errorf("expected duplicate port %d to be rejected, got nil error", port)
	}
}

// TestManagerPreferredToken verifies that passing a non-empty preferredToken to
// Start causes that token to be used (retrievable via SessionToken) rather than
// generating a new random one, and that an empty preferredToken still generates
// a random token.
func TestManagerPreferredToken(t *testing.T) {
	m := NewManager(nil)

	// Start a session with a preferred token. We use a nil client since the
	// test operates in metadata mode (no Snowflake calls needed for start).
	const wantToken = "my-persisted-token-abc123"
	info, err := m.Start(context.Background(), "preferred", "acct/user", ExecutionModeMetadata, 0, nil, SessionConfig{}, wantToken)
	if err != nil {
		t.Fatalf("Start with preferredToken failed: %v", err)
	}
	defer func() { _ = m.Stop("preferred") }()

	got, ok := m.SessionToken("preferred")
	if !ok {
		t.Fatal("SessionToken returned false for running session")
	}
	if got != wantToken {
		t.Errorf("SessionToken = %q, want %q", got, wantToken)
	}

	// SessionEndpoint should also return the same token (alongside the
	// token-free URL).
	_, epToken, ok := m.SessionEndpoint("preferred")
	if !ok {
		t.Fatal("SessionEndpoint returned false for running session")
	}
	if epToken != wantToken {
		t.Errorf("SessionEndpoint token = %q, want %q", epToken, wantToken)
	}

	// Verify port was assigned.
	if info.Port == 0 {
		t.Error("expected auto-assigned port, got 0")
	}

	// Start another session without a preferred token — should get a random one.
	_, err = m.Start(context.Background(), "random", "acct/user", ExecutionModeMetadata, 0, nil, SessionConfig{}, "")
	if err != nil {
		t.Fatalf("Start without preferredToken failed: %v", err)
	}
	defer func() { _ = m.Stop("random") }()

	randomToken, ok := m.SessionToken("random")
	if !ok {
		t.Fatal("SessionToken returned false for 'random' session")
	}
	if randomToken == "" {
		t.Error("expected non-empty random token")
	}
	if randomToken == wantToken {
		t.Error("random token should differ from the preferred token")
	}
}

// TestSessionTokenMissing verifies SessionToken returns false for non-existent sessions.
func TestSessionTokenMissing(t *testing.T) {
	m := NewManager(nil)
	if _, ok := m.SessionToken("nonexistent"); ok {
		t.Error("expected false for non-existent session")
	}
}

// TestSessionEndpoint verifies SessionEndpoint returns the token-free SSE URL
// and the raw token as separate values (for header-based client configs), and
// reports false for unknown sessions.
func TestSessionEndpoint(t *testing.T) {
	m := NewManager(nil)

	const wantToken = "endpoint-token-xyz"
	info, err := m.Start(context.Background(), "ep", "acct/user", ExecutionModeMetadata, 0, nil, SessionConfig{}, wantToken)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = m.Stop("ep") }()

	endpoint, token, ok := m.SessionEndpoint("ep")
	if !ok {
		t.Fatal("SessionEndpoint returned false for running session")
	}
	if token != wantToken {
		t.Errorf("token = %q, want %q", token, wantToken)
	}
	// The endpoint URL must NOT carry the token — that is the whole point of
	// moving it into an Authorization header.
	if strings.Contains(endpoint, wantToken) || strings.Contains(endpoint, "token=") {
		t.Errorf("endpoint %q must be token-free", endpoint)
	}
	wantURL := fmt.Sprintf("http://127.0.0.1:%d/sse", info.Port)
	if endpoint != wantURL {
		t.Errorf("endpoint = %q, want %q", endpoint, wantURL)
	}

	if _, _, ok := m.SessionEndpoint("nonexistent"); ok {
		t.Error("expected false for non-existent session")
	}
}

// TestTokenGuardLogsRejections verifies rejected (unauthenticated) GETs still
// return 401 after the auth-failure logging was added, and that valid tokens
// continue to pass. It exercises the record path (rate-limited logging) without
// asserting on log output.
func TestTokenGuardLogsRejections(t *testing.T) {
	const token = "guard-log-token"
	guard := tokenGuard("logtest", token, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Several rejected attempts in quick succession (rate-limited logging).
	for range 5 {
		req := httptest.NewRequest(http.MethodGet, "http://localhost:9100/sse", nil)
		rec := httptest.NewRecorder()
		guard.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("rejected GET status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	}

	// A valid token still passes.
	req := httptest.NewRequest(http.MethodGet, "http://localhost:9100/sse?token="+token, nil)
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("valid GET status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestAuthFailureLoggerRateLimit verifies the logger emits the first failure
// immediately, suppresses failures inside the interval (accumulating a count),
// and emits again once the interval elapses with the accumulated count.
func TestAuthFailureLoggerRateLimit(t *testing.T) {
	l := &authFailureLogger{}

	// First failure logs immediately: suppressed resets to 0, last is set.
	l.record("x", "127.0.0.1:1")
	if l.suppressed != 0 {
		t.Fatalf("after first record, suppressed = %d, want 0", l.suppressed)
	}
	if l.last.IsZero() {
		t.Fatal("after first record, last must be set")
	}

	// Failures inside the interval are counted, not logged.
	l.record("x", "127.0.0.1:2")
	l.record("x", "127.0.0.1:3")
	if l.suppressed != 2 {
		t.Fatalf("after 2 suppressed records, suppressed = %d, want 2", l.suppressed)
	}

	// Simulate the interval elapsing: the next record logs and resets the count.
	l.last = l.last.Add(-2 * authFailureLogInterval)
	l.record("x", "127.0.0.1:4")
	if l.suppressed != 0 {
		t.Fatalf("after interval elapsed, suppressed = %d, want 0 (logged)", l.suppressed)
	}
}

// TestLoopbackGuard verifies the SSE handler rejects non-loopback Host headers
// and cross-origin browser requests while allowing loopback traffic.
func TestLoopbackGuard(t *testing.T) {
	guard := loopbackGuard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name   string
		host   string
		origin string
		want   int
	}{
		{"loopback host no origin", "localhost:9100", "", http.StatusOK},
		{"loopback ip", "127.0.0.1:9100", "", http.StatusOK},
		{"loopback origin", "localhost:9100", "http://localhost:9100", http.StatusOK},
		{"rebound host", "evil.example.com", "", http.StatusForbidden},
		{"cross origin", "localhost:9100", "https://evil.example.com", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+tc.host+"/sse", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			guard.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// TestTokenGuard verifies the per-session token is required on session-creating
// GET requests (via header or query param) but POSTs pass through unchecked
// (they are authorized by the SDK-issued sessionid).
func TestTokenGuard(t *testing.T) {
	const token = "s3cret-token"
	guard := tokenGuard("test", token, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name    string
		method  string
		query   string
		authHdr string
		want    int
	}{
		{"get no token", http.MethodGet, "", "", http.StatusUnauthorized},
		{"get wrong query token", http.MethodGet, "?token=nope", "", http.StatusUnauthorized},
		{"get correct query token", http.MethodGet, "?token=" + token, "", http.StatusOK},
		{"get correct bearer token", http.MethodGet, "", "Bearer " + token, http.StatusOK},
		{"get wrong bearer token", http.MethodGet, "", "Bearer nope", http.StatusUnauthorized},
		// POSTs are authorized by the sessionid the SDK only emits over the
		// authenticated GET stream, so the guard lets them through.
		{"post no token", http.MethodPost, "?sessionid=abc", "", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://localhost:9100/sse"+tc.query, nil)
			if tc.authHdr != "" {
				req.Header.Set("Authorization", tc.authHdr)
			}
			rec := httptest.NewRecorder()
			guard.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// TestNewSessionTokenUnique verifies tokens are non-empty and not repeated.
func TestNewSessionTokenUnique(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		tok, err := newSessionToken()
		if err != nil {
			t.Fatalf("newSessionToken failed: %v", err)
		}
		if tok == "" {
			t.Fatal("newSessionToken returned empty string")
		}
		if seen[tok] {
			t.Fatalf("duplicate token generated: %q", tok)
		}
		seen[tok] = true
	}
}

// TestAuthenticatedSSERoundTrip verifies a client presenting the per-session
// token can complete the full SSE handshake (GET stream + POSTed message
// endpoint) through the loopbackGuard+tokenGuard stack, and that a client
// without the token is rejected at connect.
func TestAuthenticatedSSERoundTrip(t *testing.T) {
	const token = "round-trip-token"
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	sse := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	handler := loopbackGuard(tokenGuard("round-trip", token, sse))
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()

	// Without the token, connect must fail (the GET returns 401).
	noAuth := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	if _, err := noAuth.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL + "/sse"}, nil); err == nil {
		t.Fatal("expected connect without token to fail, got nil error")
	}

	// With the token in the URL, the full round-trip (GET + message POSTs)
	// succeeds even though the SDK drops the token from the message endpoint.
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := client.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL + "/sse?token=" + token}, nil)
	if err != nil {
		t.Fatalf("client connect with token failed: %v", err)
	}
	defer func() { _ = cs.Close() }()

	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Fatalf("ListTools over authenticated SSE failed: %v", err)
	}
}

// TestAllowedDDLKinds verifies the kind whitelist accepts standard Snowflake
// object kinds and rejects unknown/malicious strings.
func TestAllowedDDLKinds(t *testing.T) {
	for _, kind := range []string{"TABLE", "VIEW", "FUNCTION", "PROCEDURE", "STAGE"} {
		if !allowedDDLKinds[kind] {
			t.Errorf("expected %q to be allowed", kind)
		}
	}
	for _, kind := range []string{"", "EVIL", "TABLE'; DROP", "table", "DATABASE", "SCHEMA"} {
		if allowedDDLKinds[kind] {
			t.Errorf("expected %q to be rejected", kind)
		}
	}
}

// TestValidateSqlPureMarkers calls validate_sql with a nil client and SQL that
// has syntax errors, verifying that pure (phase-1) markers are returned even
// without a live Snowflake connection.
func TestValidateSqlPureMarkers(t *testing.T) {
	// Unmatched parenthesis produces a syntax error marker.
	markers := validateSQL(context.Background(), nil, "SELECT (1")
	if len(markers) == 0 {
		t.Fatal("expected at least one diagnostic marker for unmatched paren, got 0")
	}
	// All markers from the pure phase should have Severity 8 (Error) or 4 (Warning).
	for i, m := range markers {
		if m.Severity != 8 && m.Severity != 4 {
			t.Errorf("marker[%d].Severity = %d, want 8 or 4", i, m.Severity)
		}
	}
}

// TestValidateSqlEmptyArrayNotNull verifies that clean SQL returns JSON "[]"
// (not "null") through the tool handler, so external clients don't need to
// special-case nil slices.
func TestValidateSqlEmptyArrayNotNull(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "validate_sql",
		Arguments: validateSqlInput{SQL: "SELECT 1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := extractText(t, res)
	if strings.TrimSpace(text) != "[]" {
		t.Errorf("expected empty array [], got: %s", text)
	}
}

// TestFormatSqlInvalidCase verifies that invalid case values are rejected.
func TestFormatSqlInvalidCase(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "format_sql",
		Arguments: formatSqlInput{
			SQL:         "SELECT 1",
			KeywordCase: "INVALID",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected IsError=true for invalid case value, got false")
	}
}

// TestFormatSqlTool calls ApplyCasing through the format_sql path and verifies
// keyword casing is applied.
func TestFormatSqlTool(t *testing.T) {
	cases := []struct {
		name     string
		sql      string
		kwCase   string
		idCase   string
		fnCase   string
		contains string
	}{
		{"uppercase keywords", "select 1", "UPPER", "", "", "SELECT"},
		{"lowercase keywords", "SELECT 1", "lower", "", "", "select"},
		{"preserve empty", "SELECT 1", "", "", "", "SELECT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
			handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
			httpSrv := httptest.NewServer(handler)
			defer httpSrv.Close()

			ctx := context.Background()
			c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
			cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
			if err != nil {
				t.Fatalf("connect: %v", err)
			}
			defer func() { _ = cs.Close() }()

			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name: "format_sql",
				Arguments: formatSqlInput{
					SQL:            tc.sql,
					KeywordCase:    tc.kwCase,
					IdentifierCase: tc.idCase,
					FunctionCase:   tc.fnCase,
				},
			})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			text := extractText(t, res)
			if !strings.Contains(text, tc.contains) {
				t.Errorf("result %q does not contain %q", text, tc.contains)
			}
		})
	}
}

// TestGetSnowflakeKeywordsTool verifies the keyword list is non-empty.
func TestGetSnowflakeKeywordsTool(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_snowflake_keywords",
		Arguments: emptyInput{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	text := extractText(t, res)
	var keywords []string
	if err := json.Unmarshal([]byte(text), &keywords); err != nil {
		t.Fatalf("payload is not valid JSON array: %v", err)
	}
	if len(keywords) < 10 {
		t.Errorf("expected at least 10 keywords, got %d", len(keywords))
	}
}

// TestValidateSqlToolSSE exercises validate_sql through the full SSE transport
// with a nil client (phase-1 only), verifying the handler returns valid JSON.
func TestValidateSqlToolSSE(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "validate_sql",
		Arguments: validateSqlInput{SQL: "SELECT (1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := extractText(t, res)
	// Result should be a JSON array of markers.
	if !strings.HasPrefix(strings.TrimSpace(text), "[") {
		t.Errorf("expected JSON array, got: %.80s", text)
	}
}

// TestSuggestJoinConditionsNilClient verifies the nil-client error is returned
// through the SSE transport.
func TestSuggestJoinConditionsNilClient(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "suggest_join_conditions",
		Arguments: joinSuggestInput{TableA: "orders", TableB: "customers"},
	})
	// Per MCP spec, tool-level errors are surfaced as IsError=true on the
	// result, not as a Go error from CallTool.
	if err != nil {
		t.Fatalf("CallTool returned Go error (expected IsError on result): %v", err)
	}
	if !res.IsError {
		t.Errorf("expected IsError=true for nil client, got false")
	}
}

// TestParseTableParts verifies the quote-aware qualified name parser.
// Unquoted identifiers are uppercased (Snowflake canonical casing);
// quoted identifiers preserve their original case.
func TestParseTableParts(t *testing.T) {
	cases := []struct {
		input                         string
		wantDB, wantSchema, wantTable string
	}{
		// Simple unquoted cases — uppercased.
		{"orders", "", "", "ORDERS"},
		{"public.orders", "", "PUBLIC", "ORDERS"},
		{"mydb.public.orders", "MYDB", "PUBLIC", "ORDERS"},
		// Quoted identifiers with dots — preserve case.
		{`"my.db".public.orders`, "my.db", "PUBLIC", "ORDERS"},
		{`mydb."my.schema".orders`, "MYDB", "my.schema", "ORDERS"},
		{`mydb.public."my.table"`, "MYDB", "PUBLIC", "my.table"},
		{`"a.b"."c.d"."e.f"`, "a.b", "c.d", "e.f"},
		// Escaped quotes inside quoted identifier.
		{`"say""hi".public.t`, `say"hi`, "PUBLIC", "T"},
		// Two-part with quote.
		{`"my.schema".orders`, "", "my.schema", "ORDERS"},
		// Mixed case unquoted — folded to upper.
		{"MyDb.MySchema.MyTable", "MYDB", "MYSCHEMA", "MYTABLE"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := parseTableParts(tc.input)
			if got.db != tc.wantDB || got.schema != tc.wantSchema || got.table != tc.wantTable {
				t.Errorf("parseTableParts(%q) = {%q, %q, %q}, want {%q, %q, %q}",
					tc.input, got.db, got.schema, got.table, tc.wantDB, tc.wantSchema, tc.wantTable)
			}
		})
	}
}

// TestQualifyTableRef verifies session defaults are filled in for missing parts
// and that unquoted identifiers are uppercased.
func TestQualifyTableRef(t *testing.T) {
	got := qualifyTableRef("orders", "MYDB", "PUBLIC")
	if got.db != "MYDB" || got.schema != "PUBLIC" || got.table != "ORDERS" {
		t.Errorf("qualifyTableRef(orders) = %+v, want {MYDB PUBLIC ORDERS}", got)
	}
	got = qualifyTableRef("other_schema.orders", "MYDB", "PUBLIC")
	if got.db != "MYDB" || got.schema != "OTHER_SCHEMA" || got.table != "ORDERS" {
		t.Errorf("qualifyTableRef(other_schema.orders) = %+v", got)
	}
	got = qualifyTableRef("otherdb.other_schema.orders", "MYDB", "PUBLIC")
	if got.db != "OTHERDB" || got.schema != "OTHER_SCHEMA" || got.table != "ORDERS" {
		t.Errorf("qualifyTableRef(otherdb.other_schema.orders) = %+v", got)
	}
	// Quoted identifiers preserve case.
	got = qualifyTableRef(`"myTable"`, "MYDB", "PUBLIC")
	if got.db != "MYDB" || got.schema != "PUBLIC" || got.table != "myTable" {
		t.Errorf("qualifyTableRef(quoted) = %+v, want {MYDB PUBLIC myTable}", got)
	}
}

// TestSfObjsToStoreObjects verifies the snowflake→sqleditor object conversion.
func TestSfObjsToStoreObjects(t *testing.T) {
	objs := []snowflake.SnowflakeObject{
		{Name: "USERS", Kind: "TABLE"},
		{Name: "V_USERS", Kind: "VIEW"},
	}
	got := sfObjsToStoreObjects("MYDB", "PUBLIC", objs)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].DB != "MYDB" || got[0].Schema != "PUBLIC" || got[0].Name != "USERS" || got[0].Kind != "TABLE" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Name != "V_USERS" || got[1].Kind != "VIEW" {
		t.Errorf("got[1] = %+v", got[1])
	}
}

// TestSfColsToColInfo verifies column info conversion.
func TestSfColsToColInfo(t *testing.T) {
	cols := []snowflake.ColumnInfo{
		{Name: "ID", DataType: "NUMBER(38,0)", Nullable: false, IsPrimaryKey: true},
		{Name: "NAME", DataType: "VARCHAR(256)", Nullable: true},
	}
	got := sfColsToColInfo(cols)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Name != "ID" || got[0].DataType != "NUMBER(38,0)" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Name != "NAME" || got[1].DataType != "VARCHAR(256)" {
		t.Errorf("got[1] = %+v", got[1])
	}
}

// TestSfFKsToFKEntries verifies foreign key conversion.
func TestSfFKsToFKEntries(t *testing.T) {
	fks := []snowflake.TableForeignKey{
		{
			PKDatabase: "DB", PKSchema: "PUBLIC", PKTable: "PARENT", PKColumn: "ID",
			FKDatabase: "DB", FKSchema: "PUBLIC", FKTable: "CHILD", FKColumn: "PARENT_ID",
			ConstraintName: "FK_CHILD_PARENT", KeySequence: 1,
		},
	}
	got := sfFKsToFKEntries(fks)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].PKTable != "PARENT" || got[0].FKColumn != "PARENT_ID" || got[0].ConstraintName != "FK_CHILD_PARENT" {
		t.Errorf("got[0] = %+v", got[0])
	}
}

// extractText extracts the text from a single-content CallToolResult.
func extractText(t *testing.T, res *mcpsdk.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcpsdk.TextContent", res.Content[0])
	}
	return tc.Text
}

// TestJSONResultShaping verifies the tool result helpers wrap payloads as a
// single text-content block carrying the indented-JSON encoding. The tool
// handlers all funnel their output through jsonResult/textResult, so this
// covers the result-shaping logic without a live Snowflake client.
func TestJSONResultShaping(t *testing.T) {
	res := jsonResult(map[string]string{"role": "SYSADMIN"})
	if len(res.Content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcpsdk.TextContent", res.Content[0])
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(tc.Text), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["role"] != "SYSADMIN" {
		t.Errorf("decoded[role] = %q, want SYSADMIN", decoded["role"])
	}
}

// ── Mode-gating tests ───────────────────────────────────────────────────────

// toolNames connects to an SSE server, lists tools, and returns sorted names.
func toolNames(t *testing.T, srv *mcpsdk.Server) []string {
	t.Helper()
	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func hasToolName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// TestServerMetadataNoSQLTools verifies that metadata mode does NOT expose
// SQL execution or context-switching tools.
func TestServerMetadataNoSQLTools(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	names := toolNames(t, srv)

	sqlTools := []string{"execute_snowflake_sql", "use_role", "use_warehouse", "use_database", "use_schema"}
	for _, tool := range sqlTools {
		if hasToolName(names, tool) {
			t.Errorf("metadata mode should not expose %q", tool)
		}
	}
}

// TestServerExposesReadonlyTools verifies that readonly mode exposes SQL
// execution and context-switching tools.
func TestServerExposesReadonlyTools(t *testing.T) {
	srv := buildServer(nil, ExecutionModeReadonly, SessionConfig{}, nil, nil, nil, nil)
	names := toolNames(t, srv)

	required := []string{"execute_snowflake_sql", "use_role", "use_warehouse", "use_database", "use_schema"}
	for _, tool := range required {
		if !hasToolName(names, tool) {
			t.Errorf("readonly mode should expose %q (got: %v)", tool, names)
		}
	}
}

// TestServerExposesExplainOnlyTools verifies that explain_only mode exposes
// the same SQL tools as readonly.
func TestServerExposesExplainOnlyTools(t *testing.T) {
	srv := buildServer(nil, ExecutionModeExplainOnly, SessionConfig{}, nil, nil, nil, nil)
	names := toolNames(t, srv)

	required := []string{"execute_snowflake_sql", "use_database", "use_schema"}
	for _, tool := range required {
		if !hasToolName(names, tool) {
			t.Errorf("explain_only mode should expose %q (got: %v)", tool, names)
		}
	}
}

// TestServerPinnedRoleHidesUseRole verifies that when PinnedRole is set,
// the use_role tool is not registered.
func TestServerPinnedRoleHidesUseRole(t *testing.T) {
	cfg := SessionConfig{PinnedRole: true, Role: "ANALYST_RO"}
	srv := buildServer(nil, ExecutionModeReadonly, cfg, nil, nil, nil, nil)
	names := toolNames(t, srv)

	if hasToolName(names, "use_role") {
		t.Error("use_role should not be registered when PinnedRole is true")
	}
	// Other tools should still be present.
	if !hasToolName(names, "execute_snowflake_sql") {
		t.Error("execute_snowflake_sql should still be present")
	}
	if !hasToolName(names, "use_warehouse") {
		t.Error("use_warehouse should still be present when PinnedWarehouse is false")
	}
}

// TestServerPinnedWarehouseHidesUseWarehouse verifies that when
// PinnedWarehouse is set, the use_warehouse tool is not registered.
func TestServerPinnedWarehouseHidesUseWarehouse(t *testing.T) {
	cfg := SessionConfig{PinnedWarehouse: true, Warehouse: "COMPUTE_WH"}
	srv := buildServer(nil, ExecutionModeReadonly, cfg, nil, nil, nil, nil)
	names := toolNames(t, srv)

	if hasToolName(names, "use_warehouse") {
		t.Error("use_warehouse should not be registered when PinnedWarehouse is true")
	}
	if !hasToolName(names, "use_role") {
		t.Error("use_role should still be present when PinnedRole is false")
	}
}

// ── UpdateMode tests ─────────────────────────────────────────────────────────

// TestManagerUpdateMode verifies that UpdateMode swaps the execution mode,
// rejects invalid modes, and returns an error for unknown sessions.
func TestManagerUpdateMode(t *testing.T) {
	m := NewManager(nil)

	// Register a fake session directly (no real HTTP server needed for
	// updateMode — it only rebuilds the MCP server pointer). Set running=true
	// so updateMode's guard check passes.
	s := &session{
		label:     "test",
		connLabel: "acct/user",
		mode:      ExecutionModeMetadata,
		port:      9999,
		cfg:       SessionConfig{},
		running:   true,
	}
	s.server = buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	m.mu.Lock()
	m.sessions["test"] = s
	m.mu.Unlock()

	ctx := context.Background()

	// Valid mode change.
	info, err := m.UpdateMode(ctx, "test", ExecutionModeReadonly)
	if err != nil {
		t.Fatalf("UpdateMode failed: %v", err)
	}
	if info.ExecutionMode != ExecutionModeReadonly {
		t.Errorf("mode = %q, want %q", info.ExecutionMode, ExecutionModeReadonly)
	}

	// Invalid mode rejected.
	if _, err := m.UpdateMode(ctx, "test", "dangerous"); err == nil {
		t.Error("expected error for invalid mode, got nil")
	}

	// Unknown session.
	if _, err := m.UpdateMode(ctx, "nonexistent", ExecutionModeMetadata); err == nil {
		t.Error("expected error for unknown session, got nil")
	}
}

// TestUpdateModeChangesTools verifies that switching from metadata to readonly
// makes execute_snowflake_sql available, and switching back removes it.
func TestUpdateModeChangesTools(t *testing.T) {
	s := &session{
		label:     "tools-test",
		connLabel: "acct/user",
		mode:      ExecutionModeMetadata,
		cfg:       SessionConfig{},
		running:   true,
	}
	s.server = buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)

	// In metadata mode, execute_snowflake_sql should be absent.
	names := toolNames(t, s.server)
	if hasToolName(names, "execute_snowflake_sql") {
		t.Fatal("execute_snowflake_sql should not exist in metadata mode")
	}

	ctx := context.Background()

	// Switch to readonly — execute_snowflake_sql should appear.
	if err := s.updateMode(ctx, ExecutionModeReadonly); err != nil {
		t.Fatalf("updateMode to readonly: %v", err)
	}
	names = toolNames(t, s.server)
	if !hasToolName(names, "execute_snowflake_sql") {
		t.Fatal("execute_snowflake_sql should exist after switching to readonly")
	}

	// Switch back to metadata — execute_snowflake_sql should disappear.
	if err := s.updateMode(ctx, ExecutionModeMetadata); err != nil {
		t.Fatalf("updateMode to metadata: %v", err)
	}
	names = toolNames(t, s.server)
	if hasToolName(names, "execute_snowflake_sql") {
		t.Fatal("execute_snowflake_sql should not exist after switching back to metadata")
	}
}
