// SPDX-License-Identifier: GPL-3.0-or-later

package semanticview

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// LogicalTable is one entry of the TABLES ( … ) clause:
//
//	[ <alias> AS ] <table_name>
//	  [ PRIMARY KEY ( … ) ] [ UNIQUE ( … ) ]
//	  [ CONSTRAINT [ <name> ] DISTINCT RANGE BETWEEN <s> AND <e> EXCLUSIVE ]
//	  [ WITH SYNONYMS = ( … ) ] [ COMMENT = '…' ] [ WITH TAG ( … ) ]
//
// Name is the physical table/view reference and is emitted verbatim — the
// create modal supplies an already-quoted fully-qualified name from its
// database → schema → object picker.
type LogicalTable struct {
	Alias      string   `json:"alias"`
	Name       string   `json:"name"`
	PrimaryKey []string `json:"primaryKey"`
	// Unique is a single UNIQUE ( … ) column list. Snowflake allows the clause to
	// repeat; a table needing several distinct UNIQUE constraints is served by the
	// modal's raw-SQL escape hatch (Body).
	// ponytail: one UNIQUE clause, model a list-of-lists if anyone asks.
	Unique []string `json:"unique"`
	// ConstraintName/RangeStart/RangeEnd render the CONSTRAINT … DISTINCT RANGE
	// clause (a Snowflake preview feature). The clause is emitted only when both
	// range columns are set.
	ConstraintName string              `json:"constraintName"`
	RangeStart     string              `json:"rangeStart"`
	RangeEnd       string              `json:"rangeEnd"`
	Synonyms       []string            `json:"synonyms"`
	Comment        string              `json:"comment"`
	Tags           []snowflake.TagPair `json:"tags"`
}

// Relationship is one entry of the RELATIONSHIPS ( … ) clause:
//
//	[ <name> AS ] <table_alias> ( <col> [ , … ] )
//	  REFERENCES <ref_table_alias>
//	  [ ( [ ASOF ] <ref_col> [ , … ] | BETWEEN <s> AND <e> EXCLUSIVE ) ]
//
// JoinType selects which reference form is emitted: "" (standard), "ASOF", or
// "BETWEEN" (the latter two are Snowflake preview features). RangeStart and
// RangeEnd apply only to "BETWEEN"; RefColumns to the other two.
type Relationship struct {
	Name       string   `json:"name"`
	Table      string   `json:"table"`
	Columns    []string `json:"columns"`
	RefTable   string   `json:"refTable"`
	RefColumns []string `json:"refColumns"`
	JoinType   string   `json:"joinType"`
	RangeStart string   `json:"rangeStart"`
	RangeEnd   string   `json:"rangeEnd"`
}

// NonAdditiveDim is one dimension of a metric's NON ADDITIVE BY ( … ) list,
// with its optional ASC/DESC and NULLS FIRST/LAST ordering.
type NonAdditiveDim struct {
	Dimension string `json:"dimension"`
	Direction string `json:"direction"`
	Nulls     string `json:"nulls"`
}

// Expression is one entry of the FACTS / DIMENSIONS / METRICS clauses. The three
// grammars overlap almost entirely, so one type covers all of them and the
// renderer gates the clause-specific parts on the owning clause:
//
//   - Visibility — PUBLIC/PRIVATE for facts and metrics; dimensions are always
//     public, so PRIVATE is dropped there.
//   - FilterLabel (LABELS = (FILTER)) — facts and dimensions only.
//   - Using / NonAdditiveBy — metrics only.
//   - CortexSearchService / CortexSearchColumn — dimensions only.
//
// Expr is the SQL expression after AS, emitted verbatim. A window-function
// metric is just an Expr carrying its own OVER ( … ) clause — Snowflake's
// separate windowFunctionMetricExpression production adds no extra keywords
// around the expression.
type Expression struct {
	Visibility          string              `json:"visibility"`
	TableAlias          string              `json:"tableAlias"`
	Name                string              `json:"name"`
	FilterLabel         bool                `json:"filterLabel"`
	Using               []string            `json:"using"`
	NonAdditiveBy       []NonAdditiveDim    `json:"nonAdditiveBy"`
	Expr                string              `json:"expr"`
	Synonyms            []string            `json:"synonyms"`
	Tags                []snowflake.TagPair `json:"tags"`
	Comment             string              `json:"comment"`
	CortexSearchService string              `json:"cortexSearchService"`
	CortexSearchColumn  string              `json:"cortexSearchColumn"`
}

