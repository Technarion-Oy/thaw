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

	// Resolve the final, project-unique source and staging-model names up front.
	// The readable base names are not injective on their own (see SourceName), so
	// these maps break any collision deterministically; create.go builds the same
	// maps to keep {{ source(...) }} / {{ ref(...) }} references consistent.
	srcNames := SourceNames(objects)
	stagingNames := StagingNames(objects, srcNames, multiScope)

	// Build _sources.yml
	var sourcesBuilder strings.Builder
	sourcesBuilder.WriteString("version: 2\nsources:\n")

	for _, so := range objects {
		sName := srcNames[scopeKey(so.DB, so.Schema)]

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
			stem := stagingNames[tableKey(so.DB, so.Schema, t)]
			stubPath := filepath.Join("models", "staging", stem+".sql")
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

// SourceName returns the readable dbt source-name convention for a (db, schema)
// pair: the two identifiers lower-cased and joined with "_", e.g. "mydb_public".
//
// This base name is NOT unique on its own.  dbt source and model names may only
// contain [A-Za-z0-9_], so "_" is the only available separator — and because
// Snowflake identifiers may themselves contain "_", two distinct scopes can map
// to the same string ("A_B"."C" and "A"."B_C" both → "a_b_c").  No "_"-based
// scheme can avoid this, so uniqueness is enforced at the project level by
// SourceNames, which appends a numeric suffix to any collision.  Callers needing
// the final name must use that map, not this function in isolation.
func SourceName(db, schema string) string {
	return strings.ToLower(db) + "_" + strings.ToLower(schema)
}

// StagingModelName returns the readable staging-model name convention (filename
// without the .sql extension): "stg_<table>" for a single data scope, or
// "stg_<source>_<table>" when multiple scopes are present.  Like SourceName this
// is the pre-deduplication base name; StagingNames resolves collisions.
func StagingModelName(db, schema, table string, multiScope bool) string {
	return stagingBase(SourceName(db, schema), table, multiScope)
}

// stagingBase builds the readable staging-model name from an (already-resolved)
// source name and a table.  Multi-scope names embed the source name so a model
// and its source stay in sync.
func stagingBase(srcName, table string, multiScope bool) string {
	if multiScope {
		return "stg_" + srcName + "_" + strings.ToLower(table)
	}
	return "stg_" + strings.ToLower(table)
}

// stagingModelPath returns the relative path for a staging model file from its
// resolved model-name stem.
func stagingModelPath(db, schema, table string, multiScope bool) string {
	return filepath.Join("models", "staging", StagingModelName(db, schema, table, multiScope)+".sql")
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

// scopeKey / tableKey are case-insensitive map keys identifying a (db, schema)
// scope and a (db, schema, table) object respectively.  The NUL separator can't
// occur in a Snowflake identifier, so keys never collide across components.
func scopeKey(db, schema string) string {
	return strings.ToUpper(db) + "\x00" + strings.ToUpper(schema)
}

func tableKey(db, schema, table string) string {
	return scopeKey(db, schema) + "\x00" + strings.ToUpper(table)
}

// uniqueName returns base when it is unused, otherwise the first free
// base_2 / base_3 / … form, recording the chosen name in used.
func uniqueName(base string, used map[string]bool) string {
	name := base
	for i := 2; used[name]; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	used[name] = true
	return name
}

// SourceNames assigns a project-unique _sources.yml source name to every scope
// that gets a source entry — system schemas, plus regular schemas with at least
// one table or view — keyed by scopeKey.  The readable SourceName base is used
// as-is unless an earlier scope already claimed it (a collision, e.g. "A_B"."C"
// vs "A"."B_C"), in which case a _2/_3/… suffix is appended.  Iteration follows
// objects order, so Generate and create.go compute identical names.
func SourceNames(objects []SchemaObjects) map[string]string {
	used := map[string]bool{}
	out := map[string]string{}
	for _, so := range objects {
		if !so.IsSystem && len(so.Tables)+len(so.Views) == 0 {
			continue // empty schema — no source entry
		}
		k := scopeKey(so.DB, so.Schema)
		if _, ok := out[k]; ok {
			continue
		}
		out[k] = uniqueName(SourceName(so.DB, so.Schema), used)
	}
	return out
}

// StagingNames assigns a project-unique staging-model name (filename stem) to
// every (db, schema, table) that gets a stub, keyed by tableKey.  srcNames
// supplies the already-deduplicated source name used as the multi-scope prefix,
// and a separate used-set guards the model-name space (which can collide even
// when source names don't, e.g. source "a_b" + table "c_d" vs source "a_b_c" +
// table "d" both → "stg_a_b_c_d").
func StagingNames(objects []SchemaObjects, srcNames map[string]string, multiScope bool) map[string]string {
	used := map[string]bool{}
	out := map[string]string{}
	for _, so := range objects {
		if so.IsSystem || len(so.Tables)+len(so.Views) == 0 {
			continue
		}
		sName := srcNames[scopeKey(so.DB, so.Schema)]
		names := make([]string, 0, len(so.Tables)+len(so.Views))
		names = append(names, so.Tables...)
		names = append(names, so.Views...)
		for _, t := range names {
			k := tableKey(so.DB, so.Schema, t)
			if _, ok := out[k]; ok {
				continue
			}
			out[k] = uniqueName(stagingBase(sName, t, multiScope), used)
		}
	}
	return out
}

// blockComment matches /* ... */ comments, including across newlines.  The
// second pattern strips an unterminated "/*" (no closing "*/") through end of
// input — Snowflake can return truncated DDL for secured/unavailable views, and
// a body that is only an unclosed comment must still fall back to the stub.
var (
	blockComment     = regexp.MustCompile(`(?s)/\*.*?\*/`)
	openBlockComment = regexp.MustCompile(`(?s)/\*.*`)
)

// hasExecutableSQL reports whether s contains anything other than whitespace and
// SQL comments.  Used to decide between inlining a view body and emitting the
// generic source() stub — a comment-only body (e.g. "-- definition unavailable")
// would otherwise be inlined and fail dbt compile.
// ponytail: strips -- naively, so a "--" inside a string literal ends the line
// early; harmless here — a body with real SQL still tests non-empty.
func hasExecutableSQL(s string) bool {
	s = blockComment.ReplaceAllString(s, "")
	s = openBlockComment.ReplaceAllString(s, "")
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
