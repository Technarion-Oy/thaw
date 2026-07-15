// SPDX-License-Identifier: GPL-3.0-or-later

package cortexsearchservice

import (
	"strings"
	"testing"
)

func TestBuildCreateCortexSearchServiceSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      CortexSearchServiceConfig
		contains []string
		absent   []string
	}{
		{
			name: "full single-index service",
			cfg: CortexSearchServiceConfig{
				Name:           "DOCS_SEARCH",
				SearchColumn:   "BODY",
				Attributes:     []string{"CATEGORY", "AUTHOR"},
				Warehouse:      "SEARCH_WH",
				TargetLag:      "1 hour",
				EmbeddingModel: "snowflake-arctic-embed-m-v1.5",
				Comment:        "doc index",
				Query:          "SELECT id, body, category, author FROM docs",
			},
			contains: []string{
				"CREATE CORTEX SEARCH SERVICE \"DB\".\"SC\".DOCS_SEARCH",
				"ON BODY",
				"ATTRIBUTES CATEGORY, AUTHOR",
				"WAREHOUSE = \"SEARCH_WH\"",
				"TARGET_LAG = '1 hour'",
				"EMBEDDING_MODEL = 'snowflake-arctic-embed-m-v1.5'",
				"COMMENT = 'doc index'",
				"AS\nSELECT id, body, category, author FROM docs",
			},
			absent: []string{"OR REPLACE", "IF NOT EXISTS"},
		},
		{
			name: "minimal service omits optional clauses",
			cfg: CortexSearchServiceConfig{
				Name:         "MIN_SEARCH",
				SearchColumn: "TXT",
				Warehouse:    "WH",
				TargetLag:    "2 minutes",
				Query:        "SELECT id, txt FROM t",
			},
			contains: []string{
				"CREATE CORTEX SEARCH SERVICE \"DB\".\"SC\".MIN_SEARCH",
				"ON TXT",
				"WAREHOUSE = \"WH\"",
				"TARGET_LAG = '2 minutes'",
			},
			absent: []string{"ATTRIBUTES", "EMBEDDING_MODEL", "COMMENT"},
		},
		{
			name: "or replace wins over if not exists",
			cfg: CortexSearchServiceConfig{
				Name:         "BOTH",
				OrReplace:    true,
				IfNotExists:  true,
				SearchColumn: "C",
				Warehouse:    "WH",
				TargetLag:    "1 hour",
				Query:        "SELECT id, c FROM t",
			},
			contains: []string{"CREATE OR REPLACE CORTEX SEARCH SERVICE \"DB\".\"SC\".BOTH"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists and case-sensitive name",
			cfg: CortexSearchServiceConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
				IfNotExists:   true,
				SearchColumn:  "C",
				Warehouse:     "WH",
				TargetLag:     "1 hour",
				Query:         "SELECT id, c FROM t",
			},
			contains: []string{"CREATE CORTEX SEARCH SERVICE IF NOT EXISTS \"DB\".\"SC\".\"MixedCase\""},
			absent:   []string{"OR REPLACE"},
		},
		{
			name: "blanks render placeholders",
			cfg:  CortexSearchServiceConfig{},
			contains: []string{
				"CREATE CORTEX SEARCH SERVICE \"DB\".\"SC\".search_service_name",
				"ON <search_column>",
				"WAREHOUSE = <warehouse>",
				"TARGET_LAG = '1 hour'",
				"SELECT id, text_column FROM <source_table>",
			},
		},
		{
			name: "blank attributes are skipped",
			cfg: CortexSearchServiceConfig{
				Name:         "S",
				SearchColumn: "C",
				Attributes:   []string{"", "  ", "KEEP"},
				Warehouse:    "WH",
				TargetLag:    "1 hour",
				Query:        "SELECT id, c, keep FROM t",
			},
			contains: []string{"ATTRIBUTES KEEP"},
		},
		{
			name: "single-index advanced options",
			cfg: CortexSearchServiceConfig{
				Name:                       "ADV",
				SearchColumn:               "BODY",
				PrimaryKey:                 []string{"ID"},
				Warehouse:                  "WH",
				TargetLag:                  "1 hour",
				RefreshMode:                "incremental",
				Initialize:                 "on_schedule",
				FullIndexBuildIntervalDays: 7,
				RequestLogging:             true,
				AutoSuspend:                3600,
				Query:                      "SELECT id, body FROM t",
			},
			contains: []string{
				"ON BODY",
				"PRIMARY KEY ( ID )",
				"REFRESH_MODE = INCREMENTAL",
				"INITIALIZE = ON_SCHEDULE",
				"FULL_INDEX_BUILD_INTERVAL_DAYS = 7",
				"REQUEST_LOGGING = TRUE",
				"AUTO_SUSPEND = 3600",
			},
		},
		{
			name: "multi-index service",
			cfg: CortexSearchServiceConfig{
				Name:          "MULTI",
				IndexMode:     IndexModeMulti,
				TextIndexes:   []string{"TITLE", "BODY"},
				VectorIndexes: []string{"BODY (model='snowflake-arctic-embed-m')", "EMBEDDING_COL"},
				PrimaryKey:    []string{"ID"},
				Attributes:    []string{"CATEGORY"},
				Warehouse:     "WH",
				TargetLag:     "1 hour",
				// EMBEDDING_MODEL must be dropped in multi mode even if supplied.
				EmbeddingModel: "snowflake-arctic-embed-m-v1.5",
				Query:          "SELECT id, title, body, embedding_col, category FROM t",
			},
			contains: []string{
				"CREATE CORTEX SEARCH SERVICE \"DB\".\"SC\".MULTI",
				"TEXT INDEXES TITLE, BODY",
				"VECTOR INDEXES BODY (model='snowflake-arctic-embed-m'), EMBEDDING_COL",
				"PRIMARY KEY ( ID )",
				"ATTRIBUTES CATEGORY",
			},
			absent: []string{"ON BODY", "EMBEDDING_MODEL"},
		},
		{
			name: "multi-index drops IF NOT EXISTS and emits vector placeholder",
			cfg: CortexSearchServiceConfig{
				Name:        "M2",
				IndexMode:   IndexModeMulti,
				IfNotExists: true,
				Warehouse:   "WH",
				TargetLag:   "1 hour",
				Query:       "SELECT id, v FROM t",
			},
			contains: []string{"VECTOR INDEXES <vector_column>"},
			absent:   []string{"IF NOT EXISTS", "TEXT INDEXES"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateCortexSearchServiceSql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(sql, ";") {
				t.Errorf("expected trailing semicolon, got:\n%s", sql)
			}
			for _, want := range tt.contains {
				if !strings.Contains(sql, want) {
					t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
				}
			}
			for _, bad := range tt.absent {
				if strings.Contains(sql, bad) {
					t.Errorf("expected SQL to NOT contain %q, got:\n%s", bad, sql)
				}
			}
		})
	}
}

