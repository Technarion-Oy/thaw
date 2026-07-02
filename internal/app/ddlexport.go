// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"context"
	"thaw/internal/apperrors"
	"thaw/internal/config"
	"thaw/internal/ddl"
	"thaw/internal/snowflake"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportAccountObjectsDDL exports all accessible roles and warehouses as SQL files
// under <outputDir>/_account/roles/ and <outputDir>/_account/warehouses/.
func (a *App) ExportAccountObjectsDDL(outputDir string) (ddl.AccountExportResult, error) {
	if a.client == nil {
		return ddl.AccountExportResult{}, apperrors.ErrNotConnected
	}
	return ddl.ExportAccountObjects(a.ctx, a.client, outputDir)
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

// DDLExportOptions carries the per-export choices made in the frontend's
// pre-export options dialog. All fields are optional; the zero value
// reproduces the historical behavior (all types, all schemas, overwrite,
// session warehouse, configured path template).
type DDLExportOptions struct {
	// ObjectTypes restricts export to these kinds (ddl.Kind strings such as
	// "TABLE", "FILE FORMAT"). Empty = all. Post-fetch filter only.
	ObjectTypes []string `json:"objectTypes"`
	// Schemas restricts export to these schema names (case-insensitive).
	// Empty = all. Post-fetch filter only.
	Schemas []string `json:"schemas"`
	// SkipExisting leaves already-existing files untouched instead of
	// overwriting them.
	SkipExisting bool `json:"skipExisting"`
	// Warehouse runs the export on this warehouse (USE WAREHOUSE before the
	// GET_DDL calls; the previous warehouse is restored afterwards).
	// Empty = session warehouse.
	Warehouse string `json:"warehouse"`
	// PathTemplate overrides the configured export path template for this
	// export only. Empty = configured template.
	PathTemplate string `json:"pathTemplate"`
}

// ExportAllDatabasesDDL exports DDL for the given databases in parallel.
// When databases is nil or empty every exportable database owned by the
// account is exported (same behavior as before database selection was added).
//
// Progress events ("ddl:progress") are emitted after each database completes,
// allowing the frontend to show a live progress bar.
func (a *App) ExportAllDatabasesDDL(outputDir string, databases []string, options DDLExportOptions) ([]ddl.ExportResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}

	// Switch to the requested warehouse for the duration of the export.
	// UseWarehouse updates the session connector, so every pooled connection
	// the parallel GET_DDL fetches use inherits it.
	if options.Warehouse != "" {
		prev, err := a.client.CurrentWarehouse(a.ctx)
		if err != nil {
			return nil, err
		}
		if prev != options.Warehouse {
			if err := a.client.UseWarehouse(a.ctx, options.Warehouse); err != nil {
				return nil, err
			}
			if prev != "" {
				defer a.client.UseWarehouse(a.ctx, prev) //nolint:errcheck // best-effort restore
			}
		}
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

	pathTemplate := options.PathTemplate
	if pathTemplate == "" {
		if cfg, err := config.Load(); err == nil {
			pathTemplate = cfg.Git.ExportPathTemplate
		}
	}
	kinds := make([]ddl.Kind, len(options.ObjectTypes))
	for i, t := range options.ObjectTypes {
		kinds[i] = ddl.Kind(t)
	}
	opts := ddl.ExportOptions{
		OutputDir:    outputDir,
		PathTemplate: pathTemplate,
		ObjectTypes:  kinds,
		Schemas:      options.Schemas,
		SkipExisting: options.SkipExisting,
	}

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
