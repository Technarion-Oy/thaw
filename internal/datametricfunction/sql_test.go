// SPDX-License-Identifier: GPL-3.0-or-later

package datametricfunction

import (
	"strings"
	"testing"
)

func TestBuildCreateDataMetricFunctionSql_Minimal(t *testing.T) {
	sql, err := BuildCreateDataMetricFunctionSql("DB", "SC", DataMetricFunctionConfig{
		Name: "NULL_COUNT",
		Args: []DataMetricFunctionTableArg{
			{Name: "arg_t", Columns: []DataMetricFunctionColumn{{Name: "c", Type: "VARCHAR"}}},
		},
		Body: "SELECT COUNT_IF(c IS NULL) FROM arg_t",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `CREATE DATA METRIC FUNCTION "DB"."SC".NULL_COUNT(arg_t TABLE(c VARCHAR))` + "\n" +
		`  RETURNS NUMBER` + "\n" +
		`  AS` + "\n" +
		`$$` + "\n" +
		`SELECT COUNT_IF(c IS NULL) FROM arg_t` + "\n" +
		`$$;`
	if sql != want {
		t.Errorf("minimal mismatch:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildCreateDataMetricFunctionSql_Full(t *testing.T) {
	sql, err := BuildCreateDataMetricFunctionSql("DB", "SC", DataMetricFunctionConfig{
		Name:      "DUP_COUNT",
		OrReplace: true,
		Secure:    true,
		Args: []DataMetricFunctionTableArg{
			{Name: "t", Columns: []DataMetricFunctionColumn{
				{Name: "id", Type: "NUMBER"},
				{Name: "email", Type: "VARCHAR"},
			}},
		},
		NotNull: true,
		Comment: "counts duplicate ids",
		Body:    "SELECT COUNT(*) - COUNT(DISTINCT id) FROM t",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		`CREATE OR REPLACE SECURE DATA METRIC FUNCTION "DB"."SC".DUP_COUNT(t TABLE(id NUMBER, email VARCHAR))`,
		"\n  RETURNS NUMBER NOT NULL",
		"\n  COMMENT = 'counts duplicate ids'",
		"$$\nSELECT COUNT(*) - COUNT(DISTINCT id) FROM t\n$$;",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("full result missing %q:\n%s", want, sql)
		}
	}
}

func TestBuildCreateDataMetricFunctionSql_MultipleTableArgs(t *testing.T) {
	sql, err := BuildCreateDataMetricFunctionSql("DB", "SC", DataMetricFunctionConfig{
		Name: "CROSS_CHECK",
		Args: []DataMetricFunctionTableArg{
			{Name: "left_t", Columns: []DataMetricFunctionColumn{{Name: "id", Type: "NUMBER"}}},
			{Name: "right_t", Columns: []DataMetricFunctionColumn{{Name: "id", Type: "NUMBER"}, {Name: "v", Type: "VARCHAR"}}},
		},
		Body: "SELECT COUNT(*) FROM left_t",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `CREATE DATA METRIC FUNCTION "DB"."SC".CROSS_CHECK(left_t TABLE(id NUMBER), right_t TABLE(id NUMBER, v VARCHAR))`
	if !strings.Contains(sql, want) {
		t.Errorf("multi-arg result missing %q:\n%s", want, sql)
	}
}

func TestBuildCreateDataMetricFunctionSql_OrReplaceDropsIfNotExists(t *testing.T) {
	sql, err := BuildCreateDataMetricFunctionSql("DB", "SC", DataMetricFunctionConfig{
		Name:        "M",
		OrReplace:   true,
		IfNotExists: true,
		Args:        []DataMetricFunctionTableArg{{Name: "t", Columns: []DataMetricFunctionColumn{{Name: "c", Type: "NUMBER"}}}},
		Body:        "SELECT 1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(sql, "IF NOT EXISTS") {
		t.Errorf("OR REPLACE must drop IF NOT EXISTS, got: %s", sql)
	}
	if !strings.HasPrefix(sql, "CREATE OR REPLACE DATA METRIC FUNCTION") {
		t.Errorf("unexpected prefix: %s", sql)
	}
}

func TestBuildCreateDataMetricFunctionSql_IfNotExists(t *testing.T) {
	sql, err := BuildCreateDataMetricFunctionSql("DB", "SC", DataMetricFunctionConfig{
		Name:        "M",
		IfNotExists: true,
		Args:        []DataMetricFunctionTableArg{{Name: "t", Columns: []DataMetricFunctionColumn{{Name: "c", Type: "NUMBER"}}}},
		Body:        "SELECT 1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "DATA METRIC FUNCTION IF NOT EXISTS") {
		t.Errorf("expected IF NOT EXISTS, got: %s", sql)
	}
}

func TestBuildCreateDataMetricFunctionSql_Placeholders(t *testing.T) {
	sql, err := BuildCreateDataMetricFunctionSql("DB", "SC", DataMetricFunctionConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty config still yields a parseable preview: a placeholder name, default
	// argument name, a placeholder column, and a placeholder body.
	for _, want := range []string{
		"data_metric_function_name",
		"table_data TABLE(column_1 VARCHAR)",
		"RETURNS NUMBER",
		"SELECT COUNT(*) FROM table_data",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("placeholder result missing %q:\n%s", want, sql)
		}
	}
}
