// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/filesystem"
	"thaw/internal/snowflake"
	"thaw/internal/streamlit"
	"thaw/internal/streamlittemplate"
)

// Tool input types for the Streamlit tools.

type buildCreateStreamlitInput struct {
	Database string                    `json:"database" jsonschema:"the database name"`
	Schema   string                    `json:"schema" jsonschema:"the schema name"`
	Config   streamlit.StreamlitConfig `json:"config" jsonschema:"the streamlit configuration (name, stageLocation, mainFile, and optional queryWarehouse/title/comment)"`
}

type createStreamlitFromTemplateInput struct {
	TemplateName string `json:"templateName" jsonschema:"the template folder name as returned by list_streamlit_templates"`
	DestDir      string `json:"destDir" jsonschema:"the destination folder to scaffold into; must be inside the workspace root and must be empty or not yet exist"`
}

// deployStreamlitInput mirrors streamlit.DeployStreamlitParams but is declared
// here so the JSON schema can spell out the safety rails to the AI client:
// database/schema/name are required with no fallback to the session context, and
// orReplace is an explicit opt-in.
type deployStreamlitInput struct {
	Database       string `json:"database" jsonschema:"the target database; required — the session's current database is never used as a default"`
	Schema         string `json:"schema" jsonschema:"the target schema; required — the session's current schema is never used as a default"`
	Name           string `json:"name" jsonschema:"the name of the STREAMLIT object to create; required"`
	CaseSensitive  bool   `json:"caseSensitive,omitempty" jsonschema:"when true the name is double-quoted exactly as given, otherwise it is an unquoted identifier"`
	LocalDir       string `json:"localDir" jsonschema:"absolute path of the local Streamlit app folder to upload; must be inside the workspace root"`
	MainFile       string `json:"mainFile" jsonschema:"the entrypoint relative to localDir, e.g. streamlit_app.py; create_streamlit_from_template reports the detected one"`
	OrReplace      bool   `json:"orReplace,omitempty" jsonschema:"opt in to CREATE OR REPLACE, overwriting an app of the same name; when false (the default) deploying onto an existing name fails"`
	QueryWarehouse string `json:"queryWarehouse,omitempty" jsonschema:"optional warehouse the app runs its queries on"`
	Title          string `json:"title,omitempty" jsonschema:"optional display title"`
	Comment        string `json:"comment,omitempty" jsonschema:"optional comment"`
}

// templateAttribution is the provenance carried on every template result. The
// upstream repo is Apache-2.0, and the license requires attribution wherever the
// templates are offered — the MCP surface included, not just the Thaw UI.
type templateAttribution struct {
	Repository string `json:"repository"`
	URL        string `json:"url"`
	License    string `json:"license"`
}

// listStreamlitTemplatesResult is the catalog plus its attribution.
type listStreamlitTemplatesResult struct {
	Templates []streamlittemplate.Template `json:"templates"`
	// Degraded is true when the live listing could not be fetched and Templates
	// holds a small built-in fallback; Note says why.
	Degraded bool                `json:"degraded"`
	Note     string              `json:"note,omitempty"`
	Source   templateAttribution `json:"source"`
}

// createStreamlitFromTemplateResult reports where the template landed and which
// entrypoint it has, so the client can chain straight into deploy_streamlit
// without a separate detection round-trip.
type createStreamlitFromTemplateResult struct {
	TemplateName string              `json:"templateName"`
	DestDir      string              `json:"destDir"`
	MainFile     string              `json:"mainFile"`
	Candidates   []string            `json:"candidates,omitempty"`
	Source       templateAttribution `json:"source"`
}

// deployStreamlitResult reports the created app and the exact statement that ran.
type deployStreamlitResult struct {
	Database  string `json:"database"`
	Schema    string `json:"schema"`
	Name      string `json:"name"`
	MainFile  string `json:"mainFile"`
	Statement string `json:"statement"`
}

// templateSource is the attribution stamped on every template tool result.
func templateSource() templateAttribution {
	return templateAttribution{
		Repository: streamlittemplate.Repo,
		URL:        streamlittemplate.RepoURL,
		License:    streamlittemplate.License,
	}
}

