// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/fileformat"
	"thaw/internal/logger"
	"thaw/internal/pipe"
	"thaw/internal/snowflake"
	"thaw/internal/stage"
	"thaw/internal/tasks"
)

// Tool input types for pipeline (task, stage, pipe) tools.

type listTasksInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema" jsonschema:"the schema name"`
}

type taskRunHistoryInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema" jsonschema:"the schema name"`
	Name     string `json:"name" jsonschema:"the task name"`
	IsRoot   bool   `json:"is_root,omitempty" jsonschema:"if true, fetches root task run history (default false)"`
	Days     int    `json:"days,omitempty" jsonschema:"number of days of history (default 7, max 30)"`
}

type taskDependenciesInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema" jsonschema:"the schema name"`
	Task     string `json:"task" jsonschema:"the root task name"`
}

type listStageFilesInput struct {
	Stage   string `json:"stage" jsonschema:"fully qualified stage name, e.g. @DB.SCHEMA.STAGE or @DB.SCHEMA.STAGE/path/"`
	Pattern string `json:"pattern,omitempty" jsonschema:"optional regex pattern to filter file names"`
}

type previewStageFileInput struct {
	StagePath      string `json:"stage_path" jsonschema:"fully qualified stage path, e.g. @DB.SCHEMA.STAGE/path/to/file.csv"`
	Type           string `json:"type" jsonschema:"file type: CSV, JSON, AVRO, ORC, PARQUET, or XML"`
	FieldDelimiter string `json:"field_delimiter,omitempty" jsonschema:"field delimiter for CSV (default comma)"`
	SkipHeader     int    `json:"skip_header,omitempty" jsonschema:"number of header lines to skip for CSV (default 0)"`
	ParseHeader    bool   `json:"parse_header,omitempty" jsonschema:"use the first row as column names for CSV (default false)"`
	Compression    string `json:"compression,omitempty" jsonschema:"compression type: AUTO, GZIP, BZ2, BROTLI, ZSTD, DEFLATE, RAW_DEFLATE, NONE"`
}

type pipeStatusInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema" jsonschema:"the schema name"`
	Name     string `json:"name" jsonschema:"the pipe name"`
}

type pipeCopyHistoryInput struct {
	Database  string `json:"database" jsonschema:"the database name"`
	Schema    string `json:"schema" jsonschema:"the schema name"`
	Name      string `json:"name" jsonschema:"the pipe name"`
	StartTime string `json:"start_time,omitempty" jsonschema:"optional ISO-8601 start time (default: 24 hours ago)"`
	Status    string `json:"status,omitempty" jsonschema:"optional status filter: LOADED, LOAD_FAILED, PARTIALLY_LOADED"`
	FileName  string `json:"file_name,omitempty" jsonschema:"optional file name substring filter"`
}

type openTaskGraphInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema" jsonschema:"the schema name"`
	Task     string `json:"task" jsonschema:"the root task name"`
}

// OpenTaskGraphPayload is the Wails event payload for "mcp:open-task-graph".
// The frontend listens for this event and opens the task graph modal.
type OpenTaskGraphPayload struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Task     string `json:"task"`
}

// taskDependenciesResult is the composite result for get_task_dependencies.
type taskDependenciesResult struct {
	TopologicalOrder tasks.TopologicalOrder `json:"topologicalOrder"`
	HasChildren      bool                   `json:"hasChildren"`
}

