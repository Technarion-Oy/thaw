// SPDX-License-Identifier: GPL-3.0-or-later

package pipe

import (
	"strings"
	"testing"
)

func TestParseCopyIntoTarget(t *testing.T) {
	tests := []struct {
		name       string
		ddl        string
		wantDB     string
		wantSchema string
		wantTable  string
		wantErr    bool
	}{
		// ── 3-part names ─────────────────────────────────────────────────────
		{
			name:       "3-part unquoted",
			ddl:        "create pipe foo as COPY INTO db.schema.table FROM @stage",
			wantDB:     "db",
			wantSchema: "schema",
			wantTable:  "table",
		},
		{
			name:       "3-part fully quoted",
			ddl:        `create pipe foo as COPY INTO "db"."schema"."table" FROM @stage`,
			wantDB:     "db",
			wantSchema: "schema",
			wantTable:  "table",
		},
		{
			name:       "3-part mixed quoting",
			ddl:        `COPY INTO "My DB".MY_SCHEMA."my table" FROM @stage`,
			wantDB:     "My DB",
			wantSchema: "MY_SCHEMA",
			wantTable:  "my table",
		},
		{
			name:       "3-part uppercase unquoted (Snowflake GET_DDL style)",
			ddl:        `COPY INTO LINEAGE_SOURCE_DB.RAW_DATA.MY_SNOWPIPE_TEST FROM @MY_STAGE`,
			wantDB:     "LINEAGE_SOURCE_DB",
			wantSchema: "RAW_DATA",
			wantTable:  "MY_SNOWPIPE_TEST",
		},

		// ── 2-part names ─────────────────────────────────────────────────────
		{
			name:       "2-part unquoted",
			ddl:        "COPY INTO schema.table FROM @stage",
			wantDB:     "",
			wantSchema: "schema",
			wantTable:  "table",
		},
		{
			name:       "2-part quoted",
			ddl:        `COPY INTO "MY SCHEMA"."my_tbl" FROM @stage`,
			wantDB:     "",
			wantSchema: "MY SCHEMA",
			wantTable:  "my_tbl",
		},

		// ── 1-part names ─────────────────────────────────────────────────────
		{
			name:       "1-part unquoted",
			ddl:        "COPY INTO mytable FROM @stage",
			wantDB:     "",
			wantSchema: "",
			wantTable:  "mytable",
		},
		{
			name:       "1-part quoted",
			ddl:        `COPY INTO "My Table" FROM @stage`,
			wantDB:     "",
			wantSchema: "",
			wantTable:  "My Table",
		},

		// ── Column-list suffix ────────────────────────────────────────────────
		{
			name:       "3-part unquoted with column list",
			ddl:        "COPY INTO db.schema.table(col1, col2) FROM @stage",
			wantDB:     "db",
			wantSchema: "schema",
			wantTable:  "table",
		},
		{
			name:       "3-part quoted with column list",
			ddl:        `COPY INTO "DB"."SCHEMA"."TBL"(col1, col2) FROM @stage`,
			wantDB:     "DB",
			wantSchema: "SCHEMA",
			wantTable:  "TBL",
		},
		{
			name:       "2-part with column list",
			ddl:        `COPY INTO schema.table(id, name) FROM @stage`,
			wantDB:     "",
			wantSchema: "schema",
			wantTable:  "table",
		},

		// ── Embedded double-quotes (escaped as "") ────────────────────────────
		{
			name:       "quoted ident with embedded quote in db part",
			ddl:        `COPY INTO "DB""x".schema.tbl FROM @stage`,
			wantDB:     `DB"x`,
			wantSchema: "schema",
			wantTable:  "tbl",
		},
		{
			name:       "quoted ident with embedded quote in table part",
			ddl:        `COPY INTO db.schema."tbl""y" FROM @stage`,
			wantDB:     "db",
			wantSchema: "schema",
			wantTable:  `tbl"y`,
		},

		// ── Whitespace / newlines ─────────────────────────────────────────────
		{
			name:       "newlines between keywords",
			ddl:        "create pipe p as\nCOPY INTO\n  db.schema.table\nFROM @stage",
			wantDB:     "db",
			wantSchema: "schema",
			wantTable:  "table",
		},
		{
			name:       "tabs and spaces after COPY INTO",
			ddl:        "COPY INTO\t  db.schema.table\t FROM @stage",
			wantDB:     "db",
			wantSchema: "schema",
			wantTable:  "table",
		},

		// ── Case-insensitive COPY INTO keyword ────────────────────────────────
		{
			name:       "lowercase copy into",
			ddl:        "copy into db.schema.table from @stage",
			wantDB:     "db",
			wantSchema: "schema",
			wantTable:  "table",
		},
		{
			name:       "mixed-case Copy Into",
			ddl:        "Copy Into db.schema.table from @stage",
			wantDB:     "db",
			wantSchema: "schema",
			wantTable:  "table",
		},

		// ── Realistic Snowflake pipe DDLs ─────────────────────────────────────
		{
			name: "realistic pipe DDL with quoted identifiers",
			ddl: `create or replace pipe "LINEAGE_SOURCE_DB"."RAW_DATA"."MY_TEST"
  auto_ingest = true
  as
copy into "LINEAGE_SOURCE_DB"."RAW_DATA"."MY_SNOWPIPE_TEST"
  from @"LINEAGE_SOURCE_DB"."RAW_DATA"."MY_STAGE"
  file_format = (type = 'CSV');`,
			wantDB:     "LINEAGE_SOURCE_DB",
			wantSchema: "RAW_DATA",
			wantTable:  "MY_SNOWPIPE_TEST",
		},
		{
			name: "realistic pipe DDL with unquoted identifiers",
			ddl: `CREATE OR REPLACE PIPE MYDB.MYSCHEMA.MYPIPE
  AUTO_INGEST = TRUE
  AS
COPY INTO MYDB.MYSCHEMA.MYTABLE
FROM @MYDB.MYSCHEMA.MYSTAGE
FILE_FORMAT = (type = 'JSON');`,
			wantDB:     "MYDB",
			wantSchema: "MYSCHEMA",
			wantTable:  "MYTABLE",
		},
		{
			name: "pipe DDL with AS on same line",
			ddl: `create pipe "DB"."SCHEMA"."PIPE" as copy into "DB"."SCHEMA"."TBL" from @stage;`,
			wantDB:     "DB",
			wantSchema: "SCHEMA",
			wantTable:  "TBL",
		},
		{
			name: "pipe DDL with semicolon-terminated table (no space before ;)",
			ddl:  `COPY INTO db.schema.table;`,
			wantDB:     "db",
			wantSchema: "schema",
			wantTable:  "table",
		},

		// ── GET_DDL lowercase normalization ───────────────────────────────────
		// Snowflake's GET_DDL sometimes returns unquoted identifiers in lowercase.
		// ParseCopyIntoTarget must return them as-is so the caller can uppercase
		// them (unquoted identifiers are stored uppercase in Snowflake).
		{
			name:       "unquoted lowercase identifiers (GET_DDL normalisation)",
			ddl:        "COPY INTO lineage_source_db.raw_data.my_snowpipe_test FROM @stage",
			wantDB:     "lineage_source_db",
			wantSchema: "raw_data",
			wantTable:  "my_snowpipe_test",
		},

		// ── Error cases ───────────────────────────────────────────────────────
		{
			name:    "no COPY INTO in DDL",
			ddl:     "create table foo (id int)",
			wantErr: true,
		},
		{
			name:    "COPY INTO with nothing after it",
			ddl:     "COPY INTO",
			wantErr: true,
		},
		{
			name:    "empty DDL",
			ddl:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, schema, table, err := ParseCopyIntoTarget(tt.ddl)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got db=%q schema=%q table=%q", db, schema, table)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if db != tt.wantDB {
				t.Errorf("db: got %q, want %q", db, tt.wantDB)
			}
			if schema != tt.wantSchema {
				t.Errorf("schema: got %q, want %q", schema, tt.wantSchema)
			}
			if table != tt.wantTable {
				t.Errorf("table: got %q, want %q", table, tt.wantTable)
			}
		})
	}
}

