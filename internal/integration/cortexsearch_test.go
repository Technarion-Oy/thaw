// SPDX-License-Identifier: GPL-3.0-or-later

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"thaw/internal/cortexsearchservice"
)

// TestCortexSearchServiceGrammar exercises the CREATE and ALTER grammar that the
// Cortex Search Service UI emits, against a live account. It is the smoke-test
// gate the unit tests cannot replace: a clause that is well-formed but wrong only
// fails at execution time. Each ALTER clause runs as its own non-fatal sub-test,
// so a single run reports every rejected clause rather than stopping at the first.
//
// Requires the standard integration env vars (see export_test.go) plus a region
// where Cortex Search is enabled. Run with:
//
//	go test -v -tags integration -timeout 15m -run TestCortexSearchServiceGrammar ./internal/integration/
func TestCortexSearchServiceGrammar(t *testing.T) {
	client := connFromEnv(t)
	ctx := context.Background()
	warehouse := os.Getenv("SNOWFLAKE_WAREHOUSE")

	dbName := randomName("THAW_TEST_")
	t.Logf("test database: %s", dbName)
	mustExec(t, client, fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := client.Execute(c, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)); err != nil {
			t.Logf("cleanup: drop %s: %v (manual cleanup may be required)", dbName, err)
		}
	})
	mustExec(t, client, fmt.Sprintf(`CREATE SCHEMA "%s"."PUBLIC"`, dbName))

	// Source table the service indexes.
	src := fmt.Sprintf(`"%s"."PUBLIC"."DOCS"`, dbName)
	mustExec(t, client, fmt.Sprintf(
		`CREATE TABLE %s (ID NUMBER, BODY STRING, CATEGORY STRING, AUTHOR STRING, LIKES NUMBER, CREATED_AT TIMESTAMP_NTZ)`, src))
	mustExec(t, client, fmt.Sprintf(
		`INSERT INTO %s SELECT 1, 'hello world', 'greeting', 'alice', 5, CURRENT_TIMESTAMP()`, src))

	fqnOf := func(svc string) string {
		return fmt.Sprintf(`"%s"."PUBLIC"."%s"`, dbName, svc)
	}

	// ── CREATE: single-index with the full advanced-option surface ────────────
	svc := randomName("CSS_")
	createSQL, err := cortexsearchservice.BuildCreateCortexSearchServiceSql(dbName, "PUBLIC", cortexsearchservice.CortexSearchServiceConfig{
		Name:                       svc,
		SearchColumn:               "BODY",
		Attributes:                 []string{"CATEGORY"},
		Warehouse:                  warehouse,
		TargetLag:                  "1 hour",
		RefreshMode:                "FULL",
		Initialize:                 "ON_CREATE",
		FullIndexBuildIntervalDays: 7,
		RequestLogging:             true,
		AutoSuspend:                3600,
		Comment:                    "thaw smoke test",
		Query:                      fmt.Sprintf("SELECT id, body, category, author, likes, created_at FROM %s", src),
	})
	if err != nil {
		t.Fatalf("build CREATE: %v", err)
	}
	t.Logf("CREATE:\n%s", createSQL)
	mustExec(t, client, createSQL)
	fqn := fqnOf(svc)

	// ── DESCRIBE: the column names the properties modal reads must exist ───────
	descCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	desc, err := client.Execute(descCtx, fmt.Sprintf(`DESCRIBE CORTEX SEARCH SERVICE %s`, fqn))
	if err != nil {
		t.Fatalf("DESCRIBE: %v", err)
	}
	have := make(map[string]bool, len(desc.Columns))
	for _, c := range desc.Columns {
		have[strings.ToLower(c)] = true
	}
	t.Logf("DESCRIBE columns: %v", desc.Columns)
	for _, col := range []string{
		"search_column", "attribute_columns", "definition", "target_lag",
		"warehouse", "embedding_model", "indexing_state", "serving_state",
		"primary_key", "auto_suspend", "request_logging",
		"full_index_build_interval_days",
	} {
		if !have[col] {
			t.Errorf("DESCRIBE CORTEX SEARCH SERVICE is missing column %q that the properties modal reads", col)
		}
	}

	// ── ALTER: every clause the modal can emit, each non-fatal ────────────────
	alter := func(name, clause string) {
		t.Run(name, func(t *testing.T) {
			c, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			stmt := fmt.Sprintf(`ALTER CORTEX SEARCH SERVICE %s %s`, fqn, clause)
			if _, err := client.Execute(c, stmt); err != nil {
				t.Errorf("clause rejected: %v\n  %s", err, stmt)
			}
		})
	}

	// SET property bag (note: PRIMARY KEY takes "=", ATTRIBUTES does not).
	alter("set_target_lag", "SET TARGET_LAG = '2 hours'")
	alter("set_warehouse", fmt.Sprintf(`SET WAREHOUSE = "%s"`, warehouse))
	alter("set_attributes", "SET ATTRIBUTES ( CATEGORY, AUTHOR )")
	alter("unset_attributes", "UNSET ATTRIBUTES")
	alter("set_primary_key", "SET PRIMARY KEY = ( ID )")
	alter("unset_primary_key", "UNSET PRIMARY KEY")
	alter("set_auto_suspend", "SET AUTO_SUSPEND = 1800")
	alter("set_auto_suspend_null", "SET AUTO_SUSPEND = NULL")
	alter("set_full_index_interval", "SET FULL_INDEX_BUILD_INTERVAL_DAYS = 14")
	alter("set_request_logging", "SET REQUEST_LOGGING = FALSE")
	alter("set_comment", "SET COMMENT = 'updated'")
	alter("unset_comment", "UNSET COMMENT")

	// Lifecycle (ordered to leave the service resumed).
	alter("refresh", "REFRESH")
	alter("suspend_indexing", "SUSPEND INDEXING")
	alter("resume_indexing", "RESUME INDEXING")
	alter("suspend_serving", "SUSPEND SERVING")
	alter("resume_serving", "RESUME SERVING")
	alter("suspend", "SUSPEND")
	alter("resume", "RESUME")

	// Tags.
	tag := randomName("CSS_TAG_")
	mustExec(t, client, fmt.Sprintf(`CREATE TAG "%s"."PUBLIC"."%s"`, dbName, tag))
	tagFQN := fmt.Sprintf(`"%s"."PUBLIC"."%s"`, dbName, tag)
	alter("set_tag", fmt.Sprintf(`SET TAG %s = 'v1'`, tagFQN))
	alter("unset_tag", fmt.Sprintf(`UNSET TAG %s`, tagFQN))

	// Scoring profiles. The body is a single-quoted JSON scoring config (the form
	// the properties modal wraps for the user). The clauses mirror exactly what the
	// modal emits: bare ADD (no IF NOT EXISTS) and DROP ... IF EXISTS.
	profileBody := `'{ "functions": { "numeric_boosts": [ { "column": "LIKES", "weight": 2 } ], "time_decays": [ { "column": "CREATED_AT", "weight": 1, "limit_hours": 120 } ] } }'`
	alter("add_scoring_profile", fmt.Sprintf(`ADD SCORING PROFILE "SP1" %s`, profileBody))
	alter("drop_scoring_profile", `DROP SCORING PROFILE IF EXISTS "SP1"`)

	// ── TAG_REFERENCES object domain used by GetCortexSearchServiceTags ───────
	t.Run("tag_references_domain", func(t *testing.T) {
		c, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		q := fmt.Sprintf(
			`SELECT TAG_NAME FROM TABLE("%s".INFORMATION_SCHEMA.TAG_REFERENCES('%s', 'CORTEX SEARCH SERVICE'))`,
			dbName, strings.ReplaceAll(fqn, `'`, `''`))
		if _, err := client.Execute(c, q); err != nil {
			t.Errorf("TAG_REFERENCES object domain 'CORTEX SEARCH SERVICE' rejected: %v", err)
		}
	})

	// ── CREATE: multi-index (TEXT INDEXES / VECTOR INDEXES) ───────────────────
	t.Run("create_multi_index", func(t *testing.T) {
		svc2 := randomName("CSSM_")
		sql, err := cortexsearchservice.BuildCreateCortexSearchServiceSql(dbName, "PUBLIC", cortexsearchservice.CortexSearchServiceConfig{
			Name:          svc2,
			IndexMode:     cortexsearchservice.IndexModeMulti,
			TextIndexes:   []string{"BODY"},
			VectorIndexes: []string{"BODY (model='snowflake-arctic-embed-m')"},
			PrimaryKey:    []string{"ID"},
			Attributes:    []string{"CATEGORY"},
			Warehouse:     warehouse,
			TargetLag:     "1 hour",
			Query:         fmt.Sprintf("SELECT id, body, category FROM %s", src),
		})
		if err != nil {
			t.Fatalf("build multi-index CREATE: %v", err)
		}
		t.Logf("multi-index CREATE:\n%s", sql)
		c, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if _, err := client.Execute(c, sql); err != nil {
			t.Errorf("multi-index CREATE rejected: %v", err)
		}
	})
}
