// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package dbt provides scaffolding for new dbt projects wired to a live
// Snowflake connection.  It is a pure generation package — all Snowflake
// queries are performed by the caller before invoking Generate.
//
// thaw:domain: Snowpark & Developer Workflows
package dbt

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"thaw/internal/filesystem"
)

// CreateRequest contains the user-supplied parameters for a new dbt project.
type CreateRequest struct {
	ProjectName    string `json:"projectName"`
	OutputDir      string `json:"outputDir"`
	ProfileName    string `json:"profileName"`    // defaults to ProjectName when empty
	InlineViewDefs bool   `json:"inlineViewDefs"` // embed actual SELECT body in view stubs
	DatabaseVars   bool   `json:"databaseVars"`   // declare vars: for each DB in dbt_project.yml
}

// SessionInfo carries the live Snowflake session values used to populate
// profiles.yml.
type SessionInfo struct {
	Account   string
	User      string
	Role      string
	Warehouse string
	Database  string
	Schema    string
}

// SchemaObjects holds the tables and views discovered in one (database, schema)
// pair.  IsSystem marks system schemas (e.g. INFORMATION_SCHEMA) for which no
// object discovery was performed and no staging stubs should be generated.
// ViewDefs maps view name → extracted SELECT body and is populated only when
// CreateRequest.InlineViewDefs is true.
type SchemaObjects struct {
	DB       string
	Schema   string
	Tables   []string
	Views    []string
	ViewDefs map[string]string // view name → SELECT body (nil when not inlining)
	IsSystem bool
}

// CreateResult is the value returned by Generate on success.
type CreateResult struct {
	ProjectDir   string   `json:"projectDir"`
	FilesCreated []string `json:"filesCreated"`
	Warnings     []string `json:"warnings"`
}

