// SPDX-License-Identifier: GPL-3.0-or-later

package datametricfunction

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// DataMetricFunctionColumn is a single column of a DMF TABLE argument: a name and
// a data type (e.g. {Name: "c", Type: "NUMBER"}). Columns with a blank name are
// skipped by the builder.
type DataMetricFunctionColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DataMetricFunctionTableArg is one TABLE argument of a DMF: a name (referenced by
// the body) and its typed columns. A DMF takes one or more of these.
type DataMetricFunctionTableArg struct {
	Name    string                     `json:"name"`
	Columns []DataMetricFunctionColumn `json:"columns"`
}

// DataMetricFunctionConfig holds the parameters for creating a Snowflake DATA
// METRIC FUNCTION. A DMF takes one or more named TABLE arguments (each a set of
// typed columns), always RETURNS NUMBER, and its body is a scalar SQL expression
// that aggregates over those arguments. Body is the only field that always
// matters; the rest carry sensible defaults.
type DataMetricFunctionConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Secure        bool   `json:"secure"`

	// Args are the TABLE arguments referenced by the body. The Snowflake grammar
	// allows several; most DMFs use exactly one.
	Args []DataMetricFunctionTableArg `json:"args"`

	NotNull bool   `json:"notNull"` // RETURNS NUMBER NOT NULL
	Comment string `json:"comment"`
	Body    string `json:"body"` // AS $$ <expression> $$
}

// BuildCreateDataMetricFunctionSql constructs a CREATE DATA METRIC FUNCTION
// statement from the given config. The argument list and body always appear
// (placeholders fill in for empty required fields so the live preview stays
// valid-looking); COMMENT is emitted only when set. OR REPLACE and IF NOT EXISTS
// are mutually exclusive — OR REPLACE wins and IF NOT EXISTS is dropped. The
// return type is always NUMBER. The body is $$-quoted so multi-line SQL with
// embedded single quotes needs no escaping.
func BuildCreateDataMetricFunctionSql(db, schema string, cfg DataMetricFunctionConfig) (string, error) {
	var sb strings.Builder

	createBody := "DATA METRIC FUNCTION"
	if cfg.Secure {
		createBody = "SECURE " + createBody
	}
	createClause := snowflake.CreateClause(createBody, cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "data_metric_function_name"
	}

	argList, firstArgName := buildTableArgs(cfg.Args)
	fmt.Fprintf(&sb, "%s %s(%s)", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive), argList)

	sb.WriteString("\n  RETURNS NUMBER")
	if cfg.NotNull {
		sb.WriteString(" NOT NULL")
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	body := strings.TrimSpace(cfg.Body)
	if body == "" {
		body = "SELECT COUNT(*) FROM " + firstArgName
	}
	fmt.Fprintf(&sb, "\n  AS\n$$\n%s\n$$", body)

	return sb.String() + ";", nil
}

// buildTableArgs renders the comma-separated "<arg> TABLE(<cols>)" argument list
// and returns it together with the (bare) name of the first argument, used as the
// FROM target in the placeholder body. Args with no named columns still render
// (with a placeholder column) so the preview parses; a config with no args at all
// falls back to a single placeholder argument.
func buildTableArgs(args []DataMetricFunctionTableArg) (list, firstName string) {
	parts := make([]string, 0, len(args))
	for i, a := range args {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			name = "table_data"
			if i > 0 {
				name = fmt.Sprintf("table_data_%d", i+1)
			}
		}
		if firstName == "" {
			firstName = snowflake.QuoteOrBare(name, false)
		}
		parts = append(parts, fmt.Sprintf("%s TABLE(%s)", snowflake.QuoteOrBare(name, false), buildColumnList(a.Columns)))
	}
	if len(parts) == 0 {
		return "table_data TABLE(column_1 VARCHAR)", "table_data"
	}
	return strings.Join(parts, ", "), firstName
}

// buildColumnList renders the comma-separated "<name> <type>" column list for a
// TABLE argument. Columns with a blank name are skipped; a blank type falls back
// to a placeholder so the preview stays readable. Returns a placeholder column
// for an argument with no columns yet so the preview parses.
func buildColumnList(cols []DataMetricFunctionColumn) string {
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			continue
		}
		typ := strings.TrimSpace(c.Type)
		if typ == "" {
			typ = "VARCHAR"
		}
		parts = append(parts, fmt.Sprintf("%s %s", snowflake.QuoteOrBare(name, false), typ))
	}
	if len(parts) == 0 {
		return "column_1 VARCHAR"
	}
	return strings.Join(parts, ", ")
}
