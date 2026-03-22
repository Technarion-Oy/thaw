// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package main

import (
	"fmt"
	"strings"

	"thaw/internal/dbt"
	"thaw/internal/snowflake"
)

// CreateDbtProject scaffolds a new dbt project pre-wired to the active
// Snowflake connection.
//
// req describes the project name, output directory and optional profile name.
// schemasMap maps database names to the list of schemas to include as sources.
func (a *App) CreateDbtProject(req dbt.CreateRequest, schemasMap map[string][]string) (*dbt.CreateResult, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}

	// ── Fetch live session info ────────────────────────────────────────────────
	qr, err := a.client.Execute(a.ctx,
		`SELECT CURRENT_ACCOUNT(), CURRENT_USER(), CURRENT_ROLE(), CURRENT_WAREHOUSE(), CURRENT_DATABASE(), CURRENT_SCHEMA()`)
	if err != nil {
		return nil, fmt.Errorf("fetch session info: %w", err)
	}

	var session dbt.SessionInfo
	if len(qr.Rows) > 0 && len(qr.Rows[0]) >= 6 {
		row := qr.Rows[0]
		session.Account   = strings.ToLower(fmt.Sprint(row[0]))
		session.User      = fmt.Sprint(row[1])
		session.Role      = fmt.Sprint(row[2])
		session.Warehouse = fmt.Sprint(row[3])
		session.Database  = fmt.Sprint(row[4])
		session.Schema    = fmt.Sprint(row[5])
	}

	// ── Discover objects per schema ───────────────────────────────────────────
	var schemaObjects []dbt.SchemaObjects

	for db, schemas := range schemasMap {
		for _, schema := range schemas {
			// INFORMATION_SCHEMA is a virtual Snowflake system schema; listing
			// its objects is not meaningful for dbt staging models.  Add it as
			// a source entry (so _sources.yml is populated) but skip the
			// ListObjects call and don't generate stub models.
			if strings.ToUpper(schema) == "INFORMATION_SCHEMA" {
				schemaObjects = append(schemaObjects, dbt.SchemaObjects{
					DB:       db,
					Schema:   schema,
					IsSystem: true,
				})
				continue
			}

			objs, err := a.client.ListObjects(a.ctx, db, schema)
			if err != nil {
				// Non-fatal: record a warning instead of aborting the whole project.
				schemaObjects = append(schemaObjects, dbt.SchemaObjects{
					DB:     db,
					Schema: schema,
				})
				continue
			}

			var tables, views []string
			for _, o := range objs {
				switch strings.ToUpper(o.Kind) {
				case "TABLE":
					tables = append(tables, o.Name)
				case "VIEW":
					views = append(views, o.Name)
				}
			}

			so := dbt.SchemaObjects{
				DB:     db,
				Schema: schema,
				Tables: tables,
				Views:  views,
			}

			// When the user opted to inline view SQL, fetch the DDL for each
			// view and extract its SELECT body.  Sequential per-view GET_DDL
			// calls (no goroutines) to avoid exhausting the connection pool.
			// Views are typically far fewer than tables, and this path is
			// opt-in only.
			if req.InlineViewDefs && len(views) > 0 {
				viewDefs := make(map[string]string, len(views))
				for _, v := range views {
					ddl, err := a.client.GetObjectDDL(a.ctx, db, schema, "VIEW", v, "")
					if err != nil {
						continue // non-fatal: fall back to pass-through stub
					}
					if body := snowflake.ExtractDDLBody(ddl, "VIEW"); body != "" {
						viewDefs[v] = body
					}
				}
				so.ViewDefs = viewDefs
			}

			schemaObjects = append(schemaObjects, so)
		}
	}

	// ── Rewrite object references in inlined view bodies ──────────────────────
	// Now that all schema objects are known, do a second pass to replace raw
	// Snowflake three-part identifiers (DB.SCHEMA.TABLE) with the correct dbt
	// Jinja calls:
	//   • tables  → {{ source('source_name', 'TABLE') }}
	//   • views   → {{ ref('stg_model_name') }}
	//   • unknown → left unchanged (external reference not in selected schemas)
	if req.InlineViewDefs {
		// Build a fast lookup: UPPER(db + \x00 + schema + \x00 + name) → objKind.
		// Store the original name (as returned by ListObjects) alongside.
		type objInfo struct {
			kind   string // "table" or "view"
			db     string
			schema string
			name   string // original case
		}
		objLookup := make(map[string]objInfo)
		for _, so := range schemaObjects {
			for _, t := range so.Tables {
				objLookup[strings.ToUpper(so.DB+"\x00"+so.Schema+"\x00"+t)] = objInfo{"table", so.DB, so.Schema, t}
			}
			for _, v := range so.Views {
				objLookup[strings.ToUpper(so.DB+"\x00"+so.Schema+"\x00"+v)] = objInfo{"view", so.DB, so.Schema, v}
			}
		}

		// multiScope mirrors the flag used by the generator to decide whether
		// to prefix stub filenames with db_schema_.
		multiScope := len(schemaObjects) > 1

		for i := range schemaObjects {
			if len(schemaObjects[i].ViewDefs) == 0 {
				continue
			}
			newDefs := make(map[string]string, len(schemaObjects[i].ViewDefs))
			for viewName, body := range schemaObjects[i].ViewDefs {
				rewritten := snowflake.RewriteSQLReferences(
					body,
					schemaObjects[i].DB,
					schemaObjects[i].Schema,
					func(db, schema, name string) string {
						key := strings.ToUpper(db + "\x00" + schema + "\x00" + name)
						info, ok := objLookup[key]
						if !ok {
							return "" // not in selected schemas — leave as-is
						}
						sName := dbt.SourceName(info.db, info.schema)
						if info.kind == "table" {
							return fmt.Sprintf("{{ source('%s', '%s') }}", sName, info.name)
						}
						// view → ref to the generated staging model
						modelName := dbt.StagingModelName(info.db, info.schema, info.name, multiScope)
						return fmt.Sprintf("{{ ref('%s') }}", modelName)
					},
				)
				newDefs[viewName] = rewritten
			}
			schemaObjects[i].ViewDefs = newDefs
		}
	}

	return dbt.Generate(req, session, schemaObjects)
}

// GetSchemaCrossDeps returns the unique (database, schema) pairs referenced
// by views in the given schema that fall outside that schema.  Called for
// individual schema selections in the dbt project wizard.
func (a *App) GetSchemaCrossDeps(db, schema string) ([]snowflake.SchemaRef, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.GetSchemaCrossDeps(a.ctx, db, schema)
}

// GetDatabaseCrossDeps analyses all given schemas in db sequentially in a
// single call, avoiding N concurrent IPC goroutines when "Select all" is
// clicked in the dbt project wizard.
func (a *App) GetDatabaseCrossDeps(db string, schemas []string) ([]snowflake.SchemaRef, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.GetDatabaseCrossDeps(a.ctx, db, schemas)
}