// Generate writes the full dbt project file tree under
// <req.OutputDir>/<req.ProjectName>/ and returns the list of created files.
func Generate(req CreateRequest, session SessionInfo, objects []SchemaObjects) (*CreateResult, error) {
	profileName := req.ProfileName
	if profileName == "" {
		profileName = req.ProjectName
	}

	projectDir := filepath.Join(req.OutputDir, req.ProjectName)

	var filesCreated []string
	var warnings []string

	write := func(relPath, content string) error {
		abs := filepath.Join(projectDir, relPath)
		if err := filesystem.WriteFile(abs, content); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
		filesCreated = append(filesCreated, relPath)
		return nil
	}

	// ── Pre-scan: build dbVar maps for DatabaseVars mode ─────────────────────
	// dbVarName  maps UPPER(db) → dbt var name  (e.g. "MYDB" → "db_mydb").
	// dbVarOrig  maps UPPER(db) → original DB string as received from Snowflake.
	// Only databases that will produce a source entry are included.
	dbVarName := make(map[string]string)
	dbVarOrig := make(map[string]string)
	if req.DatabaseVars {
		for _, so := range objects {
			upper := strings.ToUpper(so.DB)
			if _, ok := dbVarName[upper]; ok {
				continue
			}
			// System schemas always get a source entry; regular schemas need objects.
			if !so.IsSystem && len(so.Tables)+len(so.Views) == 0 {
				continue
			}
			dbVarName[upper] = "db_" + strings.ToLower(so.DB)
			dbVarOrig[upper] = so.DB
		}
	}

	// Helper: emit the database: value, using a var reference when requested.
	dbField := func(db string) string {
		if req.DatabaseVars {
			upper := strings.ToUpper(db)
			if varName, ok := dbVarName[upper]; ok {
				return fmt.Sprintf("\"{{ var('%s', '%s') }}\"", varName, dbVarOrig[upper])
			}
		}
		return db
	}

	// ── dbt_project.yml ───────────────────────────────────────────────────────
	dbtProject := fmt.Sprintf(`name: '%s'
version: '1.0.0'
config-version: 2
profile: '%s'
model-paths: ["models"]
seed-paths: ["seeds"]
macro-paths: ["macros"]
target-path: "target"
clean-targets: ["target", "dbt_packages"]
models:
  %s:
    staging:
      +materialized: view
    marts:
      +materialized: table
`, req.ProjectName, profileName, req.ProjectName)

	if req.DatabaseVars && len(dbVarName) > 0 {
		// Collect and sort upper-case DB names for deterministic output.
		dbUppers := make([]string, 0, len(dbVarName))
		for upper := range dbVarName {
			dbUppers = append(dbUppers, upper)
		}
		sort.Strings(dbUppers)

		var varsBlock strings.Builder
		varsBlock.WriteString("vars:\n")
		for _, upper := range dbUppers {
			fmt.Fprintf(&varsBlock, "  %s: %s\n", dbVarName[upper], dbVarOrig[upper])
		}
		dbtProject += varsBlock.String()
	}

	if err := write("dbt_project.yml", dbtProject); err != nil {
		return nil, err
	}

	// ── profiles.yml ──────────────────────────────────────────────────────────
	profiles := fmt.Sprintf(`%s:
  target: dev
  outputs:
    dev:
      type: snowflake
      account: %s
      user: %s
      # authenticator: snowflake
      # password: <your_password>
      role: %s
      warehouse: %s
      database: %s
      schema: %s
      threads: 4
      client_session_keep_alive: false
`, profileName, session.Account, session.User, session.Role, session.Warehouse, session.Database, session.Schema)

	if err := write("profiles.yml", profiles); err != nil {
		return nil, err
	}

	// ── .gitkeep stubs ────────────────────────────────────────────────────────
	for _, p := range []string{
		filepath.Join("seeds", ".gitkeep"),
		filepath.Join("macros", ".gitkeep"),
		filepath.Join("models", "marts", ".gitkeep"),
	} {
		if err := write(p, ""); err != nil {
			return nil, err
		}
	}

	// ── models/staging/_sources.yml + stub models ─────────────────────────────

	// Determine whether we have multiple real (db, schema) pairs — used to build
	// unique file prefixes.  System and empty schemas produce no staging stubs,
	// so they must not inflate the count (a single data schema alongside
	// INFORMATION_SCHEMA should still get plain stg_<table> names).
	multiScope := multiScopeFor(objects)

	// Build _sources.yml
	var sourcesBuilder strings.Builder
	sourcesBuilder.WriteString("version: 2\nsources:\n")

	for _, so := range objects {
		sName := sourceName(so.DB, so.Schema)

		// System schemas (e.g. INFORMATION_SCHEMA) are written as source
		// entries so models can reference them via {{ source(...) }}, but no
		// object listing was performed and no staging stubs are generated.
		if so.IsSystem {
			fmt.Fprintf(&sourcesBuilder, "  - name: %s\n", sName)
			fmt.Fprintf(&sourcesBuilder, "    database: %s\n", dbField(so.DB))
			fmt.Fprintf(&sourcesBuilder, "    schema: %s\n", so.Schema)
			sourcesBuilder.WriteString("    description: \"System schema — add individual table entries manually as needed\"\n")
			sourcesBuilder.WriteString("    tables: []\n")
			continue
		}

		if len(so.Tables)+len(so.Views) == 0 {
			warnings = append(warnings, fmt.Sprintf("no tables or views found in %s.%s — skipped", so.DB, so.Schema))
			continue
		}

		fmt.Fprintf(&sourcesBuilder, "  - name: %s\n", sName)
		fmt.Fprintf(&sourcesBuilder, "    database: %s\n", dbField(so.DB))
		fmt.Fprintf(&sourcesBuilder, "    schema: %s\n", so.Schema)
		fmt.Fprintf(&sourcesBuilder, "    description: \"Source tables from %s.%s\"\n", so.DB, so.Schema)
		sourcesBuilder.WriteString("    tables:\n")

		allNames := make([]string, 0, len(so.Tables)+len(so.Views))
		allNames = append(allNames, so.Tables...)
		allNames = append(allNames, so.Views...)
		for _, t := range allNames {
			fmt.Fprintf(&sourcesBuilder, "      - name: %s\n", t)
			sourcesBuilder.WriteString("        description: \"\"\n")
		}

		// Write one stub model per table/view
		for _, t := range allNames {
			stubPath := stagingModelPath(so.DB, so.Schema, t, multiScope)
			stub := stagingModelSQL(sName, t, so.ViewDefs[t])
			if err := write(stubPath, stub); err != nil {
				return nil, err
			}
		}
	}

	if err := write(filepath.Join("models", "staging", "_sources.yml"), sourcesBuilder.String()); err != nil {
		return nil, err
	}

	return &CreateResult{
		ProjectDir:   projectDir,
		FilesCreated: filesCreated,
		Warnings:     warnings,
	}, nil
}

