// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package dbt

import (
	"context"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// CreateProject scaffolds a new dbt project pre-wired to the active Snowflake
// connection. It fetches live session info, discovers tables/views per schema,
// optionally inlines view bodies (rewriting object references into dbt
// source()/ref() macros), and delegates the file generation to Generate.
//
// schemasMap maps database names to the list of schemas to include as sources.
func CreateProject(
	ctx context.Context,
	client *snowflake.Client,
	req CreateRequest,
	schemasMap map[string][]string,
) (*CreateResult, error) {
	// ── Fetch live session info ────────────────────────────────────────────────
	qr, err := client.Execute(ctx,
		`SELECT CURRENT_ACCOUNT(), CURRENT_USER(), CURRENT_ROLE(), CURRENT_WAREHOUSE(), CURRENT_DATABASE(), CURRENT_SCHEMA()`)
	if err != nil {
		return nil, fmt.Errorf("fetch session info: %w", err)
	}

	var sess SessionInfo
	if len(qr.Rows) > 0 && len(qr.Rows[0]) >= 6 {
		row := qr.Rows[0]
		sess.Account = strings.ToLower(fmt.Sprint(row[0]))
		sess.User = fmt.Sprint(row[1])
		sess.Role = fmt.Sprint(row[2])
		sess.Warehouse = fmt.Sprint(row[3])
		sess.Database = fmt.Sprint(row[4])
		sess.Schema = fmt.Sprint(row[5])
	}

	// ── Discover objects per schema ───────────────────────────────────────────
	var schemaObjects []SchemaObjects

	for db, schemas := range schemasMap {
		for _, schema := range schemas {
			if strings.ToUpper(schema) == "INFORMATION_SCHEMA" {
				schemaObjects = append(schemaObjects, SchemaObjects{
					DB:       db,
					Schema:   schema,
					IsSystem: true,
				})
				continue
			}

			objs, err := client.ListObjects(ctx, db, schema)
			if err != nil {
				schemaObjects = append(schemaObjects, SchemaObjects{
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

			so := SchemaObjects{
				DB:     db,
				Schema: schema,
				Tables: tables,
				Views:  views,
			}

			if req.InlineViewDefs && len(views) > 0 {
				viewDefs := make(map[string]string, len(views))
				for _, v := range views {
					ddlText, err := client.GetObjectDDL(ctx, db, schema, "VIEW", v, "")
					if err != nil {
						continue
					}
					if body := snowflake.ExtractDDLBody(ddlText, "VIEW"); body != "" {
						viewDefs[v] = body
					}
				}
				so.ViewDefs = viewDefs
			}

			schemaObjects = append(schemaObjects, so)
		}
	}

	// ── Rewrite object references in inlined view bodies ──────────────────────
	if req.InlineViewDefs {
		type objInfo struct {
			kind   string
			db     string
			schema string
			name   string
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
							return ""
						}
						sName := SourceName(info.db, info.schema)
						if info.kind == "table" {
							return fmt.Sprintf("{{ source('%s', '%s') }}", sName, info.name)
						}
						modelName := StagingModelName(info.db, info.schema, info.name, multiScope)
						return fmt.Sprintf("{{ ref('%s') }}", modelName)
					},
				)
				newDefs[viewName] = rewritten
			}
			schemaObjects[i].ViewDefs = newDefs
		}
	}

	return Generate(req, sess, schemaObjects)
}