// hasChildrenFromRows derives whether taskName appears as a predecessor in any
// of the given status rows, avoiding a redundant SHOW TASKS round-trip that
// tasks.HasChildren would make. The predecessors field is a JSON array or
// Snowflake-quoted bracket syntax; we parse each entry's last dot-segment and
// compare case-insensitively, matching the logic in tasks.HasChildren.
func hasChildrenFromRows(rows []tasks.StatusRow, taskName string) bool {
	upper := strings.ToUpper(taskName)
	for _, r := range rows {
		preds := r.Predecessors
		if preds == "" || preds == "[]" || preds == "<nil>" || preds == "null" {
			continue
		}
		// Try JSON array first (e.g. ["DB.SCH.TASK_A"]).
		var jsonArr []string
		if json.Unmarshal([]byte(preds), &jsonArr) == nil {
			for _, ref := range jsonArr {
				segs := strings.Split(ref, ".")
				last := strings.Trim(segs[len(segs)-1], `"`)
				if strings.ToUpper(last) == upper {
					return true
				}
			}
			continue
		}
		// Fallback: bracket syntax like ["DB"."SCH"."TASK_A"].
		p := strings.TrimSuffix(strings.TrimPrefix(preds, "["), "]")
		for _, part := range strings.Split(p, ",") {
			segs := strings.Split(strings.TrimSpace(part), ".")
			last := strings.Trim(segs[len(segs)-1], `" `)
			if strings.ToUpper(last) == upper {
				return true
			}
		}
	}
	return false
}