// VerifiedQuery is one entry of the AI_VERIFIED_QUERIES ( … ) clause — a
// question/SQL pair Cortex Analyst can reuse verbatim. VerifiedAt is a
// timestamp emitted unquoted (Snowflake's examples use a Unix epoch integer).
type VerifiedQuery struct {
	Name               string `json:"name"`
	Question           string `json:"question"`
	VerifiedAt         string `json:"verifiedAt"`
	OnboardingQuestion bool   `json:"onboardingQuestion"`
	VerifiedBy         string `json:"verifiedBy"`
	Sql                string `json:"sql"`
}

// SemanticViewConfig holds the parameters for creating a Snowflake SEMANTIC
// VIEW object. The order-sensitive definition clauses — TABLES ( … )
// [ RELATIONSHIPS ( … ) ] [ FACTS ( … ) ] [ DIMENSIONS ( … ) ] [ METRICS ( … ) ]
// — are modeled structurally so the create modal can drive them from form
// controls and the builder (not the user) guarantees the clause order.
//
// Body is the raw-SQL escape hatch: when non-blank it replaces the whole
// structured definition, so anything the form doesn't cover can still be typed
// into the modal's Monaco editor.
type SemanticViewConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	// Body overrides Tables/Relationships/Facts/Dimensions/Metrics when set, and
	// is emitted verbatim between the name and the COMMENT clause.
	Body                     string              `json:"body"`
	Tables                   []LogicalTable      `json:"tables"`
	Relationships            []Relationship      `json:"relationships"`
	Facts                    []Expression        `json:"facts"`
	Dimensions               []Expression        `json:"dimensions"`
	Metrics                  []Expression        `json:"metrics"`
	Comment                  string              `json:"comment"`
	MaxStaleness             int                 `json:"maxStaleness"`
	AISqlGeneration          string              `json:"aiSqlGeneration"`
	AIQuestionCategorization string              `json:"aiQuestionCategorization"`
	VerifiedQueries          []VerifiedQuery     `json:"verifiedQueries"`
	Tags                     []snowflake.TagPair `json:"tags"`
	CopyGrants               bool                `json:"copyGrants"`
}

// ident renders an alias/column/entity identifier, quoting it only when
// Snowflake requires it (invalid bare identifier or reserved keyword).
func ident(name string) string {
	return snowflake.QuoteOrBare(strings.TrimSpace(name), false)
}

// identList renders a comma-separated identifier list, dropping blanks.
func identList(names []string) string {
	return strings.Join(snowflake.QuoteIdentList(names, false), ", ")
}

