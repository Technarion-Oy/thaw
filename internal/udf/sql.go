// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package udf

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// FuncArg is a single function parameter (or, in a RETURNS TABLE clause, a single
// output column): a name and a data type, e.g. {Name: "x", DataType: "NUMBER"}.
// Entries with a blank name are skipped by the builder.
type FuncArg struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
}

// FunctionConfig holds the parameters for creating a Snowflake user-defined
// FUNCTION. Only Name and Body are conceptually required; every other field is
// optional — an empty/zero value means the corresponding clause is omitted and
// Snowflake's default applies.
type FunctionConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	Secure        bool   `json:"secure"`
	IfNotExists   bool   `json:"ifNotExists"`

	Args         []FuncArg `json:"args"`         // ( <name> <dataType>, ... )
	ReturnType   string    `json:"returnType"`   // scalar return type, e.g. "NUMBER"; ignored if ReturnsTable
	ReturnsTable bool      `json:"returnsTable"` // RETURNS TABLE ( ... ) instead of a scalar type
	TableColumns []FuncArg `json:"tableColumns"` // used when ReturnsTable: RETURNS TABLE (name type, ...)

	Language     string `json:"language"`     // "SQL" | "PYTHON" | "JAVA" | "JAVASCRIPT" | "SCALA"
	NullHandling string `json:"nullHandling"` // "" | "CALLED ON NULL INPUT" | "RETURNS NULL ON NULL INPUT"
	Volatility   string `json:"volatility"`   // "" | "VOLATILE" | "IMMUTABLE"

	RuntimeVersion string   `json:"runtimeVersion"` // RUNTIME_VERSION = '<version>'
	Packages       []string `json:"packages"`       // PACKAGES = ( '<pkg>', ... )
	Imports        []string `json:"imports"`        // IMPORTS = ( '<stage_path>', ... )
	Handler        string   `json:"handler"`        // HANDLER = '<handler>'

	Comment string `json:"comment"`
	Body    string `json:"body"`
}

// BuildCreateFunctionSql constructs a CREATE FUNCTION statement from the given
// config. The argument list, return type, and body always appear (placeholders
// fill in for empty fields so the live preview stays valid-looking); the
// remaining clauses are emitted only when set, in the order Snowflake documents
// them. OR REPLACE, SECURE, and IF NOT EXISTS are independent modifiers on the
// CREATE keyword.
func BuildCreateFunctionSql(db, schema string, cfg FunctionConfig) (string, error) {
	var sb strings.Builder

	body := "FUNCTION"
	if cfg.Secure {
		body = "SECURE " + body
	}
	createClause := snowflake.CreateClause(body, cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "function_name"
	}

	fmt.Fprintf(&sb, "%s %s(%s)", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive), buildArgList(cfg.Args))

	// RETURNS — either a TABLE (col type, ...) or a scalar type.
	if cfg.ReturnsTable {
		fmt.Fprintf(&sb, "\n  RETURNS TABLE (%s)", buildArgList(cfg.TableColumns))
	} else {
		ret := strings.TrimSpace(cfg.ReturnType)
		if ret == "" {
			ret = "VARIANT"
		}
		fmt.Fprintf(&sb, "\n  RETURNS %s", ret)
	}

	// LANGUAGE — SQL is the default, so emit only for the other handler languages.
	if lang := strings.ToUpper(strings.TrimSpace(cfg.Language)); lang != "" && lang != "SQL" {
		fmt.Fprintf(&sb, "\n  LANGUAGE %s", lang)
	}

	// Null-handling / volatility precede RUNTIME_VERSION/PACKAGES/IMPORTS/HANDLER
	// in the CREATE FUNCTION grammar. This is the reverse of CREATE PROCEDURE
	// (where they follow the handler clauses) — the asymmetry is intentional and
	// matches Snowflake's two separate, order-sensitive grammars, so keep it.
	if nh := strings.TrimSpace(cfg.NullHandling); nh != "" {
		fmt.Fprintf(&sb, "\n  %s", strings.ToUpper(nh))
	}
	if vol := strings.TrimSpace(cfg.Volatility); vol != "" {
		fmt.Fprintf(&sb, "\n  %s", strings.ToUpper(vol))
	}

	// RUNTIME_VERSION / PACKAGES / IMPORTS / HANDLER apply only to the handler
	// languages (Python, Java, Scala). SQL and JavaScript UDFs carry their logic
	// inline in the body, so these clauses are skipped regardless of any stale
	// values the caller may still hold from a previous language selection.
	if snowflake.IsHandlerLanguage(cfg.Language) {
		if rv := strings.TrimSpace(cfg.RuntimeVersion); rv != "" {
			fmt.Fprintf(&sb, "\n  RUNTIME_VERSION = '%s'", snowflake.EscapeStringLit(rv))
		}
		if pkgs := buildQuotedList(cfg.Packages); pkgs != "" {
			fmt.Fprintf(&sb, "\n  PACKAGES = (%s)", pkgs)
		}
		if imports := buildQuotedList(cfg.Imports); imports != "" {
			fmt.Fprintf(&sb, "\n  IMPORTS = (%s)", imports)
		}
		if h := strings.TrimSpace(cfg.Handler); h != "" {
			fmt.Fprintf(&sb, "\n  HANDLER = '%s'", snowflake.EscapeStringLit(h))
		}
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	// Body — the returned expression (SQL) or handler source code (other
	// languages), wrapped in $$ … $$. Always last.
	b := strings.TrimSpace(cfg.Body)
	if b == "" {
		b = "<function_body>"
	}
	fmt.Fprintf(&sb, "\n  AS $$\n%s\n$$", b)

	return sb.String() + ";", nil
}

// buildArgList renders the comma-separated "<name> <dataType>" list used by both
// the parameter list and the RETURNS TABLE column list. Entries with a blank name
// are skipped; a blank type falls back to a placeholder so the preview stays
// readable. Returns "" for an empty list.
func buildArgList(args []FuncArg) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			continue
		}
		typ := strings.TrimSpace(a.DataType)
		if typ == "" {
			typ = "VARIANT"
		}
		parts = append(parts, fmt.Sprintf("%s %s", snowflake.QuoteOrBare(name, false), typ))
	}
	return strings.Join(parts, ", ")
}

// buildQuotedList renders a comma-separated list of single-quoted string literals
// (for PACKAGES / IMPORTS), skipping blank entries. Returns "" when none are set.
func buildQuotedList(items []string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		v := strings.TrimSpace(it)
		if v == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("'%s'", snowflake.EscapeStringLit(v)))
	}
	return strings.Join(parts, ", ")
}