// registerPipelineTools wires task graph, stage, and pipe inspection tools
// onto srv. Most tools are always-on metadata operations. preview_stage_file
// is mode-gated (readonly/explain_only only). open_task_graph is emit-gated.
func registerPipelineTools(srv *mcpsdk.Server, client *snowflake.Client, mode string, emit func(string, interface{})) {

	// ── Task tools (always-on) ────────────────────────────────────────────

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_tasks",
		Description: "List tasks in a schema with their current state, predecessors, and last run status.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in listTasksInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		result, err := tasks.GetStatuses(ctx, client, in.Database, in.Schema)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(result), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_task_run_history",
		Description: "Return run history for a task. Set is_root=true for root task " +
			"history (all tasks in the graph). Days defaults to 7 (max 30).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in taskRunHistoryInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		days := in.Days
		if days <= 0 {
			days = 7
		}
		if days > 30 {
			days = 30
		}
		rows, err := tasks.GetTaskRunHistory(ctx, client, in.Database, in.Schema, in.Name, in.IsRoot, days)
		if err != nil {
			return nil, nil, err
		}
		if len(rows) > maxMCPResultRows {
			rows = rows[:maxMCPResultRows]
		}
		return jsonResult(rows), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_task_dependencies",
		Description: "Return the topological order (execution sequence) and child status " +
			"for a root task's dependency graph.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in taskDependenciesInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if in.Task == "" {
			return nil, nil, fmt.Errorf("task is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		statuses, err := tasks.GetStatuses(ctx, client, in.Database, in.Schema)
		if err != nil {
			return nil, nil, err
		}
		topo := tasks.GetTopologicalOrder(statuses.Rows, in.Task)
		// Derive HasChildren from the already-fetched rows to avoid a
		// redundant SHOW TASKS round-trip (tasks.HasChildren makes one).
		children := hasChildrenFromRows(statuses.Rows, in.Task)
		return jsonResult(taskDependenciesResult{
			TopologicalOrder: topo,
			HasChildren:      children,
		}), nil, nil
	})

	// ── Stage tools (always-on) ───────────────────────────────────────────

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "list_stage_files",
		Description: "List files in a Snowflake stage. Stage must be a fully qualified " +
			"name, e.g. @DB.SCHEMA.STAGE or @DB.SCHEMA.STAGE/subpath/. " +
			"Optionally filter by a regex pattern.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in listStageFilesInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Stage == "" {
			return nil, nil, fmt.Errorf("stage is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		files, err := stage.ListStageFiles(ctx, client, in.Stage, in.Pattern)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(files), nil, nil
	})

	// ── Stage preview (mode-gated: readonly / explain_only) ───────────────

	if mode == ExecutionModeReadonly || mode == ExecutionModeExplainOnly {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name: "preview_stage_file",
			Description: "Preview up to 50 rows from a stage file. Requires a stage path " +
				"and file type (CSV, JSON, AVRO, ORC, PARQUET, XML). " +
				"Optional parameters control CSV parsing (field_delimiter, skip_header, " +
				"parse_header) and compression.",
		}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in previewStageFileInput) (*mcpsdk.CallToolResult, any, error) {
			if in.StagePath == "" {
				return nil, nil, fmt.Errorf("stage_path is required")
			}
			if in.Type == "" {
				return nil, nil, fmt.Errorf("type is required")
			}
			if client == nil {
				return nil, nil, fmt.Errorf("no Snowflake connection available")
			}
			cfg := fileformat.FileFormatConfig{
				Type:           in.Type,
				FieldDelimiter: in.FieldDelimiter,
				SkipHeader:     in.SkipHeader,
				ParseHeader:    in.ParseHeader,
				Compression:    in.Compression,
			}
			result, err := fileformat.PreviewStageFile(ctx, client, in.StagePath, cfg)
			if err != nil {
				return nil, nil, err
			}
			if result.Error != "" {
				return textResult(result.Error), nil, nil
			}
			return jsonResult(result), nil, nil
		})
	}

	// ── Pipe tools (always-on) ────────────────────────────────────────────

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_pipe_status",
		Description: "Return the current status of a Snowpipe (execution state, pending " +
			"file count, notification channel) via SYSTEM$PIPE_STATUS.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in pipeStatusInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		pipeFqn := snowflake.QuoteIdent(in.Database) + "." + snowflake.QuoteIdent(in.Schema) + "." + snowflake.QuoteIdent(in.Name)
		sql := fmt.Sprintf("SELECT SYSTEM$PIPE_STATUS('%s')", snowflake.EscapeStringLit(pipeFqn))
		result, err := client.Execute(ctx, sql)
		if err != nil {
			return nil, nil, err
		}
		if result == nil || len(result.Rows) == 0 || len(result.Rows[0]) == 0 || result.Rows[0][0] == nil {
			return textResult("{}"), nil, nil
		}
		raw := fmt.Sprint(result.Rows[0][0])
		// Pretty-print the JSON for readability.
		var parsed json.RawMessage
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			pretty, err := json.MarshalIndent(parsed, "", "  ")
			if err == nil {
				return textResult(string(pretty)), nil, nil
			}
		}
		return textResult(raw), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_pipe_copy_history",
		Description: "Return copy history for a Snowpipe from INFORMATION_SCHEMA. " +
			"Optionally filter by start_time (ISO-8601, default 24h ago), " +
			"status (LOADED, LOAD_FAILED, PARTIALLY_LOADED), or file_name substring.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in pipeCopyHistoryInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		result, err := pipe.GetCopyHistory(ctx, client, in.Database, in.Schema, in.Name, in.StartTime, in.Status, in.FileName)
		if err != nil {
			return nil, nil, err
		}
		if result != nil && len(result.Rows) > maxMCPResultRows {
			result.Rows = result.Rows[:maxMCPResultRows]
			result.Truncated = true
		}
		return jsonResult(result), nil, nil
	})

	// ── open_task_graph (emit-gated) ──────────────────────────────────────

	if emit != nil {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name: "open_task_graph",
			Description: "Open the task graph visualization in Thaw for a root task. " +
				"The user sees the interactive DAG in the app.",
		}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in openTaskGraphInput) (*mcpsdk.CallToolResult, any, error) {
			if in.Database == "" {
				return nil, nil, fmt.Errorf("database is required")
			}
			if in.Schema == "" {
				return nil, nil, fmt.Errorf("schema is required")
			}
			if in.Task == "" {
				return nil, nil, fmt.Errorf("task is required")
			}

			var emitFailed bool
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.L.Error("mcp open_task_graph: emit panicked", "err", r)
						emitFailed = true
					}
				}()
				emit("mcp:open-task-graph", OpenTaskGraphPayload{
					Database: in.Database,
					Schema:   in.Schema,
					Task:     in.Task,
				})
			}()
			if emitFailed {
				return textResult("Failed to open task graph: internal error"), nil, nil
			}
			return textResult("Task graph opened successfully."), nil, nil
		})
	}
}