// registerStreamlitTools wires the Streamlit tools that are not mode-gated onto
// srv: the pure CREATE STREAMLIT builder and the template catalog are registered
// in every mode, and the template scaffolder is workspace-gated (it writes into
// the workspace, so it is only registered when workspaceRoot is non-empty and its
// destination is validated against that root).
//
// The one mutating tool, deploy_streamlit, lives in registerStreamlitModeTools.
func registerStreamlitTools(srv *mcpsdk.Server, workspaceRoot string) {

	// ── Always-registered tools ─────────────────────────────────────────

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_create_streamlit_sql",
		Description: "Generate a CREATE STREAMLIT DDL statement from a streamlit configuration. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in buildCreateStreamlitInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		sql, err := streamlit.BuildCreateStreamlitSql(in.Database, in.Schema, in.Config)
		if err != nil {
			return nil, nil, err
		}
		return textResult(sql), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "list_streamlit_templates",
		Description: "List the Streamlit app templates available from the " + streamlittemplate.Repo +
			" repository (" + streamlittemplate.License + "), each with a short description. " +
			"Reads from GitHub; no Snowflake connection is used. When the repository cannot be reached the " +
			"result is marked degraded and carries a small built-in fallback list.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
		catalog := streamlittemplate.ListTemplates(ctx)
		return jsonResult(listStreamlitTemplatesResult{
			Templates: catalog.Templates,
			Degraded:  catalog.Degraded,
			Note:      catalog.Note,
			Source:    templateSource(),
		}), nil, nil
	})

	// ── Workspace-gated tools ───────────────────────────────────────────

	if workspaceRoot == "" {
		return
	}

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "create_streamlit_from_template",
		Description: "Scaffold a Streamlit app template from " + streamlittemplate.Repo + " (" + streamlittemplate.License +
			") into a local folder, together with the upstream LICENSE and a NOTICE recording its provenance. " +
			"The destination must be inside the configured workspace root and must be empty or not yet exist. " +
			"Returns the detected entrypoint so the folder can be handed straight to deploy_streamlit.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in createStreamlitFromTemplateInput) (*mcpsdk.CallToolResult, any, error) {
		if strings.TrimSpace(in.TemplateName) == "" {
			return nil, nil, fmt.Errorf("templateName is required")
		}
		if strings.TrimSpace(in.DestDir) == "" {
			return nil, nil, fmt.Errorf("destDir is required")
		}
		// The destination legitimately may not exist yet, so containment is
		// checked via the nearest existing ancestor (same rule git_get_head_file
		// uses for deleted files).
		if err := filesystem.ValidatePathOrAncestorInsideOrEqual(in.DestDir, workspaceRoot); err != nil {
			return nil, nil, fmt.Errorf("access denied: %w", err)
		}
		if err := streamlittemplate.DownloadTemplate(ctx, in.TemplateName, in.DestDir); err != nil {
			return nil, nil, err
		}
		// Best-effort: a template whose entrypoint can't be detected still
		// scaffolded fine, and the caller can pick from Candidates.
		detected, _ := streamlit.DetectStreamlitMainFile(in.DestDir)
		return jsonResult(createStreamlitFromTemplateResult{
			TemplateName: in.TemplateName,
			DestDir:      in.DestDir,
			MainFile:     detected.MainFile,
			Candidates:   detected.Candidates,
			Source:       templateSource(),
		}), nil, nil
	})
}

// validateDeployStreamlitFields checks the required deploy inputs and returns
// the trimmed entrypoint. Nothing is defaulted from the session context: a
// deploy names its target explicitly or it does not happen.
func validateDeployStreamlitFields(in deployStreamlitInput) (string, error) {
	if strings.TrimSpace(in.Database) == "" {
		return "", fmt.Errorf("database is required")
	}
	if strings.TrimSpace(in.Schema) == "" {
		return "", fmt.Errorf("schema is required")
	}
	if strings.TrimSpace(in.Name) == "" {
		return "", fmt.Errorf("name is required")
	}
	if strings.TrimSpace(in.LocalDir) == "" {
		return "", fmt.Errorf("localDir is required")
	}
	mainFile := strings.TrimSpace(in.MainFile)
	if mainFile == "" {
		return "", fmt.Errorf("mainFile is required")
	}
	return mainFile, nil
}

