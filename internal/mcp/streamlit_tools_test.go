// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/streamlit"
	"thaw/internal/streamlittemplate"
)

// newStreamlitDeploySession creates a client session against a server built in
// readonly mode with a workspace root — the only combination where
// deploy_streamlit is registered. The client is nil, as in the other tool tests.
func newStreamlitDeploySession(t *testing.T, workspaceRoot string) *mcpsdk.ClientSession {
	t.Helper()
	cfg := SessionConfig{WorkspaceRoot: workspaceRoot}
	srv := buildServer(nil, ExecutionModeReadonly, cfg, nil, nil, nil, nil)
	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	t.Cleanup(httpSrv.Close)

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// streamlitAlwaysTools are registered in every mode, with or without a
// workspace: a pure SQL builder and a network-only template listing.
var streamlitAlwaysTools = []string{"build_create_streamlit_sql", "list_streamlit_templates"}

// TestStreamlitToolRegistration covers the gating matrix: the builder and
// template listing are always present, create_streamlit_from_template needs a
// workspace root, and deploy_streamlit needs a workspace root *and* readonly
// mode — it is the only MCP tool that mutates Snowflake, and explain_only
// promises that nothing is executed.
func TestStreamlitToolRegistration(t *testing.T) {
	cases := []struct {
		name       string
		mode       string
		workspace  bool
		wantScaff  bool
		wantDeploy bool
	}{
		{"metadata without workspace", ExecutionModeMetadata, false, false, false},
		{"metadata with workspace", ExecutionModeMetadata, true, true, false},
		{"explain_only with workspace", ExecutionModeExplainOnly, true, true, false},
		{"readonly without workspace", ExecutionModeReadonly, false, false, false},
		{"readonly with workspace", ExecutionModeReadonly, true, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := SessionConfig{}
			if tc.workspace {
				cfg.WorkspaceRoot = t.TempDir()
			}
			srv := buildServer(nil, tc.mode, cfg, nil, nil, nil, nil)
			names := toolNames(t, srv)

			for _, tool := range streamlitAlwaysTools {
				if !hasToolName(names, tool) {
					t.Errorf("%s should be registered in %s mode", tool, tc.mode)
				}
			}
			if got := hasToolName(names, "create_streamlit_from_template"); got != tc.wantScaff {
				t.Errorf("create_streamlit_from_template registered = %v, want %v", got, tc.wantScaff)
			}
			if got := hasToolName(names, "deploy_streamlit"); got != tc.wantDeploy {
				t.Errorf("deploy_streamlit registered = %v, want %v", got, tc.wantDeploy)
			}
		})
	}
}

// TestStreamlitDeployModeSwitch verifies that deploy_streamlit follows a running
// session across mode switches: it appears on the way into readonly and is
// removed again on the way out. This is what listing it in modeSpecificToolNames
// buys — updateMode strips it before re-registering for the new mode.
func TestStreamlitDeployModeSwitch(t *testing.T) {
	cfg := SessionConfig{WorkspaceRoot: t.TempDir()}
	s := &session{
		label:     "streamlit-mode-test",
		connLabel: "acct/user",
		mode:      ExecutionModeMetadata,
		cfg:       cfg,
		running:   true,
	}
	s.server = buildServer(nil, ExecutionModeMetadata, cfg, nil, nil, nil, nil)

	if hasToolName(toolNames(t, s.server), "deploy_streamlit") {
		t.Fatal("deploy_streamlit should not exist in metadata mode")
	}

	ctx := context.Background()

	if err := s.updateMode(ctx, ExecutionModeReadonly); err != nil {
		t.Fatalf("updateMode to readonly: %v", err)
	}
	if !hasToolName(toolNames(t, s.server), "deploy_streamlit") {
		t.Error("deploy_streamlit should exist after switching to readonly")
	}

	// explain_only executes nothing, so the deploy tool must go away again.
	if err := s.updateMode(ctx, ExecutionModeExplainOnly); err != nil {
		t.Fatalf("updateMode to explain_only: %v", err)
	}
	if hasToolName(toolNames(t, s.server), "deploy_streamlit") {
		t.Error("deploy_streamlit should not exist in explain_only mode")
	}

	if err := s.updateMode(ctx, ExecutionModeMetadata); err != nil {
		t.Fatalf("updateMode to metadata: %v", err)
	}
	if hasToolName(toolNames(t, s.server), "deploy_streamlit") {
		t.Error("deploy_streamlit should not exist after switching back to metadata")
	}
}