// TestClauseOrder pins the emitted clause order to the order Snowflake documents
// for each shape. The grammar is order-sensitive, so this guards against a future
// reordering of the builder that the substring-only assertions above would miss.
func TestClauseOrder(t *testing.T) {
	tests := []struct {
		name  string
		cfg   CortexSearchServiceConfig
		order []string
	}{
		{
			name: "single-index",
			cfg: CortexSearchServiceConfig{
				Name:                       "S",
				SearchColumn:               "BODY",
				PrimaryKey:                 []string{"ID"},
				Attributes:                 []string{"CAT"},
				Warehouse:                  "WH",
				TargetLag:                  "1 hour",
				EmbeddingModel:             "snowflake-arctic-embed-m-v1.5",
				RefreshMode:                "FULL",
				Initialize:                 "ON_CREATE",
				FullIndexBuildIntervalDays: 7,
				RequestLogging:             true,
				AutoSuspend:                3600,
				Comment:                    "c",
				Query:                      "SELECT id, body, cat FROM t",
			},
			order: []string{
				"ON ", "PRIMARY KEY", "ATTRIBUTES", "WAREHOUSE", "TARGET_LAG",
				"EMBEDDING_MODEL", "REFRESH_MODE", "INITIALIZE",
				"FULL_INDEX_BUILD_INTERVAL_DAYS", "REQUEST_LOGGING", "AUTO_SUSPEND",
				"COMMENT", "AS",
			},
		},
		{
			name: "multi-index",
			cfg: CortexSearchServiceConfig{
				Name:                       "M",
				IndexMode:                  IndexModeMulti,
				TextIndexes:                []string{"TITLE"},
				VectorIndexes:              []string{"BODY (model='snowflake-arctic-embed-m')"},
				PrimaryKey:                 []string{"ID"},
				Attributes:                 []string{"CAT"},
				Warehouse:                  "WH",
				TargetLag:                  "1 hour",
				RefreshMode:                "INCREMENTAL",
				Initialize:                 "ON_SCHEDULE",
				FullIndexBuildIntervalDays: 7,
				RequestLogging:             true,
				AutoSuspend:                3600,
				Comment:                    "c",
				Query:                      "SELECT id, title, body, cat FROM t",
			},
			order: []string{
				"TEXT INDEXES", "VECTOR INDEXES", "PRIMARY KEY", "ATTRIBUTES",
				"WAREHOUSE", "TARGET_LAG", "REFRESH_MODE", "INITIALIZE",
				"FULL_INDEX_BUILD_INTERVAL_DAYS", "REQUEST_LOGGING", "AUTO_SUSPEND",
				"COMMENT", "AS",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateCortexSearchServiceSql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			prev := -1
			for _, clause := range tt.order {
				idx := strings.Index(sql, clause)
				if idx < 0 {
					t.Fatalf("expected SQL to contain %q, got:\n%s", clause, sql)
				}
				if idx < prev {
					t.Errorf("clause %q out of order, got:\n%s", clause, sql)
				}
				prev = idx
			}
		})
	}
}
