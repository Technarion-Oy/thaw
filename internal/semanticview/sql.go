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
// TableAlias, Name and Expr are all required — the grammar is
// `[ visibility ] <table_alias>.<name> … AS <sql_expr>`, with no alias-less
// form — so a row missing any of them is dropped rather than rendered.
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

// renderer carries the config-wide settings the per-entity renderers need. The
// case flag applies to every identifier in the definition — aliases, columns,
// entity and relationship names — not just the view's own name, matching the
// other CREATE builders (hybrid/iceberg/external tables all quote their column
// names with cfg.CaseSensitive).
type renderer struct {
	caseSensitive bool
}

// ident renders an alias/column/entity identifier, quoting it when the config
// asks for case sensitivity or when Snowflake requires it (invalid bare
// identifier or reserved keyword).
func (r renderer) ident(name string) string {
	return snowflake.QuoteOrBare(strings.TrimSpace(name), r.caseSensitive)
}

// identList renders a comma-separated identifier list, dropping blanks.
func (r renderer) identList(names []string) string {
	return strings.Join(snowflake.QuoteIdentList(names, r.caseSensitive), ", ")
}

// qualifiedIdent renders a possibly-dotted `alias.name` reference, quoting each
// half separately — quoting the whole string would produce one identifier
// containing a dot. Used for a metric's NON ADDITIVE BY dimensions, which are
// the only references in the grammar the form supplies pre-qualified.
func (r renderer) qualifiedIdent(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	alias, name, dotted := strings.Cut(ref, ".")
	if !dotted {
		return r.ident(ref)
	}
	return r.ident(alias) + "." + r.ident(name)
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
func (r renderer) table(t LogicalTable) string {
	name := strings.TrimSpace(t.Name)
	if name == "" {
		return ""
	}
	var sb strings.Builder
	if alias := strings.TrimSpace(t.Alias); alias != "" {
		sb.WriteString(r.ident(alias) + " AS ")
	}
	sb.WriteString(name)
	if pk := r.identList(t.PrimaryKey); pk != "" {
		sb.WriteString(" PRIMARY KEY (" + pk + ")")
	}
	if uq := r.identList(t.Unique); uq != "" {
		sb.WriteString(" UNIQUE (" + uq + ")")
	}
	// CONSTRAINT … DISTINCT RANGE needs both bounds; a half-filled range is
	// dropped rather than emitted as invalid SQL.
	if start, end := strings.TrimSpace(t.RangeStart), strings.TrimSpace(t.RangeEnd); start != "" && end != "" {
		sb.WriteString(" CONSTRAINT ")
		if n := strings.TrimSpace(t.ConstraintName); n != "" {
			sb.WriteString(r.ident(n) + " ")
		}
		sb.WriteString("DISTINCT RANGE BETWEEN " + r.ident(start) + " AND " + r.ident(end) + " EXCLUSIVE")
	}
	sb.WriteString(synonymsClause(t.Synonyms))
	sb.WriteString(commentPart(t.Comment))
	sb.WriteString(tagPart(t.Tags))
	return sb.String()
}

// renderRelationship emits one relationshipDef. A row missing either side or
// the source columns is incomplete and renders as "".
func (r renderer) relationship(rel Relationship) string {
	table, refTable, cols := strings.TrimSpace(rel.Table), strings.TrimSpace(rel.RefTable), r.identList(rel.Columns)
	if table == "" || refTable == "" || cols == "" {
		return ""
	}
	var sb strings.Builder
	if n := strings.TrimSpace(rel.Name); n != "" {
		sb.WriteString(r.ident(n) + " AS ")
	}
	sb.WriteString(r.ident(table) + " (" + cols + ") REFERENCES " + r.ident(refTable))
	// The reference form is required for ASOF and BETWEEN — without it the row
	// would render as a plain standard relationship, silently dropping the join
	// semantics the user picked. Drop the row instead, like every other
	// incomplete entry. Only the standard form may omit the columns (Snowflake
	// then matches the target's declared key).
	switch strings.ToUpper(strings.TrimSpace(rel.JoinType)) {
	case "BETWEEN":
		start, end := strings.TrimSpace(rel.RangeStart), strings.TrimSpace(rel.RangeEnd)
		if start == "" || end == "" {
			return ""
		}
		sb.WriteString(" (BETWEEN " + r.ident(start) + " AND " + r.ident(end) + " EXCLUSIVE)")
	case "ASOF":
		refCols := r.identList(rel.RefColumns)
		if refCols == "" {
			return ""
		}
		sb.WriteString(" (ASOF " + refCols + ")")
	default:
		if refCols := r.identList(rel.RefColumns); refCols != "" {
			sb.WriteString(" (" + refCols + ")")
		}
	}
	return sb.String()
}

func (r renderer) nonAdditive(dims []NonAdditiveDim) string {
	parts := make([]string, 0, len(dims))
	for _, d := range dims {
		name := r.qualifiedIdent(d.Dimension)
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
// The grammar is `[ visibility ] <table_alias>.<name> … AS <sql_expr>`, so all
// three of the alias, the name and the expression are required; a row missing
// any of them is incomplete and renders as "".
func (r renderer) expression(kind string, e Expression) string {
	alias, name, expr := strings.TrimSpace(e.TableAlias), strings.TrimSpace(e.Name), strings.TrimSpace(e.Expr)
	if alias == "" || name == "" || expr == "" {
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
	sb.WriteString(r.ident(alias) + "." + r.ident(name))
	if kind == "METRICS" {
		if using := r.identList(e.Using); using != "" {
			sb.WriteString(" USING (" + using + ")")
		}
		if nonAdditive := r.nonAdditive(e.NonAdditiveBy); nonAdditive != "" {
			sb.WriteString(" NON ADDITIVE BY (" + nonAdditive + ")")
		}
	} else if e.FilterLabel {
		sb.WriteString(" LABELS = (FILTER)")
	}
	sb.WriteString(" AS " + expr)
	// Deliberately TAG before COMMENT — the opposite of renderer.table. The
	// factExpression / dimensionExpression / metricExpression productions order
	// the trailing clauses SYNONYMS → TAG → COMMENT, while logicalTable orders
	// them SYNONYMS → COMMENT → TAG. Both orders are asserted by the tests.
	sb.WriteString(synonymsClause(e.Synonyms))
	sb.WriteString(tagPart(e.Tags))
	sb.WriteString(commentPart(e.Comment))
	if kind == "DIMENSIONS" {
		// The service name arrives fully qualified and already quoted from the
		// modal's picker, so it is emitted verbatim.
		if svc := strings.TrimSpace(e.CortexSearchService); svc != "" {
			sb.WriteString(" WITH CORTEX SEARCH SERVICE " + svc)
			if col := strings.TrimSpace(e.CortexSearchColumn); col != "" {
				sb.WriteString(" USING " + r.ident(col))
			}
		}
	}
	return sb.String()
}

// renderVerifiedQuery emits one verifiedQuery. The name, question, and SQL are
// all required; a row missing any of them renders as "".
func (r renderer) verifiedQuery(q VerifiedQuery) string {
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
	return r.ident(name) + " AS (" + strings.Join(parts, " ") + ")"
}

// definitionBody renders the order-sensitive TABLES → RELATIONSHIPS → FACTS →
// DIMENSIONS → METRICS block. A non-blank Body replaces the whole block (the
// modal's raw-SQL escape hatch). With neither a body nor any logical table, a
// minimal TABLES placeholder keeps the live preview a completable template
// rather than invalid SQL.
func (r renderer) definitionBody(cfg SemanticViewConfig) string {
	if body := strings.TrimSpace(cfg.Body); body != "" {
		return "\n  " + body
	}
	var sb strings.Builder
	if tables := renderEach(cfg.Tables, r.table); len(tables) > 0 {
		sb.WriteString(clause("TABLES", tables))
	} else {
		sb.WriteString("\n  TABLES (\n    my_table AS <database>.<schema>.<table>\n  )")
	}
	sb.WriteString(clause("RELATIONSHIPS", renderEach(cfg.Relationships, r.relationship)))
	for _, c := range []struct {
		keyword string
		items   []Expression
	}{
		{"FACTS", cfg.Facts},
		{"DIMENSIONS", cfg.Dimensions},
		{"METRICS", cfg.Metrics},
	} {
		sb.WriteString(clause(c.keyword, renderEach(c.items, func(e Expression) string {
			return r.expression(c.keyword, e)
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

	// The case flag governs every identifier in the statement, not just the
	// view's own name.
	r := renderer{caseSensitive: cfg.CaseSensitive}

	createClause := snowflake.CreateClause("SEMANTIC VIEW", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "semantic_view_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	sb.WriteString(r.definitionBody(cfg))

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
	sb.WriteString(clause("AI_VERIFIED_QUERIES", renderEach(cfg.VerifiedQueries, r.verifiedQuery)))

	if t := snowflake.TagClause(cfg.Tags); t != "" {
		sb.WriteString("\n  WITH " + t)
	}

	if cfg.CopyGrants {
		sb.WriteString("\n  COPY GRANTS")
	}

	return sb.String() + ";", nil
}
