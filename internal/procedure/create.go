// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package procedure

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ProcArg is a single procedure parameter (for the signature) or a single column
// of a RETURNS TABLE result: a name and a data type. Entries with a blank name
// are skipped by the builder; a blank data type falls back to a placeholder so
// the live preview stays readable.
type ProcArg struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
}

// ProcedureConfig holds the parameters for creating a Snowflake stored
// PROCEDURE. Name is the only required field; every other field is optional — an
// empty/zero value means the corresponding clause is omitted and Snowflake's
// default applies. OR REPLACE, SECURE and IF NOT EXISTS are independent modifiers
// on the CREATE keyword (OR REPLACE and IF NOT EXISTS are mutually exclusive).
type ProcedureConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	Secure        bool   `json:"secure"`
	IfNotExists   bool   `json:"ifNotExists"`

	Args         []ProcArg `json:"args"`         // ( <name> <dataType>, ... )
	ReturnType   string    `json:"returnType"`   // RETURNS <type> (scalar)
	ReturnsTable bool      `json:"returnsTable"` // RETURNS TABLE (...)
	TableColumns []ProcArg `json:"tableColumns"` // RETURNS TABLE ( <name> <type>, ... )

	// Language is "" / "SQL" (default — clause omitted) or one of "PYTHON",
	// "JAVA", "JAVASCRIPT", "SCALA".
	Language       string   `json:"language"`
	RuntimeVersion string   `json:"runtimeVersion"` // RUNTIME_VERSION = '<v>'
	Packages       []string `json:"packages"`       // PACKAGES = ('a', 'b')
	Imports        []string `json:"imports"`        // IMPORTS = ('x')
	Handler        string   `json:"handler"`        // HANDLER = '<h>'

	// NullHandling is "" (default), "CALLED ON NULL INPUT", or
	// "RETURNS NULL ON NULL INPUT".
	NullHandling string `json:"nullHandling"`
	// Volatility is "" (default), "VOLATILE", or "IMMUTABLE".
	Volatility string `json:"volatility"`
	// ExecuteAs is "" (default), "CALLER", or "OWNER".
	ExecuteAs string `json:"executeAs"`

	Comment string `json:"comment"`
	Body    string `json:"body"`
}

// BuildCreateProcedureSql constructs a CREATE PROCEDURE statement from the given
// config. The name, argument list, return type and body always appear
// (placeholders fill in for empty required fields so the live preview stays
// valid-looking); the remaining clauses are emitted only when set, in the order
// Snowflake documents them. OR REPLACE, SECURE and IF NOT EXISTS are independent
// modifiers on the CREATE keyword. The body is wrapped in a `$$ ... $$` literal
// as the final clause.
func BuildCreateProcedureSql(db, schema string, cfg ProcedureConfig) (string, error) {
	var sb strings.Builder

	body := "PROCEDURE"
	if cfg.Secure {
		body = "SECURE " + body
	}
	createClause := snowflake.CreateClause(body, cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "procedure_name"
	}

	fmt.Fprintf(&sb, "%s %s(%s)", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive), buildProcArgList(cfg.Args))

	if cfg.ReturnsTable {
		fmt.Fprintf(&sb, "\n  RETURNS TABLE (%s)", buildProcArgList(cfg.TableColumns))
	} else {
		ret := strings.TrimSpace(cfg.ReturnType)
		if ret == "" {
			ret = "VARIANT"
		}
		fmt.Fprintf(&sb, "\n  RETURNS %s", ret)
	}

	// LANGUAGE is REQUIRED for CREATE PROCEDURE — unlike a UDF, there is no
	// implicit default, so a SQL procedure must still emit `LANGUAGE SQL`
	// (omitting it is a syntax error before AS). Default an empty language to SQL.
	lang := strings.ToUpper(strings.TrimSpace(cfg.Language))
	if lang == "" {
		lang = "SQL"
	}
	fmt.Fprintf(&sb, "\n  LANGUAGE %s", lang)

	// RUNTIME_VERSION / PACKAGES / IMPORTS / HANDLER apply only to the handler
	// languages (Python, Java, Scala). SQL (Snowflake Scripting) and JavaScript
	// procedures carry their logic inline in the body, so these clauses are
	// skipped regardless of any stale values left over from a previous language
	// selection.
	if snowflake.IsHandlerLanguage(lang) {
		if rv := strings.TrimSpace(cfg.RuntimeVersion); rv != "" {
			fmt.Fprintf(&sb, "\n  RUNTIME_VERSION = '%s'", snowflake.EscapeStringLit(rv))
		}
		if pkgs := quoteList(cfg.Packages); pkgs != "" {
			fmt.Fprintf(&sb, "\n  PACKAGES = (%s)", pkgs)
		}
		if imps := quoteList(cfg.Imports); imps != "" {
			fmt.Fprintf(&sb, "\n  IMPORTS = (%s)", imps)
		}
		if h := strings.TrimSpace(cfg.Handler); h != "" {
			fmt.Fprintf(&sb, "\n  HANDLER = '%s'", snowflake.EscapeStringLit(h))
		}
	}

	// Null-handling / volatility follow the handler clauses (RUNTIME_VERSION/…/
	// HANDLER) in the CREATE PROCEDURE grammar. This is the reverse of CREATE
	// FUNCTION (where they precede the handler clauses) — the asymmetry is
	// intentional and matches Snowflake's two separate, order-sensitive grammars.
	if nh := strings.TrimSpace(cfg.NullHandling); nh != "" {
		fmt.Fprintf(&sb, "\n  %s", strings.ToUpper(nh))
	}
	if vol := strings.TrimSpace(cfg.Volatility); vol != "" {
		fmt.Fprintf(&sb, "\n  %s", strings.ToUpper(vol))
	}

	// COMMENT precedes EXECUTE AS in the documented CREATE PROCEDURE grammar.
	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	if ea := strings.TrimSpace(cfg.ExecuteAs); ea != "" {
		fmt.Fprintf(&sb, "\n  EXECUTE AS %s", strings.ToUpper(ea))
	}

	procBody := strings.TrimSpace(cfg.Body)
	if procBody == "" {
		if lang == "SQL" {
			procBody = "BEGIN\n  RETURN 1;\nEND"
		} else {
			procBody = "# procedure body"
		}
	}
	fmt.Fprintf(&sb, "\n  AS $$\n%s\n$$", procBody)

	return sb.String() + ";", nil
}

// buildProcArgList renders the comma-separated "<name> <type>" list used for both
// the parameter signature and a RETURNS TABLE column list. Entries with a blank
// name are skipped; a blank type falls back to a placeholder so the preview stays
// readable. Returns "" for an empty list (a zero-argument procedure renders ()).
func buildProcArgList(args []ProcArg) string {
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

// quoteList renders a comma-separated list of single-quoted string literals
// (e.g. for PACKAGES / IMPORTS), trimming and skipping blank entries. Returns ""
// when none are set.
func quoteList(items []string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		if v := strings.TrimSpace(it); v != "" {
			parts = append(parts, fmt.Sprintf("'%s'", snowflake.EscapeStringLit(v)))
		}
	}
	return strings.Join(parts, ", ")
}
