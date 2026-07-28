// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"testing"
)

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
