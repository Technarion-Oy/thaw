// SPDX-License-Identifier: GPL-3.0-or-later

package pipe

import (
	"strings"
	"testing"
)

func TestValidateCopyStatement(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string // substring; empty means success
	}{
		{
			name:  "simple valid",
			input: "COPY INTO my_table FROM @my_stage",
		},
		{
			name:  "lowercase copy into",
			input: "copy into db.schema.table from @stage/path",
		},
		{
			name:  "valid with options",
			input: "COPY INTO my_table FROM @my_stage FILE_FORMAT = (TYPE = CSV)",
		},
		{
			name:  "trailing semicolon treated as single statement",
			input: "COPY INTO my_table FROM @my_stage;",
		},
		{
			name:    "stacked queries",
			input:   "COPY INTO my_table FROM @my_stage; DROP TABLE users",
			wantErr: "exactly one SQL statement",
		},
		{
			name:    "stacked via comment-hidden semicolon still two statements",
			input:   "COPY INTO my_table FROM @my_stage; -- safe\nSELECT 1",
			wantErr: "exactly one SQL statement",
		},
		{
			name:    "empty after trim",
			input:   "   ",
			wantErr: "exactly one SQL statement",
		},
		{
			name:    "wrong statement type",
			input:   "SELECT 1",
			wantErr: "must start with COPY INTO",
		},
		{
			name:    "drop table injection",
			input:   "DROP TABLE users",
			wantErr: "must start with COPY INTO",
		},
		{
			name:  "semicolon inside string is not a statement separator",
			input: "COPY INTO t FROM @s FILE_FORMAT = (FIELD_DELIMITER = ';')",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateCopyStatement(tc.input)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (result %q)", tc.wantErr, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(strings.ToUpper(got), "COPY INTO ") {
				t.Fatalf("returned statement does not start with COPY INTO: %q", got)
			}
		})
	}
}

func TestBuildCreatePipeSqlValidation(t *testing.T) {
	t.Run("rejects stacked queries in CopyStatement", func(t *testing.T) {
		_, err := BuildCreatePipeSql("DB", "SCHEMA", PipeConfig{
			Name:          "my_pipe",
			CopyStatement: "COPY INTO t FROM @s; DROP TABLE users",
		})
		if err == nil {
			t.Fatal("expected error for stacked query injection, got nil")
		}
	})

	t.Run("rejects non-COPY statement", func(t *testing.T) {
		_, err := BuildCreatePipeSql("DB", "SCHEMA", PipeConfig{
			Name:          "my_pipe",
			CopyStatement: "SELECT * FROM sensitive",
		})
		if err == nil {
			t.Fatal("expected error for non-COPY statement, got nil")
		}
	})

	t.Run("accepts valid COPY INTO", func(t *testing.T) {
		sql, err := BuildCreatePipeSql("DB", "SCHEMA", PipeConfig{
			Name:          "my_pipe",
			CopyStatement: "COPY INTO my_table FROM @my_stage",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(sql, "COPY INTO my_table FROM @my_stage") {
			t.Fatalf("COPY statement not found in output: %q", sql)
		}
	})

	t.Run("empty CopyStatement uses placeholder without error", func(t *testing.T) {
		sql, err := BuildCreatePipeSql("DB", "SCHEMA", PipeConfig{Name: "my_pipe"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(sql, "COPY INTO <table>") {
			t.Fatalf("placeholder not found in output: %q", sql)
		}
	})
}