// TestStreamlitDeployModeSwitchWithoutWorkspace verifies the workspace gate
// survives a mode switch: without a workspace root, entering readonly must not
// conjure deploy_streamlit.
func TestStreamlitDeployModeSwitchWithoutWorkspace(t *testing.T) {
	s := &session{
		label:   "streamlit-no-workspace",
		mode:    ExecutionModeMetadata,
		cfg:     SessionConfig{},
		running: true,
	}
	s.server = buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)

	if err := s.updateMode(context.Background(), ExecutionModeReadonly); err != nil {
		t.Fatalf("updateMode to readonly: %v", err)
	}
	if hasToolName(toolNames(t, s.server), "deploy_streamlit") {
		t.Error("deploy_streamlit should stay unregistered without a workspace root")
	}
}

// ── build_create_streamlit_sql tests ────────────────────────────────────────

// TestBuildCreateStreamlitSqlValidation verifies the required db/schema inputs,
// matching the other pure builder tools.
func TestBuildCreateStreamlitSqlValidation(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		in      buildCreateStreamlitInput
		wantErr string
	}{
		{
			name:    "empty database",
			in:      buildCreateStreamlitInput{Schema: "PUBLIC", Config: streamlit.StreamlitConfig{Name: "APP"}},
			wantErr: "database is required",
		},
		{
			name:    "empty schema",
			in:      buildCreateStreamlitInput{Database: "MYDB", Config: streamlit.StreamlitConfig{Name: "APP"}},
			wantErr: "schema is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "build_create_streamlit_sql",
				Arguments: tc.in,
			})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if !res.IsError {
				t.Fatalf("expected IsError=true for %s", tc.name)
			}
			if text := extractText(t, res); !strings.Contains(text, tc.wantErr) {
				t.Errorf("error should mention %q, got: %s", tc.wantErr, text)
			}
		})
	}
}

// TestBuildCreateStreamlitSqlSuccess verifies the tool returns the modern
// FROM <stage> + MAIN_FILE grammar without touching Snowflake.
func TestBuildCreateStreamlitSqlSuccess(t *testing.T) {
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "build_create_streamlit_sql",
		Arguments: buildCreateStreamlitInput{
			Database: "MYDB",
			Schema:   "PUBLIC",
			Config: streamlit.StreamlitConfig{
				Name:           "SALES_APP",
				StageLocation:  "@MYDB.PUBLIC.APP_STAGE",
				MainFile:       "streamlit_app.py",
				QueryWarehouse: "COMPUTE_WH",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	for _, want := range []string{"CREATE STREAMLIT", "SALES_APP", "FROM @MYDB.PUBLIC.APP_STAGE", "MAIN_FILE = 'streamlit_app.py'", "QUERY_WAREHOUSE"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in generated SQL, got: %s", want, text)
		}
	}
}

// ── attribution tests ───────────────────────────────────────────────────────

// TestTemplateAttribution verifies the provenance stamped on every template
// result names the upstream repo, its URL, and its license — the credit the
// Apache-2.0 terms require wherever the templates are offered (issue #847
// step 5), not just in the Thaw UI.
func TestTemplateAttribution(t *testing.T) {
	src := templateSource()
	if src.Repository != "Snowflake-Labs/snowflake-demo-streamlit" {
		t.Errorf("repository = %q, want Snowflake-Labs/snowflake-demo-streamlit", src.Repository)
	}
	if src.URL != streamlittemplate.RepoURL {
		t.Errorf("url = %q, want %q", src.URL, streamlittemplate.RepoURL)
	}
	if src.License != "Apache-2.0" {
		t.Errorf("license = %q, want Apache-2.0", src.License)
	}
}

// TestTemplateResultsCarryAttribution verifies both template-tool payloads
// serialize the source block, so an MCP client can surface the credit.
func TestTemplateResultsCarryAttribution(t *testing.T) {
	listJSON := extractText(t, jsonResult(listStreamlitTemplatesResult{
		Templates: []streamlittemplate.Template{{Name: "Inventory Tracker"}},
		Source:    templateSource(),
	}))
	scaffoldJSON := extractText(t, jsonResult(createStreamlitFromTemplateResult{
		TemplateName: "Inventory Tracker",
		Source:       templateSource(),
	}))

	for _, payload := range []struct{ name, json string }{
		{"list_streamlit_templates", listJSON},
		{"create_streamlit_from_template", scaffoldJSON},
	} {
		for _, want := range []string{`"repository"`, "Snowflake-Labs/snowflake-demo-streamlit", streamlittemplate.RepoURL, "Apache-2.0"} {
			if !strings.Contains(payload.json, want) {
				t.Errorf("%s result should carry %q, got: %s", payload.name, want, payload.json)
			}
		}
	}
}

// ── create_streamlit_from_template tests ────────────────────────────────────

// TestCreateStreamlitFromTemplateValidation verifies required inputs are
// rejected before any network call is made.
func TestCreateStreamlitFromTemplateValidation(t *testing.T) {
	workspace := t.TempDir()
	cs := newWorkspaceTestSession(t, workspace)
	ctx := context.Background()

	cases := []struct {
		name    string
		in      createStreamlitFromTemplateInput
		wantErr string
	}{
		{
			name:    "empty template name",
			in:      createStreamlitFromTemplateInput{TemplateName: "  ", DestDir: filepath.Join(workspace, "app")},
			wantErr: "templateName is required",
		},
		{
			name:    "empty dest dir",
			in:      createStreamlitFromTemplateInput{TemplateName: "Inventory Tracker", DestDir: ""},
			wantErr: "destDir is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      "create_streamlit_from_template",
				Arguments: tc.in,
			})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if !res.IsError {
				t.Fatalf("expected IsError=true for %s", tc.name)
			}
			if text := extractText(t, res); !strings.Contains(text, tc.wantErr) {
				t.Errorf("error should mention %q, got: %s", tc.wantErr, text)
			}
		})
	}
}

