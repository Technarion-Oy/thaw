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

	"thaw/internal/querylog"
)

// Feature names attributed to internal queries in the query log. Each IPC entry
// point tags its context with the feature that issued the query (via fctx) so
// the Query Log pane can show not just *what* SQL Thaw sent but *why*. The value
// is surfaced verbatim as the "Feature" column/filter in the frontend, so keep
// these strings human-readable.
const (
	FeatureSQLEditor     = "SQL Editor"     // user-run editor queries + editor helpers
	FeatureObjectBrowser = "Object Browser" // object listing, DDL, dependencies, undrop
	FeatureObjectEditor  = "Object Editor"  // create/alter/drop of individual object types
	FeatureSessionSetup  = "Session Setup"  // roles, warehouses, USE, session params/vars
	FeatureDDLExport     = "DDL Export"     // Export DDL panel
	FeatureERDiagram     = "ER Diagram"     // Entity-Relationship diagram / designer
	FeatureWarehouses    = "Warehouses"     // warehouse administration
	FeatureTasks         = "Tasks"          // task graph management
	FeatureStages        = "Stages"         // stage file browser / PUT / GET
	FeatureBackup        = "Backup"         // backup sets, policies, restore
	FeatureIntegrations  = "Integrations"   // integrations, secrets, external volumes
	FeatureUsersRoles    = "Users & Roles"  // user administration
	FeatureTags          = "Tags"           // object tagging
	FeatureDbtProjects   = "dbt Projects"   // dbt project objects
	FeatureNotebooks     = "Notebooks"      // native Snowflake notebooks
)

// fctx returns the app-wide context annotated with the Thaw feature that issued
// the query. Pass its result to client/domain calls instead of a.ctx so the
// query-log OnQuery hook can attribute the resulting internal queries to a
// feature. It only sets a context value, so it is cheap to call per request.
func (a *App) fctx(feature string) context.Context {
	return querylog.WithFeature(a.ctx, feature)
}
