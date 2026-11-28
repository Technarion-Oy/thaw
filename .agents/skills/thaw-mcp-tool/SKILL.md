---
name: thaw-mcp-tool
description: >-
  Use this skill when adding or modifying tools, execution modes, security middleware,
  or state stores in Thaw's built-in Snowflake MCP server (`internal/mcp/`).
---

# Thaw MCP Server Tool Workflow

Follow this procedure when extending or modifying Thaw's localhost MCP server (`internal/mcp/`). Read `internal/mcp/README.md` before starting.

## 1. Respect Architectural & Security Boundaries
- **`internal/mcp` must NEVER import `internal/app`** (import cycle). Access Wails events or external services via injected callbacks/interfaces (e.g. `emit` closure, `NotebookBackend`, `EditorContextStore`, `ERDesignerStateStore`, `OpenFormStateStore`).
- **Critical Go MCP SDK invariant**: `mcp.AddTool[In, Out]` panics at registration if `Out`'s JSON schema is not `"object"`. **Always declare `Out` as `any`**, return `nil` as the structured result, and return payload via `jsonResult` / `textResult`.
- **Workspace sandboxing**: every tool accepting a local file or directory path MUST be workspace-gated (`cfg.WorkspaceRoot != ""`) and validate containment with `filesystem.ValidateInsideOrEqual` (or `ValidatePathOrAncestorInsideOrEqual` when the target may not exist yet).
- **SQL execution safety**: never execute arbitrary user/AI SQL without routing through the three-layer EXPLAIN precompilation gate (`gate.go` / `executeSQLPipeline` in `sql_tools.go`). Read-only metadata tools must call safe Go client methods directly.
- **Mutation policy**: tools should read, generate SQL for the user, or write within the sandboxed workspace — do not add Snowflake-mutating tools without explicit double-gating and review (see `deploy_streamlit` in `streamlit_tools.go`).

## 2. Place & Register the Tool
1. Add the input struct, handler, and registration in the matching `internal/mcp/*_tools.go` file (e.g. `schema_tools.go`, `account_tools.go`, `spcs_tools.go`, `builder_tools.go`, `pipeline_tools.go`, `workspace_tools.go`).
2. Wire registration into `buildServer` in `internal/mcp/server.go` according to its gating requirements:
   - **Always registered** (metadata + readonly + explain_only)
   - **Mode-gated** (`readonly` / `explain_only`): register in **both** `buildServer` and `session.updateMode`, AND add the tool name to `modeSpecificToolNames` in `server.go` so live mode switches remove and re-register it cleanly.
   - **Dependency-gated**: guard on `cfg.WorkspaceRoot != ""`, `emit != nil`, `editorCtx != nil`, `fnStore != nil`, or `nb != nil`.
3. If the tool requires new `*App` IPC wiring (for example pushing frontend UI state into a store), add a thin delegator in `internal/app/` and run `wails generate module`.

## 3. Write Unit Tests
Add tests in the corresponding `internal/mcp/*_tools_test.go` covering:
- Registration across the full gating matrix (execution modes × optional dependencies, plus `updateMode` transitions if mode-gated).
- Empty/invalid inputs and nil-client handling.
- Path-escape and symlink-escape tests if workspace-gated.
- Success path verifying structured JSON output (or emitted event payload).

## 4. Verify & Update Docs
1. Run:
   ```bash
   go test -race ./internal/mcp/...
   go test ./internal/app/...
   ```
2. Update `internal/mcp/README.md` (tool count, table, gating notes), `FEATURES.md`, and `docs/concepts/patterns.md` if you changed a cross-cutting MCP pattern.