// TestCreateStreamlitFromTemplateSandbox verifies destinations outside the
// workspace are refused — including a symlink planted inside the workspace that
// points out of it, and a relative traversal — before any download happens.
func TestCreateStreamlitFromTemplateSandbox(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(workspace, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	cs := newWorkspaceTestSession(t, workspace)
	ctx := context.Background()

	cases := []struct {
		name    string
		destDir string
	}{
		{"absolute path outside workspace", filepath.Join(outside, "app")},
		{"relative traversal out of workspace", filepath.Join(workspace, "..", "escaped-app")},
		{"symlink inside workspace pointing out", filepath.Join(link, "app")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name: "create_streamlit_from_template",
				Arguments: createStreamlitFromTemplateInput{
					TemplateName: "Inventory Tracker",
					DestDir:      tc.destDir,
				},
			})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if !res.IsError {
				t.Fatalf("expected IsError=true for %s", tc.name)
			}
			if text := extractText(t, res); !strings.Contains(text, "access denied") {
				t.Errorf("error should mention access denied, got: %s", text)
			}
			if _, err := os.Stat(filepath.Join(outside, "app")); !os.IsNotExist(err) {
				t.Errorf("nothing should have been written outside the workspace")
			}
		})
	}
}

// ── deploy_streamlit tests ──────────────────────────────────────────────────

// TestDeployStreamlitFieldValidation verifies every required field is checked,
// and that none of them is silently defaulted from the session context.
func TestDeployStreamlitFieldValidation(t *testing.T) {
	workspace := t.TempDir()
	cs := newStreamlitDeploySession(t, workspace)
	ctx := context.Background()

	valid := deployStreamlitInput{
		Database: "MYDB",
		Schema:   "PUBLIC",
		Name:     "SALES_APP",
		LocalDir: workspace,
		MainFile: "streamlit_app.py",
	}

	cases := []struct {
		name    string
		mutate  func(*deployStreamlitInput)
		wantErr string
	}{
		{"missing database", func(in *deployStreamlitInput) { in.Database = "" }, "database is required"},
		{"blank schema", func(in *deployStreamlitInput) { in.Schema = "   " }, "schema is required"},
		{"missing name", func(in *deployStreamlitInput) { in.Name = "" }, "name is required"},
		{"missing localDir", func(in *deployStreamlitInput) { in.LocalDir = "" }, "localDir is required"},
		{"missing mainFile", func(in *deployStreamlitInput) { in.MainFile = "" }, "mainFile is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := valid
			tc.mutate(&in)
			res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "deploy_streamlit", Arguments: in})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if !res.IsError {
				t.Fatalf("expected IsError=true for %s", tc.name)
			}
			if text := extractText(t, res); !strings.Contains(text, tc.wantErr) {
				t.Errorf("error should mention %q, got: %s", tc.wantErr, text)
			}
		})
	}
}