// TestParseCopyIntoTargetTokenBoundaries covers the cases where the previous
// substring search for "COPY INTO" picked the wrong occurrence or missed the
// real one.
func TestParseCopyIntoTargetTokenBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		ddl        string
		wantDB     string
		wantSchema string
		wantTable  string
	}{
		{
			// The COMMENT clause precedes AS COPY INTO in CREATE PIPE DDL, so
			// a comment mentioning "copy into" used to hijack the parse.
			name:      "copy into inside the COMMENT string literal",
			ddl:       `CREATE PIPE mypipe COMMENT='nightly copy into staging' AS COPY INTO real_target FROM @s`,
			wantTable: "real_target",
		},
		{
			name:      "copy into inside a line comment",
			ddl:       "CREATE PIPE p\n-- copy into old_target\nAS COPY INTO real_target FROM @s",
			wantTable: "real_target",
		},
		{
			name:      "copy into inside a block comment",
			ddl:       "CREATE PIPE p /* copy into old_target */ AS COPY INTO real_target FROM @s",
			wantTable: "real_target",
		},
		{
			name:      "newline between COPY and INTO",
			ddl:       "CREATE PIPE p AS COPY\nINTO t FROM @s",
			wantTable: "t",
		},
		{
			name:       "comment between the target parts",
			ddl:        "COPY INTO db. /* c */ sch.tbl FROM @s",
			wantDB:     "db",
			wantSchema: "sch",
			wantTable:  "tbl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, schema, table, err := ParseCopyIntoTarget(tt.ddl)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if db != tt.wantDB || schema != tt.wantSchema || table != tt.wantTable {
				t.Errorf("got (%q, %q, %q), want (%q, %q, %q)",
					db, schema, table, tt.wantDB, tt.wantSchema, tt.wantTable)
			}
		})
	}
}

