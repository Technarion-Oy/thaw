// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// thaw:file-domain: Core IPC & App Lifecycle
package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"thaw/internal/apperrors"
	"thaw/internal/config"
	"thaw/internal/ddl"
	"thaw/internal/filesystem"
	"thaw/internal/snowflake"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// AccountExportResult reports the outcome of exporting account-level objects.
type AccountExportResult struct {
	Roles      int      `json:"roles"`
	Warehouses int      `json:"warehouses"`
	Errors     []string `json:"errors,omitempty"`
}

// ExportAccountObjectsDDL exports all accessible roles and warehouses as SQL files
// under <outputDir>/_account/roles/ and <outputDir>/_account/warehouses/.
func (a *App) ExportAccountObjectsDDL(outputDir string) (AccountExportResult, error) {
	if a.client == nil {
		return AccountExportResult{}, apperrors.ErrNotConnected
	}

	var result AccountExportResult

	// ── Roles ────────────────────────────────────────────────────────────────
	roles, err := a.client.ListRoles(a.ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list roles: %v", err))
	} else {
		for _, role := range roles {
			src, err := a.client.GetRoleDDL(a.ctx, role)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("role %s: %v", role, err))
				continue
			}
			path := filepath.Join(outputDir, "_account", "roles", sanitizeAccountFilename(role)+".sql")
			if writeErr := filesystem.WriteFile(path, strings.TrimRight(src, "\n")+"\n"); writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("write role %s: %v", role, writeErr))
				continue
			}
			result.Roles++
		}
	}

	// ── Warehouses ───────────────────────────────────────────────────────────
	warehouses, err := a.client.ListWarehouses(a.ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list warehouses: %v", err))
	} else {
		for _, wh := range warehouses {
			src, err := a.client.GetWarehouseDDL(a.ctx, wh)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("warehouse %s: %v", wh, err))
				continue
			}
			path := filepath.Join(outputDir, "_account", "warehouses", sanitizeAccountFilename(wh)+".sql")
			if writeErr := filesystem.WriteFile(path, strings.TrimRight(src, "\n")+"\n"); writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("write warehouse %s: %v", wh, writeErr))
				continue
			}
			result.Warehouses++
		}
	}

	return result, nil
}

// sanitizeAccountFilename replaces characters that are invalid in file names.
func sanitizeAccountFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
}

// GetERDiagramData fetches column metadata, primary keys, and foreign keys for
// every table in the database and returns the data needed to render an Entity
// Relationship Diagram on the frontend.
func (a *App) GetERDiagramData(database string) (snowflake.ERDiagramData, error) {
	if a.client == nil {
		return snowflake.ERDiagramData{}, apperrors.ErrNotConnected
	}
	return a.client.GetERDiagramData(a.ctx, database)
}

// ddlProgressEvent is the Wails event name emitted during export.
const ddlProgressEvent = "ddl:progress"

// DDLProgressPayload is the structure emitted with each ddl:progress event.
type DDLProgressPayload struct {
	Done   int              `json:"done"`
	Total  int              `json:"total"`
	Result ddl.ExportResult `json:"result"`
}

// ExportDatabaseDDL fetches the complete DDL for a single database via
// GET_DDL, splits it into one file per object, and writes the files under
// outputDir/<database>/.
//
// Progress is also emitted as a "ddl:progress" Wails event so the frontend
// can update a progress indicator in real time.
func (a *App) ExportDatabaseDDL(database, outputDir string) (ddl.ExportResult, error) {
	if a.client == nil {
		return ddl.ExportResult{}, apperrors.ErrNotConnected
	}

	// Temporarily scale up pool for parallel DDL fetching.
	a.client.SetPoolLimits(32, 32)
	defer a.client.SetPoolLimits(snowflake.DefaultMaxOpenConns, snowflake.DefaultMaxIdleConns)

	ctx, cancel := context.WithCancel(a.ctx)
	a.exportCancelFunc = cancel
	defer func() {
		cancel()
		a.exportCancelFunc = nil
	}()

	var pathTemplate string
	if cfg, err := config.Load(); err == nil {
		pathTemplate = cfg.Git.ExportPathTemplate
	}
	opts := ddl.ExportOptions{OutputDir: outputDir, PathTemplate: pathTemplate}

	var result ddl.ExportResult
	ddl.ExportDatabases(
		ctx,
		[]string{database},
		a.client.GetCompleteDatabaseDDL,
		opts,
		func(done, total int, res ddl.ExportResult) {
			result = res
			wailsruntime.EventsEmit(a.ctx, ddlProgressEvent, DDLProgressPayload{
				Done:   done,
				Total:  total,
				Result: res,
			})
		},
	)

	return result, nil
}

// ListExportableDatabases returns the names of all databases that can be
// exported (own databases; shared/imported databases such as
// SNOWFLAKE_SAMPLE_DATA are excluded).  The frontend uses this list to
// populate the database-selection checkboxes in the Export DDL panel.
func (a *App) ListExportableDatabases() ([]string, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListExportableDatabases(a.ctx)
}

// ExportAllDatabasesDDL exports DDL for the given databases in parallel.
// When databases is nil or empty every exportable database owned by the
// account is exported (same behavior as before database selection was added).
//
// Progress events ("ddl:progress") are emitted after each database completes,
// allowing the frontend to show a live progress bar.
func (a *App) ExportAllDatabasesDDL(outputDir string, databases []string) ([]ddl.ExportResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}

	// Temporarily scale up pool for parallel DDL fetching.
	a.client.SetPoolLimits(32, 32)
	defer a.client.SetPoolLimits(snowflake.DefaultMaxOpenConns, snowflake.DefaultMaxIdleConns)

	if len(databases) == 0 {
		var err error
		databases, err = a.client.ListExportableDatabases(a.ctx)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.exportCancelFunc = cancel
	defer func() {
		cancel()
		a.exportCancelFunc = nil
	}()

	var pathTemplate string
	if cfg, err := config.Load(); err == nil {
		pathTemplate = cfg.Git.ExportPathTemplate
	}
	opts := ddl.ExportOptions{OutputDir: outputDir, PathTemplate: pathTemplate}

	results := ddl.ExportDatabases(
		ctx,
		databases,
		a.client.GetCompleteDatabaseDDL,
		opts,
		func(done, total int, res ddl.ExportResult) {
			wailsruntime.EventsEmit(a.ctx, ddlProgressEvent, DDLProgressPayload{
				Done:   done,
				Total:  total,
				Result: res,
			})
		},
	)

	return results, nil
}

// GetSchemaCrossDeps returns the unique (database, schema) pairs referenced
// by views in the given schema that fall outside that schema.
func (a *App) GetSchemaCrossDeps(db, schema string) ([]snowflake.SchemaRef, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.GetSchemaCrossDeps(a.ctx, db, schema)
}

// GetDatabaseCrossDeps analyses all given schemas in db sequentially.
func (a *App) GetDatabaseCrossDeps(db string, schemas []string) ([]snowflake.SchemaRef, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.GetDatabaseCrossDeps(a.ctx, db, schemas)
}