// TestDeployStreamlitNilClient verifies a deploy without a connection fails
// cleanly rather than panicking.
func TestDeployStreamlitNilClient(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "streamlit_app.py"), []byte("import streamlit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := newStreamlitDeploySession(t, workspace)

	res, err := cs.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "deploy_streamlit",
		Arguments: deployStreamlitInput{
			Database: "MYDB",
			Schema:   "PUBLIC",
			Name:     "SALES_APP",
			LocalDir: workspace,
			MainFile: "streamlit_app.py",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for nil client")
	}
	if text := extractText(t, res); !strings.Contains(text, "no active Snowflake connection") {
		t.Errorf("error should mention no connection, got: %s", text)
	}
}

// TestDeployStreamlitNilClientBeforePathValidation verifies the connection
// check fires before path validation, so a client with no connection cannot
// probe the filesystem through the differing validation errors (mirrors
// generate_dbt_project).
func TestDeployStreamlitNilClientBeforePathValidation(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	cs := newStreamlitDeploySession(t, workspace)

	res, err := cs.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "deploy_streamlit",
		Arguments: deployStreamlitInput{
			Database: "MYDB",
			Schema:   "PUBLIC",
			Name:     "SALES_APP",
			LocalDir: outside,
			MainFile: "streamlit_app.py",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true")
	}
	if text := extractText(t, res); !strings.Contains(text, "no active Snowflake connection") {
		t.Errorf("nil-client check should fire before path validation, got: %s", text)
	}
}

// TestValidateDeployStreamlitPaths covers the deploy sandbox directly: the
// handler reaches these checks only with a live connection, so they are tested
// through the helper rather than an MCP round-trip.
func TestValidateDeployStreamlitPaths(t *testing.T) {
	workspace := t.TempDir()
	appDir := filepath.Join(workspace, "app")
	if err := os.MkdirAll(filepath.Join(appDir, "pages"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"streamlit_app.py", "pages/page_1.py"} {
		if err := os.WriteFile(filepath.Join(appDir, filepath.FromSlash(rel)), []byte("import streamlit\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	outside := t.TempDir()
	outsideApp := filepath.Join(outside, "secret_app.py")
	if err := os.WriteFile(outsideApp, []byte("import streamlit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkedDir := filepath.Join(workspace, "escape")
	symlinksOK := os.Symlink(outside, linkedDir) == nil

	cases := []struct {
		name     string
		localDir string
		mainFile string
		wantErr  string // "" means the input must be accepted
		needLink bool
	}{
		{name: "app folder inside workspace", localDir: appDir, mainFile: "streamlit_app.py"},
		{name: "entrypoint in a subdirectory", localDir: appDir, mainFile: "pages/page_1.py"},
		{name: "workspace root itself", localDir: workspace, mainFile: "app/streamlit_app.py"},
		{name: "app folder outside workspace", localDir: outside, mainFile: "secret_app.py", wantErr: "access denied"},
		{name: "symlinked app folder escaping workspace", localDir: linkedDir, mainFile: "secret_app.py", wantErr: "access denied", needLink: true},
		{name: "absolute entrypoint", localDir: appDir, mainFile: outsideApp, wantErr: "relative to localDir"},
		{name: "entrypoint climbing out of the app folder", localDir: appDir, mainFile: "../../secret_app.py", wantErr: "invalid mainFile"},
		{name: "entrypoint that does not exist", localDir: appDir, mainFile: "missing.py", wantErr: "invalid mainFile"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.needLink && !symlinksOK {
				t.Skip("symlinks unavailable")
			}
			err := validateDeployStreamlitPaths(tc.localDir, tc.mainFile, workspace)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected the input to be accepted, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %q, got no error", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

// TestValidateDeployStreamlitFieldsTrims verifies the entrypoint is returned
// trimmed, so a padded value still resolves against the app folder.
func TestValidateDeployStreamlitFieldsTrims(t *testing.T) {
	mainFile, err := validateDeployStreamlitFields(deployStreamlitInput{
		Database: "MYDB",
		Schema:   "PUBLIC",
		Name:     "APP",
		LocalDir: "/tmp/app",
		MainFile: "  streamlit_app.py  ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mainFile != "streamlit_app.py" {
		t.Errorf("mainFile = %q, want it trimmed", mainFile)
	}
}