// TestParseCopyIntoTargetParts verifies that quoting metadata is preserved so
// the caller can uppercase unquoted parts for canonical Snowflake resolution.
func TestParseCopyIntoTargetParts(t *testing.T) {
	tests := []struct {
		name        string
		ddl         string
		wantValues  []string // one entry per part
		wantQuoted  []bool
		wantErr     bool
	}{
		{
			name:       "3-part all quoted",
			ddl:        `COPY INTO "DB"."SCHEMA"."TABLE" FROM @stage`,
			wantValues: []string{"DB", "SCHEMA", "TABLE"},
			wantQuoted: []bool{true, true, true},
		},
		{
			name:       "3-part all unquoted uppercase",
			ddl:        `COPY INTO DB.SCHEMA.TABLE FROM @stage`,
			wantValues: []string{"DB", "SCHEMA", "TABLE"},
			wantQuoted: []bool{false, false, false},
		},
		{
			// GET_DDL may return unquoted identifiers in lowercase even though
			// Snowflake stores them as uppercase.  The Quoted flag must be false
			// so the caller knows to uppercase before resolution.
			name:       "3-part unquoted lowercase (GET_DDL normalisation)",
			ddl:        `COPY INTO lineage_source_db.raw_data.my_snowpipe_test FROM @stage`,
			wantValues: []string{"lineage_source_db", "raw_data", "my_snowpipe_test"},
			wantQuoted: []bool{false, false, false},
		},
		{
			name:       "mixed quoting",
			ddl:        `COPY INTO "My DB".MY_SCHEMA."my tbl" FROM @stage`,
			wantValues: []string{"My DB", "MY_SCHEMA", "my tbl"},
			wantQuoted: []bool{true, false, true},
		},
		{
			name:       "1-part quoted",
			ddl:        `COPY INTO "TBL" FROM @stage`,
			wantValues: []string{"TBL"},
			wantQuoted: []bool{true},
		},
		{
			name:    "no COPY INTO",
			ddl:     "create table foo (id int)",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, err := ParseCopyIntoTargetParts(tt.ddl)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got parts %v", parts)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(parts) != len(tt.wantValues) {
				t.Fatalf("got %d parts, want %d", len(parts), len(tt.wantValues))
			}
			for i, p := range parts {
				if p.Value != tt.wantValues[i] {
					t.Errorf("parts[%d].Value: got %q, want %q", i, p.Value, tt.wantValues[i])
				}
				if p.Quoted != tt.wantQuoted[i] {
					t.Errorf("parts[%d].Quoted: got %v, want %v", i, p.Quoted, tt.wantQuoted[i])
				}
			}
		})
	}
}

// TestTableNameArgBuilding simulates the logic in GetPipeCopyHistory that
// converts parsed FQN parts into the TABLE_NAME string passed to copy_history.
// Unquoted parts must be uppercased so that GET_DDL's lowercase normalisation
// does not produce a case-sensitive mismatch against the actual table name.
func TestTableNameArgBuilding(t *testing.T) {
	build := func(ddl string) (string, error) {
		parts, err := ParseCopyIntoTargetParts(ddl)
		if err != nil {
			return "", err
		}
		quotedParts := make([]string, len(parts))
		for i, p := range parts {
			val := p.Value
			if !p.Quoted {
				val = strings.ToUpper(val)
			}
			quotedParts[i] = `"` + strings.ReplaceAll(val, `"`, `""`) + `"`
		}
		return strings.Join(quotedParts, "."), nil
	}

	tests := []struct {
		name    string
		ddl     string
		wantArg string
	}{
		{
			// Realistic GET_DDL output with unquoted lowercase identifiers.
			name:    "unquoted lowercase → uppercased",
			ddl:     `COPY INTO lineage_source_db.raw_data.my_snowpipe_test FROM @stage`,
			wantArg: `"LINEAGE_SOURCE_DB"."RAW_DATA"."MY_SNOWPIPE_TEST"`,
		},
		{
			// Quoted identifiers must preserve case exactly.
			name:    "quoted mixed-case preserved",
			ddl:     `COPY INTO "My DB"."raw_data"."My Table" FROM @stage`,
			wantArg: `"My DB"."raw_data"."My Table"`,
		},
		{
			// Standard GET_DDL output with quoted uppercase identifiers.
			name:    "quoted uppercase preserved",
			ddl:     `copy into "LINEAGE_SOURCE_DB"."RAW_DATA"."MY_SNOWPIPE_TEST" from @stage`,
			wantArg: `"LINEAGE_SOURCE_DB"."RAW_DATA"."MY_SNOWPIPE_TEST"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := build(tt.ddl)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantArg {
				t.Errorf("got %q, want %q", got, tt.wantArg)
			}
		})
	}
}