// renderEach maps items through render and drops the entries it rejects (an
// incomplete row in the create modal renders as "" rather than broken SQL).
func renderEach[T any](items []T, render func(T) string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s := render(it); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// clause wraps rendered entries in "\n  KEYWORD (\n    …,\n    …\n  )", or
// returns "" when there is nothing to emit.
func clause(keyword string, entries []string) string {
	if len(entries) == 0 {
		return ""
	}
	return "\n  " + keyword + " (\n    " + strings.Join(entries, ",\n    ") + "\n  )"
}

func synonymsClause(synonyms []string) string {
	cleaned := snowflake.CleanList(synonyms)
	if len(cleaned) == 0 {
		return ""
	}
	quoted := make([]string, len(cleaned))
	for i, s := range cleaned {
		quoted[i] = snowflake.QuoteStringLit(s)
	}
	return " WITH SYNONYMS = (" + strings.Join(quoted, ", ") + ")"
}

func commentPart(comment string) string {
	if c := strings.TrimSpace(comment); c != "" {
		return " COMMENT = " + snowflake.QuoteTextLit(c)
	}
	return ""
}

func tagPart(tags []snowflake.TagPair) string {
	if c := snowflake.TagClause(tags); c != "" {
		return " WITH " + c
	}
	return ""
}

// renderTable emits one logicalTable. A row without a physical table name is
// incomplete and renders as "".
func renderTable(t LogicalTable) string {
	name := strings.TrimSpace(t.Name)
	if name == "" {
		return ""
	}
	var sb strings.Builder
	if alias := strings.TrimSpace(t.Alias); alias != "" {
		sb.WriteString(ident(alias) + " AS ")
	}
	sb.WriteString(name)
	if pk := identList(t.PrimaryKey); pk != "" {
		sb.WriteString(" PRIMARY KEY (" + pk + ")")
	}
	if uq := identList(t.Unique); uq != "" {
		sb.WriteString(" UNIQUE (" + uq + ")")
	}
	// CONSTRAINT … DISTINCT RANGE needs both bounds; a half-filled range is
	// dropped rather than emitted as invalid SQL.
	if start, end := strings.TrimSpace(t.RangeStart), strings.TrimSpace(t.RangeEnd); start != "" && end != "" {
		sb.WriteString(" CONSTRAINT ")
		if n := strings.TrimSpace(t.ConstraintName); n != "" {
			sb.WriteString(ident(n) + " ")
		}
		sb.WriteString("DISTINCT RANGE BETWEEN " + ident(start) + " AND " + ident(end) + " EXCLUSIVE")
	}
	sb.WriteString(synonymsClause(t.Synonyms))
	sb.WriteString(commentPart(t.Comment))
	sb.WriteString(tagPart(t.Tags))
	return sb.String()
}

// renderRelationship emits one relationshipDef. A row missing either side or
// the source columns is incomplete and renders as "".
func renderRelationship(r Relationship) string {
	table, refTable, cols := strings.TrimSpace(r.Table), strings.TrimSpace(r.RefTable), identList(r.Columns)
	if table == "" || refTable == "" || cols == "" {
		return ""
	}
	var sb strings.Builder
	if n := strings.TrimSpace(r.Name); n != "" {
		sb.WriteString(ident(n) + " AS ")
	}
	sb.WriteString(ident(table) + " (" + cols + ") REFERENCES " + ident(refTable))
	switch strings.ToUpper(strings.TrimSpace(r.JoinType)) {
	case "BETWEEN":
		if start, end := strings.TrimSpace(r.RangeStart), strings.TrimSpace(r.RangeEnd); start != "" && end != "" {
			sb.WriteString(" (BETWEEN " + ident(start) + " AND " + ident(end) + " EXCLUSIVE)")
		}
	case "ASOF":
		if refCols := identList(r.RefColumns); refCols != "" {
			sb.WriteString(" (ASOF " + refCols + ")")
		}
	default:
		if refCols := identList(r.RefColumns); refCols != "" {
			sb.WriteString(" (" + refCols + ")")
		}
	}
	return sb.String()
}

func renderNonAdditive(dims []NonAdditiveDim) string {
	parts := make([]string, 0, len(dims))
	for _, d := range dims {
		// The dimension may be a dotted alias.dimension reference, so it is
		// emitted verbatim rather than quoted as a single identifier.
		name := strings.TrimSpace(d.Dimension)
		if name == "" {
			continue
		}
		if dir := strings.ToUpper(strings.TrimSpace(d.Direction)); dir == "ASC" || dir == "DESC" {
			name += " " + dir
		}
		if nulls := strings.ToUpper(strings.TrimSpace(d.Nulls)); nulls == "FIRST" || nulls == "LAST" {
			name += " NULLS " + nulls
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, ", ")
}

// renderExpression emits one FACTS / DIMENSIONS / METRICS entry. kind is the
// owning clause keyword and gates the clause-specific parts (see Expression).
// A row without a name or expression is incomplete and renders as "".
func renderExpression(kind string, e Expression) string {
	name, expr := strings.TrimSpace(e.Name), strings.TrimSpace(e.Expr)
	if name == "" || expr == "" {
		return ""
	}
	var sb strings.Builder
	switch strings.ToUpper(strings.TrimSpace(e.Visibility)) {
	case "PUBLIC":
		sb.WriteString("PUBLIC ")
	case "PRIVATE":
		// Dimensions have no PRIVATE form — they are always public.
		if kind != "DIMENSIONS" {
			sb.WriteString("PRIVATE ")
		}
	}
	if alias := strings.TrimSpace(e.TableAlias); alias != "" {
		sb.WriteString(ident(alias) + ".")
	}
	sb.WriteString(ident(name))
	if kind == "METRICS" {
		if using := identList(e.Using); using != "" {
			sb.WriteString(" USING (" + using + ")")
		}
		if nonAdditive := renderNonAdditive(e.NonAdditiveBy); nonAdditive != "" {
			sb.WriteString(" NON ADDITIVE BY (" + nonAdditive + ")")
		}
	} else if e.FilterLabel {
		sb.WriteString(" LABELS = (FILTER)")
	}
	sb.WriteString(" AS " + expr)
	sb.WriteString(synonymsClause(e.Synonyms))
	sb.WriteString(tagPart(e.Tags))
	sb.WriteString(commentPart(e.Comment))
	if kind == "DIMENSIONS" {
		// The service name arrives fully qualified and already quoted from the
		// modal's picker, so it is emitted verbatim.
		if svc := strings.TrimSpace(e.CortexSearchService); svc != "" {
			sb.WriteString(" WITH CORTEX SEARCH SERVICE " + svc)
			if col := strings.TrimSpace(e.CortexSearchColumn); col != "" {
				sb.WriteString(" USING " + ident(col))
			}
		}
	}
	return sb.String()
}

// renderVerifiedQuery emits one verifiedQuery. The name, question, and SQL are
// all required; a row missing any of them renders as "".
func renderVerifiedQuery(q VerifiedQuery) string {
	name, question, query := strings.TrimSpace(q.Name), strings.TrimSpace(q.Question), strings.TrimSpace(q.Sql)
	if name == "" || question == "" || query == "" {
		return ""
	}
	parts := []string{"QUESTION " + snowflake.QuoteTextLit(question)}
	if at := strings.TrimSpace(q.VerifiedAt); at != "" {
		parts = append(parts, "VERIFIED_AT "+at)
	}
	if q.OnboardingQuestion {
		parts = append(parts, "ONBOARDING_QUESTION TRUE")
	}
	if by := strings.TrimSpace(q.VerifiedBy); by != "" {
		parts = append(parts, "VERIFIED_BY "+snowflake.QuoteTextLit(by))
	}
	parts = append(parts, "SQL "+snowflake.QuoteTextLit(query))
	return ident(name) + " AS (" + strings.Join(parts, " ") + ")"
}

// definitionBody renders the order-sensitive TABLES → RELATIONSHIPS → FACTS →
// DIMENSIONS → METRICS block. A non-blank Body replaces the whole block (the
// modal's raw-SQL escape hatch). With neither a body nor any logical table, a
// minimal TABLES placeholder keeps the live preview a completable template
// rather than invalid SQL.
func definitionBody(cfg SemanticViewConfig) string {
	if body := strings.TrimSpace(cfg.Body); body != "" {
		return "\n  " + body
	}
	var sb strings.Builder
	if tables := renderEach(cfg.Tables, renderTable); len(tables) > 0 {
		sb.WriteString(clause("TABLES", tables))
	} else {
		sb.WriteString("\n  TABLES (\n    my_table AS <database>.<schema>.<table>\n  )")
	}
	sb.WriteString(clause("RELATIONSHIPS", renderEach(cfg.Relationships, renderRelationship)))
	for _, c := range []struct {
		keyword string
		items   []Expression
	}{
		{"FACTS", cfg.Facts},
		{"DIMENSIONS", cfg.Dimensions},
		{"METRICS", cfg.Metrics},
	} {
		sb.WriteString(clause(c.keyword, renderEach(c.items, func(e Expression) string {
			return renderExpression(c.keyword, e)
		})))
	}
	return sb.String()
}

// BuildCreateSemanticViewSql constructs a CREATE SEMANTIC VIEW statement from
// the given config. OR REPLACE and IF NOT EXISTS are mutually exclusive in
// Snowflake; the create modal prevents selecting both, and if both are set here
// OR REPLACE wins (IF NOT EXISTS is dropped by CreateClause).
//
//	CREATE [OR REPLACE] SEMANTIC VIEW [IF NOT EXISTS] <fqn>
//	  TABLES ( … )
//	  [RELATIONSHIPS ( … )]
//	  [FACTS ( … )]
//	  [DIMENSIONS ( … )]
//	  [METRICS ( … )]
//	  [COMMENT = '…']
//	  [MAX_STALENESS = <n>]
//	  [AI_SQL_GENERATION '…']
//	  [AI_QUESTION_CATEGORIZATION '…']
//	  [AI_VERIFIED_QUERIES ( … )]
//	  [WITH TAG ( … )]
//	  [COPY GRANTS];
func BuildCreateSemanticViewSql(db, schema string, cfg SemanticViewConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("SEMANTIC VIEW", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "semantic_view_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	sb.WriteString(definitionBody(cfg))

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	if cfg.MaxStaleness > 0 {
		fmt.Fprintf(&sb, "\n  MAX_STALENESS = %d", cfg.MaxStaleness)
	}
	// Both AI_* options take a bare string literal — no "=" in the grammar.
	if v := strings.TrimSpace(cfg.AISqlGeneration); v != "" {
		sb.WriteString("\n  AI_SQL_GENERATION " + snowflake.QuoteTextLit(v))
	}
	if v := strings.TrimSpace(cfg.AIQuestionCategorization); v != "" {
		sb.WriteString("\n  AI_QUESTION_CATEGORIZATION " + snowflake.QuoteTextLit(v))
	}
	sb.WriteString(clause("AI_VERIFIED_QUERIES", renderEach(cfg.VerifiedQueries, renderVerifiedQuery)))

	if t := snowflake.TagClause(cfg.Tags); t != "" {
		sb.WriteString("\n  WITH " + t)
	}

	if cfg.CopyGrants {
		sb.WriteString("\n  COPY GRANTS")
	}

	return sb.String() + ";", nil
}