// validateDeployStreamlitPaths keeps a deploy inside the sandbox: the app folder
// must resolve inside the workspace root, and the entrypoint must be a relative
// path resolving inside that folder — so an app can neither be uploaded from
// outside the workspace nor claim a main file it does not contain. Both checks
// resolve symlinks, so a link planted inside the workspace cannot tunnel out.
func validateDeployStreamlitPaths(localDir, mainFile, workspaceRoot string) error {
	if err := filesystem.ValidateInsideOrEqual(localDir, workspaceRoot); err != nil {
		return fmt.Errorf("access denied: %w", err)
	}
	if filepath.IsAbs(mainFile) {
		return fmt.Errorf("mainFile must be a path relative to localDir")
	}
	if err := filesystem.ValidateInsideOrEqual(filepath.Join(localDir, filepath.FromSlash(mainFile)), localDir); err != nil {
		return fmt.Errorf("invalid mainFile: %w", err)
	}
	return nil
}

// registerStreamlitModeTools registers deploy_streamlit, which is gated twice
// over. Called from both buildServer (initial setup) and updateMode (mode
// switches); "deploy_streamlit" is listed in modeSpecificToolNames so updateMode
// removes it before calling this function.
//
// It is the only MCP tool that mutates Snowflake — it creates a temporary stage,
// PUTs the local app files into it, and runs CREATE STREAMLIT — so it is
// registered only when:
//
//   - a workspace root is configured (it reads local files, path-validated), and
//   - the mode is readonly, the one mode that actually executes statements.
//
// explain_only is deliberately excluded even though it also registers
// execute_snowflake_sql: that mode's contract is that a statement is never
// actually executed, and a deploy would silently break it.
func registerStreamlitModeTools(srv *mcpsdk.Server, client *snowflake.Client, workspaceRoot, mode string) {
	if workspaceRoot == "" || mode != ExecutionModeReadonly {
		return
	}

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "deploy_streamlit",
		Description: "Deploy a local Streamlit app folder to Snowflake: upload it to a temporary stage and create a STREAMLIT object from it. " +
			"This MUTATES Snowflake — it creates an object in the target schema. " +
			"database, schema and name are required and are never defaulted from the session context; " +
			"set orReplace only when overwriting the named app is intended. " +
			"The app folder must be inside the configured workspace root. Returns the CREATE STREAMLIT statement that was executed.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in deployStreamlitInput) (*mcpsdk.CallToolResult, any, error) {
		mainFile, err := validateDeployStreamlitFields(in)
		if err != nil {
			return nil, nil, err
		}
		// The connection check precedes path validation on purpose: without a
		// client the tool can do nothing anyway, and answering path questions
		// first would let a caller probe the filesystem through the differing
		// validation errors (same ordering as generate_dbt_project).
		if client == nil {
			return nil, nil, fmt.Errorf("no active Snowflake connection")
		}
		if err := validateDeployStreamlitPaths(in.LocalDir, mainFile, workspaceRoot); err != nil {
			return nil, nil, err
		}

		stmt, err := streamlit.DeployStreamlit(ctx, client, streamlit.DeployStreamlitParams{
			Database:       in.Database,
			Schema:         in.Schema,
			Name:           in.Name,
			CaseSensitive:  in.CaseSensitive,
			LocalDir:       in.LocalDir,
			MainFile:       mainFile,
			OrReplace:      in.OrReplace,
			QueryWarehouse: in.QueryWarehouse,
			Title:          in.Title,
			Comment:        in.Comment,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(deployStreamlitResult{
			Database:  in.Database,
			Schema:    in.Schema,
			Name:      in.Name,
			MainFile:  mainFile,
			Statement: stmt,
		}), nil, nil
	})
}
