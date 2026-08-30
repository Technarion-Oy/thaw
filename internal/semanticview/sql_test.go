// SPDX-License-Identifier: GPL-3.0-or-later

package semanticview

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildCreateSemanticViewSql(t *testing.T) {
	body := "TABLES (\n    orders AS DB.SC.ORDERS PRIMARY KEY (order_id)\n  )\n  METRICS (\n    orders.revenue AS SUM(amount)\n  )"

	tests := []struct {
		name     string
		db       string
		schema   string
		cfg      SemanticViewConfig
		contains []string
		absent   []string
		// order asserts the listed substrings appear in this relative order —
		// CREATE SEMANTIC VIEW rejects a body whose clauses are out of sequence.
		order []string
	}{
		{
			name:   "basic with body and comment",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:    "sales",
				Body:    body,
				Comment: "sales layer",
			},
			contains: []string{
				`CREATE SEMANTIC VIEW "DB"."SC".sales`,
				"TABLES (",
				"orders.revenue AS SUM(amount)",
				"COMMENT = 'sales layer'",
			},
			absent: []string{"OR REPLACE", "IF NOT EXISTS", "COPY GRANTS"},
		},
		{
			name:   "or replace wins over if not exists",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:        "sales",
				OrReplace:   true,
				IfNotExists: true,
				Body:        body,
				CopyGrants:  true,
			},
			contains: []string{
				`CREATE OR REPLACE SEMANTIC VIEW "DB"."SC".sales`,
				"COPY GRANTS",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name:   "if not exists alone",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:        "sales",
				IfNotExists: true,
				Body:        body,
			},
			contains: []string{`CREATE SEMANTIC VIEW IF NOT EXISTS "DB"."SC".sales`},
			absent:   []string{"OR REPLACE"},
		},
		{
			name:   "case sensitive name preserved",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
				Body:          body,
			},
			contains: []string{`"MixedCase"`},
		},
		{
			name:   "blank body falls back to placeholder",
			db:     "DB",
			schema: "SC",
			cfg:    SemanticViewConfig{Name: "sales"},
			contains: []string{
				"TABLES (",
				"my_table AS",
			},
		},
		{
			name:     "blank name falls back to placeholder",
			db:       "DB",
			schema:   "SC",
			cfg:      SemanticViewConfig{Body: body},
			contains: []string{"semantic_view_name"},
		},
		{
			name:   "comment with single quote escaped",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:    "sales",
				Body:    body,
				Comment: "it's fine",
			},
			contains: []string{"COMMENT = 'it''s fine'"},
		},
		{
			name:   "structured clauses render in the required order",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name: "sales",
				Tables: []LogicalTable{
					{
						Alias:      "orders",
						Name:       `"DB"."SC"."ORDERS"`,
						PrimaryKey: []string{"order_id"},
						Unique:     []string{"external_id"},
						Synonyms:   []string{"sales", "purchases"},
						Comment:    "Order facts",
						Tags:       []snowflake.TagPair{{Name: "domain", Value: "sales"}},
					},
					{Alias: "customers", Name: `"DB"."SC"."CUSTOMERS"`, PrimaryKey: []string{"customer_id"}},
				},
				Relationships: []Relationship{
					{Name: "orders_to_customers", Table: "orders", Columns: []string{"customer_id"}, RefTable: "customers", RefColumns: []string{"customer_id"}},
				},
				Facts: []Expression{
					{TableAlias: "orders", Name: "amount", Expr: "orders.amount", Visibility: "PRIVATE", FilterLabel: true},
				},
				Dimensions: []Expression{
					{TableAlias: "customers", Name: "region", Expr: "customers.region", CortexSearchService: `"DB"."SC"."REGION_SEARCH"`, CortexSearchColumn: "region"},
				},
				Metrics: []Expression{
					{TableAlias: "orders", Name: "revenue", Expr: "SUM(orders.amount)", Visibility: "PUBLIC"},
				},
			},
			contains: []string{
				`orders AS "DB"."SC"."ORDERS" PRIMARY KEY (order_id) UNIQUE (external_id) WITH SYNONYMS = ('sales', 'purchases') COMMENT = 'Order facts' WITH TAG ("domain" = 'sales')`,
				"orders_to_customers AS orders (customer_id) REFERENCES customers (customer_id)",
				"PRIVATE orders.amount LABELS = (FILTER) AS orders.amount",
				`customers.region AS customers.region WITH CORTEX SEARCH SERVICE "DB"."SC"."REGION_SEARCH" USING region`,
				"PUBLIC orders.revenue AS SUM(orders.amount)",
			},
			// The placeholder body must not leak once real tables are supplied.
			absent: []string{"my_table AS"},
			order: []string{
				"TABLES (", "RELATIONSHIPS (", "FACTS (", "DIMENSIONS (", "METRICS (",
			},
		},
		{
			name:   "raw body overrides the structured clauses",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:       "sales",
				Body:       body,
				Tables:     []LogicalTable{{Alias: "ignored", Name: `"DB"."SC"."IGNORED"`}},
				Dimensions: []Expression{{TableAlias: "ignored", Name: "d", Expr: "1"}},
			},
			contains: []string{"orders AS DB.SC.ORDERS"},
			absent:   []string{"IGNORED", "DIMENSIONS ("},
		},
		{
			name:   "incomplete rows are dropped rather than emitted",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:   "sales",
				Tables: []LogicalTable{{Alias: "orders", Name: `"DB"."SC"."ORDERS"`}, {Alias: "no_table"}},
				// Missing RefTable / source columns / expression respectively.
				Relationships: []Relationship{{Table: "orders", Columns: []string{"customer_id"}}},
				Metrics:       []Expression{{TableAlias: "orders", Name: "revenue"}},
			},
			contains: []string{"orders AS"},
			absent:   []string{"no_table", "RELATIONSHIPS (", "METRICS ("},
		},
		{
			name:   "preview join types and distinct range",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name: "sales",
				Tables: []LogicalTable{{
					Alias: "rates", Name: `"DB"."SC"."RATES"`,
					ConstraintName: "rate_range", RangeStart: "valid_from", RangeEnd: "valid_to",
				}},
				Relationships: []Relationship{
					{Table: "orders", Columns: []string{"ts"}, RefTable: "rates", RefColumns: []string{"ts"}, JoinType: "ASOF"},
					{Table: "orders", Columns: []string{"ts"}, RefTable: "rates", JoinType: "BETWEEN", RangeStart: "valid_from", RangeEnd: "valid_to"},
				},
			},
			contains: []string{
				"CONSTRAINT rate_range DISTINCT RANGE BETWEEN valid_from AND valid_to EXCLUSIVE",
				"orders (ts) REFERENCES rates (ASOF ts)",
				"orders (ts) REFERENCES rates (BETWEEN valid_from AND valid_to EXCLUSIVE)",
			},
		},
		{
			name:   "metric using and non additive by",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:   "sales",
				Tables: []LogicalTable{{Alias: "orders", Name: `"DB"."SC"."ORDERS"`}},
				Metrics: []Expression{{
					TableAlias:    "orders",
					Name:          "balance",
					Expr:          "SUM(orders.amount)",
					Using:         []string{"orders_to_customers"},
					NonAdditiveBy: []NonAdditiveDim{{Dimension: "orders.order_date", Direction: "DESC", Nulls: "LAST"}},
				}},
			},
			contains: []string{
				"orders.balance USING (orders_to_customers) NON ADDITIVE BY (orders.order_date DESC NULLS LAST) AS SUM(orders.amount)",
			},
		},
		{
			name:   "dimension private is dropped, window metric expression passes through",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:       "sales",
				Tables:     []LogicalTable{{Alias: "orders", Name: `"DB"."SC"."ORDERS"`}},
				Dimensions: []Expression{{TableAlias: "orders", Name: "order_date", Expr: "orders.order_date", Visibility: "PRIVATE"}},
				Metrics: []Expression{{
					TableAlias: "orders", Name: "running_total",
					Expr: "SUM(orders.amount) OVER (PARTITION BY orders.region ORDER BY orders.order_date)",
				}},
			},
			contains: []string{
				"SUM(orders.amount) OVER (PARTITION BY orders.region ORDER BY orders.order_date)",
			},
			absent: []string{"PRIVATE"},
		},
		{
			name:   "view level options",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:                     "sales",
				Tables:                   []LogicalTable{{Alias: "orders", Name: `"DB"."SC"."ORDERS"`}},
				MaxStaleness:             120,
				AISqlGeneration:          "prefer joins",
				AIQuestionCategorization: "route billing questions",
				VerifiedQueries: []VerifiedQuery{{
					Name:               "top_regions",
					Question:           "What are the top regions?",
					VerifiedAt:         "1715120000",
					OnboardingQuestion: true,
					VerifiedBy:         "( analyst = jane )",
					Sql:                "SELECT * FROM SEMANTIC_VIEW(sales METRICS revenue)",
				}},
				Tags:       []snowflake.TagPair{{Name: "tier", Value: "gold"}},
				CopyGrants: true,
			},
			contains: []string{
				"MAX_STALENESS = 120",
				"AI_SQL_GENERATION 'prefer joins'",
				"AI_QUESTION_CATEGORIZATION 'route billing questions'",
				"AI_VERIFIED_QUERIES (",
				`top_regions AS (QUESTION 'What are the top regions?' VERIFIED_AT 1715120000 ONBOARDING_QUESTION TRUE VERIFIED_BY '( analyst = jane )' SQL 'SELECT * FROM SEMANTIC_VIEW(sales METRICS revenue)')`,
				`WITH TAG ("tier" = 'gold')`,
				"COPY GRANTS",
			},
		},
		{
			name:   "expression without a table alias is dropped",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:   "sales",
				Tables: []LogicalTable{{Alias: "orders", Name: `"DB"."SC"."ORDERS"`}},
				// The grammar is `<table_alias>.<name> AS <expr>` — an alias-less
				// row would render as a bare `name AS expr`, which Snowflake rejects.
				Metrics: []Expression{
					{Name: "orphan", Expr: "SUM(1)"},
					{TableAlias: "orders", Name: "revenue", Expr: "SUM(orders.amount)"},
				},
			},
			contains: []string{"orders.revenue AS SUM(orders.amount)"},
			absent:   []string{"orphan"},
		},
		{
			name:   "case sensitive quotes every identifier, not just the view name",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:          "Sales",
				CaseSensitive: true,
				Tables: []LogicalTable{{
					Alias: "Orders", Name: `"DB"."SC"."Orders"`, PrimaryKey: []string{"OrderId"},
				}},
				Relationships: []Relationship{
					{Name: "Rel", Table: "Orders", Columns: []string{"CustomerId"}, RefTable: "Orders", RefColumns: []string{"OrderId"}},
				},
				Metrics: []Expression{{TableAlias: "Orders", Name: "Revenue", Expr: "SUM(Orders.Amount)"}},
			},
			contains: []string{
				`"Sales"`,
				`"Orders" AS "DB"."SC"."Orders" PRIMARY KEY ("OrderId")`,
				`"Rel" AS "Orders" ("CustomerId") REFERENCES "Orders" ("OrderId")`,
				`"Orders"."Revenue" AS SUM(Orders.Amount)`,
			},
		},
		{
			name:   "trailing clause order differs between tables and expressions",
			db:     "DB",
			schema: "SC",
			// logicalTable orders the trailing clauses SYNONYMS → COMMENT → TAG;
			// the fact/dimension/metric productions order them SYNONYMS → TAG →
			// COMMENT. Both are asserted so the divergence can't drift.
			cfg: SemanticViewConfig{
				Name: "sales",
				Tables: []LogicalTable{{
					Alias: "orders", Name: `"DB"."SC"."ORDERS"`,
					Synonyms: []string{"sales"},
					Comment:  "table comment",
					Tags:     []snowflake.TagPair{{Name: "domain", Value: "sales"}},
				}},
				Metrics: []Expression{{
					TableAlias: "orders", Name: "revenue", Expr: "SUM(orders.amount)",
					Synonyms: []string{"income"},
					Comment:  "metric comment",
					Tags:     []snowflake.TagPair{{Name: "tier", Value: "gold"}},
				}},
			},
			contains: []string{
				`WITH SYNONYMS = ('sales') COMMENT = 'table comment' WITH TAG ("domain" = 'sales')`,
				`WITH SYNONYMS = ('income') WITH TAG ("tier" = 'gold') COMMENT = 'metric comment'`,
			},
		},
		{
			name:   "preview join type without its reference form is dropped",
			db:     "DB",
			schema: "SC",
			// An ASOF/BETWEEN row missing its reference would otherwise render as
			// a plain standard relationship, silently losing the join semantics
			// the user picked.
			cfg: SemanticViewConfig{
				Name:   "sales",
				Tables: []LogicalTable{{Alias: "orders", Name: `"DB"."SC"."ORDERS"`}},
				Relationships: []Relationship{
					{Table: "orders", Columns: []string{"ts"}, RefTable: "rates", JoinType: "ASOF"},
					{Table: "orders", Columns: []string{"ts"}, RefTable: "rates", JoinType: "BETWEEN", RangeStart: "valid_from"},
					// The standard form may omit the columns — Snowflake then
					// matches the target's declared key — so this one survives.
					{Table: "orders", Columns: []string{"customer_id"}, RefTable: "customers"},
				},
			},
			contains: []string{"orders (customer_id) REFERENCES customers"},
			absent:   []string{"rates"},
		},
		{
			name:   "case sensitive quotes each half of a non additive by reference",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:          "sales",
				CaseSensitive: true,
				Tables:        []LogicalTable{{Alias: "Orders", Name: `"DB"."SC"."Orders"`}},
				Metrics: []Expression{{
					TableAlias:    "Orders",
					Name:          "Balance",
					Expr:          "SUM(Orders.Amount)",
					NonAdditiveBy: []NonAdditiveDim{{Dimension: "Orders.Order_Date"}},
				}},
			},
			contains: []string{`NON ADDITIVE BY ("Orders"."Order_Date")`},
		},
		{
			name:   "non additive by splits an alias containing a dot on the last dot",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:   "sales",
				Tables: []LogicalTable{{Alias: "ord.v2", Name: `"DB"."SC"."ORDERS"`}},
				Metrics: []Expression{{
					TableAlias:    "ord.v2",
					Name:          "balance",
					Expr:          "SUM(amount)",
					NonAdditiveBy: []NonAdditiveDim{{Dimension: "ord.v2.order_date"}},
				}},
			},
			contains: []string{`NON ADDITIVE BY ("ord.v2".order_date)`},
			absent:   []string{`."v2.order_date"`, `"ord.v2.order_date"`},
		},
		{
			name:   "free text is escaped as text, not as a control literal",
			db:     "DB",
			schema: "SC",
			// A trailing backslash in a synonym would otherwise escape the
			// closing quote and swallow the rest of the statement.
			cfg: SemanticViewConfig{
				Name:   "sales",
				Tables: []LogicalTable{{Alias: "orders", Name: `"DB"."SC"."ORDERS"`, Synonyms: []string{`sales\`}}},
			},
			contains: []string{`WITH SYNONYMS = ('sales\\')`},
		},
		{
			name:   "non numeric verified_at drops only its clause",
			db:     "DB",
			schema: "SC",
			// VERIFIED_AT is embedded unquoted, so anything non-numeric would
			// splice raw text into the statement.
			cfg: SemanticViewConfig{
				Name:   "sales",
				Tables: []LogicalTable{{Alias: "orders", Name: `"DB"."SC"."ORDERS"`}},
				VerifiedQueries: []VerifiedQuery{{
					Name: "q", Question: "Why?", Sql: "SELECT 1",
					VerifiedAt: "0 OR 1=1",
				}},
			},
			contains: []string{`q AS (QUESTION 'Why?' SQL 'SELECT 1')`},
			absent:   []string{"VERIFIED_AT"},
		},
		{
			name:   "incomplete verified query is dropped",
			db:     "DB",
			schema: "SC",
			cfg: SemanticViewConfig{
				Name:            "sales",
				Tables:          []LogicalTable{{Alias: "orders", Name: `"DB"."SC"."ORDERS"`}},
				VerifiedQueries: []VerifiedQuery{{Name: "no_sql", Question: "What?"}},
			},
			absent: []string{"AI_VERIFIED_QUERIES"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateSemanticViewSql(tt.db, tt.schema, tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(sql, ";") {
				t.Errorf("expected statement to end with ';', got:\n%s", sql)
			}
			for _, want := range tt.contains {
				if !strings.Contains(sql, want) {
					t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
				}
			}
			for _, no := range tt.absent {
				if strings.Contains(sql, no) {
					t.Errorf("expected SQL NOT to contain %q, got:\n%s", no, sql)
				}
			}
			prev := -1
			for _, want := range tt.order {
				at := strings.Index(sql, want)
				if at < 0 {
					t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
					continue
				}
				if at < prev {
					t.Errorf("expected %q to come after the preceding clause, got:\n%s", want, sql)
				}
				prev = at
			}
		})
	}
}

func TestBuildCreateSemanticViewSql_MaxStalenessFloor(t *testing.T) {
	base := SemanticViewConfig{Name: "sales", Body: "TABLES ( orders AS DB.SC.ORDERS )"}

	base.MaxStaleness = 0
	if _, err := BuildCreateSemanticViewSql("DB", "SC", base); err != nil {
		t.Errorf("unset MaxStaleness: unexpected error: %v", err)
	}

	base.MaxStaleness = MinMaxStaleness - 1
	if _, err := BuildCreateSemanticViewSql("DB", "SC", base); err == nil {
		t.Error("expected an error for MaxStaleness below the floor")
	}

	base.MaxStaleness = MinMaxStaleness
	if _, err := BuildCreateSemanticViewSql("DB", "SC", base); err != nil {
		t.Errorf("MaxStaleness at the floor: unexpected error: %v", err)
	}
}

func TestBuildAddMaterializationSql(t *testing.T) {
	base := MaterializationConfig{
		Name: "mv1", Warehouse: "WH_XS",
		Dimensions: []string{"customers.region"},
		Metrics:    []string{"orders.revenue"},
	}

	t.Run("minimal clause omits the default AUTO refresh mode", func(t *testing.T) {
		sql, err := BuildAddMaterializationSql("DB", "SC", "sales", base)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `ALTER SEMANTIC VIEW "DB"."SC"."sales" ADD MATERIALIZATION mv1 WAREHOUSE = "WH_XS" AS` +
			"\n  DIMENSIONS \"customers\".\"region\"\n  METRICS \"orders\".\"revenue\";"
		if sql != want {
			t.Errorf("got:\n%s\nwant:\n%s", sql, want)
		}
	})

	t.Run("includes REFRESH_MODE when it isn't AUTO", func(t *testing.T) {
		cfg := base
		cfg.RefreshMode = "full"
		sql, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(sql, "REFRESH_MODE = FULL") {
			t.Errorf("expected REFRESH_MODE = FULL, got:\n%s", sql)
		}
	})

	t.Run("rejects an invalid REFRESH_MODE", func(t *testing.T) {
		cfg := base
		cfg.RefreshMode = "BOGUS"
		if _, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg); err == nil {
			t.Error("expected an error for an invalid REFRESH_MODE")
		}
	})

	t.Run("parenthesizes IMMUTABLE WHERE and WHERE as raw SQL, not string literals", func(t *testing.T) {
		cfg := base
		cfg.ImmutableWhere = "region = 'US'"
		cfg.Where = "revenue > 0"
		sql, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(sql, "IMMUTABLE WHERE (region = 'US')") {
			t.Errorf("expected IMMUTABLE WHERE clause, got:\n%s", sql)
		}
		if !strings.Contains(sql, "WHERE (revenue > 0)") {
			t.Errorf("expected WHERE clause, got:\n%s", sql)
		}
	})

	t.Run("qualifies each dimension/metric by table, splitting on the last dot", func(t *testing.T) {
		cfg := MaterializationConfig{
			Name: "mv1", Warehouse: "WH_XS",
			Dimensions: []string{"customers.region", "orders.segment"},
			Metrics:    []string{"orders.revenue"},
		}
		sql, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(sql, `DIMENSIONS "customers"."region", "orders"."segment"`) {
			t.Errorf("expected qualified dimensions, got:\n%s", sql)
		}
	})

	t.Run("falls back to a bare quoted identifier when there's no dot", func(t *testing.T) {
		cfg := MaterializationConfig{
			Name: "mv1", Warehouse: "WH_XS",
			Dimensions: []string{"region"},
			Metrics:    []string{"revenue"},
		}
		sql, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(sql, `DIMENSIONS "region"`) || !strings.Contains(sql, `METRICS "revenue"`) {
			t.Errorf("expected bare quoted identifiers, got:\n%s", sql)
		}
	})

	t.Run("quotes a name containing special characters", func(t *testing.T) {
		cfg := base
		cfg.Name = `my "mv"`
		sql, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(sql, `ADD MATERIALIZATION "my ""mv"""`) {
			t.Errorf("expected doubled embedded quotes, got:\n%s", sql)
		}
	})

	t.Run("leaves a plain bare name unquoted, letting Snowflake fold its case", func(t *testing.T) {
		// A materialization name is free-typed (unlike Warehouse, picked from
		// an existing ListWarehouses value), so unlike the warehouse it must
		// not be force-quoted: forcing quotes would make an unquoted
		// `ADD MATERIALIZATION sales_mv` (folded by Snowflake to SALES_MV)
		// unmatchable by a later SUSPEND/RESUME/REFRESH/DROP typed the same
		// lowercase way.
		cfg := base
		cfg.Name = "sales_mv"
		sql, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(sql, "ADD MATERIALIZATION sales_mv WAREHOUSE") {
			t.Errorf("expected an unquoted bare name, got:\n%s", sql)
		}
	})

	t.Run("still quotes the warehouse even when its name is bare-safe", func(t *testing.T) {
		cfg := base
		cfg.Warehouse = "wh_xs"
		sql, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(sql, `WAREHOUSE = "wh_xs"`) {
			t.Errorf(`expected WAREHOUSE = "wh_xs", got:%s`, sql)
		}
	})

	t.Run("requires a name", func(t *testing.T) {
		cfg := base
		cfg.Name = ""
		if _, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg); err == nil {
			t.Error("expected an error for a missing name")
		}
	})

	t.Run("requires a warehouse", func(t *testing.T) {
		cfg := base
		cfg.Warehouse = ""
		if _, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg); err == nil {
			t.Error("expected an error for a missing warehouse")
		}
	})

	t.Run("requires at least one dimension", func(t *testing.T) {
		cfg := base
		cfg.Dimensions = nil
		if _, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg); err == nil {
			t.Error("expected an error for no dimensions")
		}
	})

	t.Run("requires at least one metric", func(t *testing.T) {
		cfg := base
		cfg.Metrics = nil
		if _, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg); err == nil {
			t.Error("expected an error for no metrics")
		}
	})

	t.Run("rejects a dimension list that is blank after trimming", func(t *testing.T) {
		cfg := base
		cfg.Dimensions = []string{"   "}
		if _, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg); err == nil {
			t.Error("expected an error for a whitespace-only dimension, not a rendered empty DIMENSIONS clause")
		}
	})

	t.Run("rejects a metric list that is blank after trimming", func(t *testing.T) {
		cfg := base
		cfg.Metrics = []string{"   "}
		if _, err := BuildAddMaterializationSql("DB", "SC", "sales", cfg); err == nil {
			t.Error("expected an error for a whitespace-only metric, not a rendered empty METRICS clause")
		}
	})
}