// SourceName returns the lower-case dbt source name for the given (db, schema)
// pair, e.g. "mydb_public".  Exported so IPC callers can build consistent
// {{ source('...', '...') }} references without duplicating the convention.
func SourceName(db, schema string) string {
	return scopeName(db, schema)
}

// sourceName is the unexported alias kept for internal use within this file.
func sourceName(db, schema string) string { return SourceName(db, schema) }

// scopeName builds the lower-case "db_schema" scope name shared by source names
// and (multi-scope) staging-model names.
//
// A single "_" separator is ambiguous when an identifier itself contains an
// underscore: "A_B"."C" and "A"."B_C" would both join to "a_b_c", colliding
// distinct Snowflake scopes.  To make the join injective, each identifier's own
// underscores are doubled (escapeIdent), so the single un-doubled "_" is always
// the real db/schema boundary and the doubled runs are interior.  Identifiers
// without underscores are unchanged, so the common "mydb_public" form is
// preserved.  References built via SourceName/StagingModelName stay consistent
// because they call this same helper.
func scopeName(db, schema string) string {
	return escapeIdent(db) + "_" + escapeIdent(schema)
}

// escapeIdent lower-cases an identifier and doubles every "_" it contains, so a
// single "_" can be used as an unambiguous component separator.
func escapeIdent(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "_", "__")
}

// multiScopeFor reports whether staging model names need a db_schema prefix to
// stay unique.  Only schemas that actually produce staging stubs count — system
// schemas (no object listing) and empty schemas (no tables/views) are excluded,
// so filenames don't depend on which non-data schemas happened to be discovered.
func multiScopeFor(objects []SchemaObjects) bool {
	n := 0
	for _, so := range objects {
		if so.IsSystem || len(so.Tables)+len(so.Views) == 0 {
			continue
		}
		n++
	}
	return n > 1
}

// StagingModelName returns the dbt model name (filename without the .sql
// extension) for a staging model.  Exported so IPC callers can build
// consistent {{ ref('...') }} references that match the generated filenames.
func StagingModelName(db, schema, table string, multiScope bool) string {
	if multiScope {
		return "stg_" + scopeName(db, schema) + "_" + strings.ToLower(table)
	}
	return fmt.Sprintf("stg_%s", strings.ToLower(table))
}

// stagingModelPath returns the relative path for a staging model file.
// When multiScope is true (multiple db/schema pairs) a db_schema_ prefix is
// added to avoid collisions.
func stagingModelPath(db, schema, table string, multiScope bool) string {
	return filepath.Join("models", "staging", StagingModelName(db, schema, table, multiScope)+".sql")
}

// blockComment matches /* ... */ comments, including across newlines.
var blockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

// hasExecutableSQL reports whether s contains anything other than whitespace and
// SQL comments.  Used to decide between inlining a view body and emitting the
// generic source() stub — a comment-only body (e.g. "-- definition unavailable")
// would otherwise be inlined and fail dbt compile.
// ponytail: strips -- naively, so a "--" inside a string literal ends the line
// early; harmless here — a body with real SQL still tests non-empty.
func hasExecutableSQL(s string) bool {
	s = blockComment.ReplaceAllString(s, "")
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

// stagingModelSQL returns the body of a staging model stub.
// When sqlBody carries actual SQL the view body is inlined instead of a generic
// pass-through {{ source(...) }} reference.  A body that is blank or only
// comments would compile to nothing, so it falls back to the stub.
func stagingModelSQL(src, table, sqlBody string) string {
	if hasExecutableSQL(sqlBody) {
		return fmt.Sprintf(
			"-- Generated by Thaw — view SQL inlined from Snowflake\n"+
				"-- TODO: replace Snowflake table references with {{ source('...', '...') }} or {{ ref('...') }} calls\n"+
				"%s\n",
			sqlBody,
		)
	}
	return fmt.Sprintf(`-- Generated by Thaw — dbt stub for {{ source('%s', '%s') }}
with source as (
    select * from {{ source('%s', '%s') }}
),
renamed as (
    select
        *
        -- TODO: select and rename individual columns
    from source
)
select * from renamed
`, src, table, src, table)
}
