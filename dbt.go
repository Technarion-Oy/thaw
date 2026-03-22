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

			schemaObjects = append(schemaObjects, dbt.SchemaObjects{
				DB:     db,
				Schema: schema,
				Tables: tables,
				Views:  views,
			})
		}
	}

	return dbt.Generate(req, session, schemaObjects)
}
