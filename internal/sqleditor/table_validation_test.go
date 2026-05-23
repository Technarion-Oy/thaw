package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_CreateIcebergTable(t *testing.T) {
	validCases := []string{
		// Snowflake-managed
		"CREATE ICEBERG TABLE t1 (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://my-snowflake-bucket/'",
		"CREATE ICEBERG TABLE t1 (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://another-bucket/' CLUSTER BY (id) DATA_RETENTION_TIME_IN_DAYS = 1",
		"CREATE ICEBERG TABLE t1 (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://comment-bucket/' COMMENT = 'CLUSTER BY is a table property'",
		"CREATE ICEBERG TABLE t1 (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://transient-comment/' COMMENT = 'TRANSIENT tables are not supported'",
		"CREATE ICEBERG TABLE t1 (id int) EXTERNAL_VOLUME = 'my_ev' CATALOG = 'my_cat' BASE_LOCATION = 's3://external-bucket/'",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'loc' COMMENT = 'CLUSTER BY is not applicable'",
		"CREATE ICEBERG TABLE transient (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/'",
		"CREATE ICEBERG TABLE t (id int) CATALOG = 'snowflake' BASE_LOCATION = 's3://test/'",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = my_ev CATALOG = 'my_cat' BASE_LOCATION = 's3://bucket/'",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 40)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected 0 warnings, got %d: %v", len(warns), warns)
			}
		})
	}

	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{"OR REPLACE IF NOT EXISTS Iceberg", "CREATE OR REPLACE ICEBERG TABLE IF NOT EXISTS t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/'", []string{"Conflict between OR REPLACE and IF NOT EXISTS"}},
		{"Transient keyword used", "CREATE TRANSIENT ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/'", []string{"TRANSIENT is not supported for Iceberg tables."}},
		{"Missing BASE_LOCATION for Snowflake-managed", "CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE'", []string{"BASE_LOCATION is mandatory for all Iceberg tables and cannot be empty."}},
		{"Empty BASE_LOCATION for Snowflake-managed", "CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = ''", []string{"BASE_LOCATION is mandatory for all Iceberg tables and cannot be empty."}},
		{"Missing EXTERNAL_VOLUME", "CREATE ICEBERG TABLE t (id int) CATALOG = 'c' BASE_LOCATION = 'l'", []string{"EXTERNAL_VOLUME is mandatory for Iceberg tables with external catalogs."}},
		{"Missing CATALOG", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' BASE_LOCATION = 'l'", []string{"CATALOG is mandatory for Iceberg tables with external catalogs."}},
		{"Empty EXTERNAL_VOLUME", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = '' CATALOG = 'c' BASE_LOCATION = 'l'", []string{"EXTERNAL_VOLUME is mandatory for Iceberg tables with external catalogs."}},
		{"Empty CATALOG", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = '' BASE_LOCATION = 'l'", []string{"CATALOG is mandatory for Iceberg tables with external catalogs."}},
		{"CATALOG_TABLE_NAME with SNOWFLAKE", "CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/' CATALOG_TABLE_NAME = 'ctn'", []string{"CATALOG_TABLE_NAME is only valid when CATALOG is not 'SNOWFLAKE'"}},
		{"CATALOG_NAMESPACE with SNOWFLAKE", "CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/' CATALOG_NAMESPACE = 'cns'", []string{"CATALOG_NAMESPACE is only valid when CATALOG is not 'SNOWFLAKE'"}},
		{"OR REPLACE with external catalog", "CREATE OR REPLACE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l'", []string{"OR REPLACE is not supported for Iceberg tables backed by external catalogs."}},
		{"CLUSTER BY with external catalog", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' CLUSTER BY (id)", []string{"CLUSTER BY is supported only for Snowflake-managed Iceberg tables."}},
		{"DATA_RETENTION_TIME_IN_DAYS with external catalog", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' DATA_RETENTION_TIME_IN_DAYS = 1", []string{"DATA_RETENTION_TIME_IN_DAYS applies only to Snowflake-managed Iceberg tables."}},
		{"Invalid REFRESH_MODE", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' REFRESH_MODE = 'INVALID'", []string{"Invalid REFRESH_MODE value. Must be AUTO, FULL, or INCREMENTAL."}},
		{"Invalid INITIALIZE", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' INITIALIZE = 'INVALID'", []string{"Invalid INITIALIZE value. Must be ON_CREATE or ON_SCHEDULE."}},
		{"Invalid AUTO_REFRESH", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' AUTO_REFRESH = 'INVALID'", []string{"AUTO_REFRESH must be TRUE or FALSE."}},
		{"Quoted AUTO_REFRESH Invalid", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'cat' BASE_LOCATION = 'loc' AUTO_REFRESH = 'BAD'", []string{"AUTO_REFRESH must be TRUE or FALSE."}},
		{"Quoted REPLACE_INVALID_CHARACTERS Invalid", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'cat' BASE_LOCATION = 'loc' REPLACE_INVALID_CHARACTERS = 'BAD'", []string{"REPLACE_INVALID_CHARACTERS must be TRUE or FALSE."}},
		{"OR REPLACE IF NOT EXISTS External Catalog", "CREATE OR REPLACE ICEBERG TABLE IF NOT EXISTS t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l'", []string{"Conflict between OR REPLACE and IF NOT EXISTS", "OR REPLACE is not supported for Iceberg tables backed by external catalogs."}},
		{"CREATE OR REPLACE TRANSIENT ICEBERG TABLE", "CREATE OR REPLACE TRANSIENT ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l'", []string{"TRANSIENT is not supported for Iceberg tables.", "OR REPLACE is not supported for Iceberg tables backed by external catalogs."}},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			for _, wantMsg := range tt.wantMsgs {
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, wantMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning for %q, but got none matching %q. Warnings: %v", tt.sql, wantMsg, warns)
				}
			}
			if len(warns) < len(tt.wantMsgs) {
				t.Errorf("Expected %d warnings, got %d for %q", len(tt.wantMsgs), len(warns), tt.sql)
			}
		})
	}
}

func TestValidateSnowflakePatterns_CreateHybridTable(t *testing.T) {
	validCases := []string{
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY NOT NULL)",
		"CREATE HYBRID TABLE t1 (id INT NOT NULL PRIMARY KEY, val VARCHAR INDEX idx_val (val))",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY NOT NULL, val VARCHAR) COMMENT = 'test'",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY NOT NULL, c2 INT, CONSTRAINT fk_c2 FOREIGN KEY (c2) REFERENCES t2(id))",
		"CREATE HYBRID TABLE t1 (id INT NOT NULL, val VARCHAR, PRIMARY KEY (id))",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY NOT NULL, val VARCHAR NOT NULL)",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY NOT NULL) COMMENT = 'no cluster by here'",
		"CREATE TABLE t1 (id INT, val VARCHAR DEFAULT 'INDEX is not supported here')",
		"CREATE HYBRID TABLE t1 (id INT NOT NULL, val VARCHAR DEFAULT 'PRIMARY KEY', PRIMARY KEY (id))",
		"CREATE HYBRID TABLE IF NOT EXISTS t1 (id INT PRIMARY KEY NOT NULL)",
		"CREATE HYBRID TABLE t1 (id INT NOT NULL, CONSTRAINT pk1 PRIMARY KEY (id))",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY AUTOINCREMENT)",
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY IDENTITY (1, 1))",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 40)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected 0 warnings, got %d: %v", len(warns), warns)
			}
		})
	}

	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{"Missing Primary Key", "CREATE HYBRID TABLE t1 (id INT)", []string{"Hybrid tables must have a PRIMARY KEY"}},
		{"Cluster By not supported", "CREATE HYBRID TABLE t1 (id INT PRIMARY KEY) CLUSTER BY (id)", []string{"CLUSTER BY is not supported on hybrid tables"}},
		{"Data Retention not supported", "CREATE HYBRID TABLE t1 (id INT PRIMARY KEY) DATA_RETENTION_TIME_IN_DAYS = 7", []string{"DATA_RETENTION_TIME_IN_DAYS is not applicable to hybrid tables"}},
		{"Change Tracking not supported", "CREATE HYBRID TABLE t1 (id INT PRIMARY KEY) CHANGE_TRACKING = TRUE", []string{"CHANGE_TRACKING is not supported on hybrid tables"}},
		{"Transient not supported", "CREATE TRANSIENT HYBRID TABLE t1 (id INT PRIMARY KEY)", []string{"TRANSIENT is not supported for hybrid tables"}},
		{"TRANSIENT + missing PK", "CREATE TRANSIENT HYBRID TABLE t1 (id INT)", []string{"TRANSIENT is not supported for hybrid tables", "Hybrid tables must have a PRIMARY KEY"}},
		{"OR REPLACE not supported", "CREATE OR REPLACE HYBRID TABLE t1 (id INT PRIMARY KEY)", []string{"OR REPLACE is not supported for hybrid tables"}},
		{"COPY GRANTS not supported", "CREATE HYBRID TABLE t1 (id INT PRIMARY KEY) COPY GRANTS", []string{"COPY GRANTS is not supported on hybrid tables"}},
		{"Index on regular table", "CREATE TABLE t1 (id INT PRIMARY KEY, val VARCHAR INDEX idx_val (val))", []string{"Secondary indexes (INDEX) are only supported on hybrid tables"}},
		{"PK column missing NOT NULL (out of line)", "CREATE HYBRID TABLE t1 (id INT, PRIMARY KEY (id))", []string{"Primary key columns in a hybrid table must be NOT NULL"}},
		{"PK column missing NOT NULL (inline)", "CREATE HYBRID TABLE t1 (id INT PRIMARY KEY)", []string{"Primary key columns in a hybrid table must be NOT NULL"}},
		{"PK column missing NOT NULL (out of line, extra spaces)", "CREATE HYBRID TABLE t1 (id INT, PRIMARY  KEY  (id))", []string{"Primary key columns in a hybrid table must be NOT NULL (column 'ID' omits it)."}},
		{"Composite PK missing NOT NULL on one column", "CREATE HYBRID TABLE t1 (id INT NOT NULL, name INT, PRIMARY KEY (id, name))", []string{"Primary key columns in a hybrid table must be NOT NULL (column 'NAME' omits it)."}},
		{"Constraint-named PK missing NOT NULL", "CREATE HYBRID TABLE t1 (id INT, CONSTRAINT pk1 PRIMARY KEY (id))", []string{"Primary key columns in a hybrid table must be NOT NULL"}},
		{"string literal containing NOT NULL suppresses false negative", "CREATE HYBRID TABLE t1 (id INT DEFAULT 'NOT NULL here', PRIMARY KEY (id))", []string{"Primary key columns in a hybrid table must be NOT NULL"}},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			for _, wantMsg := range tt.wantMsgs {
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, wantMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning for %q, but got none matching %q. Warnings: %v", tt.sql, wantMsg, warns)
				}
			}
		})
	}
}


func TestValidateSnowflakePatterns_CreateEventTable(t *testing.T) {
	validCases := []string{
		// Minimal valid event table
		"CREATE EVENT TABLE my_events",
		// With OR REPLACE
		"CREATE OR REPLACE EVENT TABLE my_events",
		// With IF NOT EXISTS
		"CREATE EVENT TABLE IF NOT EXISTS my_events",
		// With COMMENT
		"CREATE EVENT TABLE my_events COMMENT = 'telemetry data'",
		// With DATA_RETENTION_TIME_IN_DAYS
		"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = 30",
		// With MAX_DATA_EXTENSION_TIME_IN_DAYS
		"CREATE EVENT TABLE my_events MAX_DATA_EXTENSION_TIME_IN_DAYS = 14",
		// With CHANGE_TRACKING = TRUE
		"CREATE EVENT TABLE my_events CHANGE_TRACKING = TRUE",
		// With CHANGE_TRACKING = FALSE
		"CREATE EVENT TABLE my_events CHANGE_TRACKING = FALSE",
		// With DEFAULT_DDL_COLLATION
		"CREATE EVENT TABLE my_events DEFAULT_DDL_COLLATION = 'en-ci'",
		// With COPY GRANTS
		"CREATE OR REPLACE EVENT TABLE my_events COPY GRANTS",
		// Multiple properties
		"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = 7 MAX_DATA_EXTENSION_TIME_IN_DAYS = 14 CHANGE_TRACKING = TRUE COMMENT = 'logs'",
		// Schema-qualified name
		"CREATE EVENT TABLE my_db.my_schema.my_events",
		// Zero retention
		"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = 0",
		// Zero extension
		"CREATE EVENT TABLE my_events MAX_DATA_EXTENSION_TIME_IN_DAYS = 0",
		// CLUSTER BY inside COMMENT string must not trigger false positive
		"CREATE EVENT TABLE my_events COMMENT = 'has CLUSTER BY inside'",
		// CLUSTER BY inside a line comment must not trigger false positive
		"CREATE EVENT TABLE my_events\n-- CLUSTER BY (ts)\nCOMMENT = 'test'",
		// Keywords inside a block comment must not trigger false positive
		"CREATE EVENT TABLE my_events /* AUTO_REFRESH = TRUE */ COMMENT = 'test'",
		// TAG property
		"CREATE EVENT TABLE my_events TAG (cost_center = 'finance')",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			if warns := getWarnings(markers); len(warns) > 0 {
				t.Errorf("Expected 0 warnings, got %d: %v", len(warns), warns)
			}
		})
	}

	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"OR REPLACE and IF NOT EXISTS conflict",
			"CREATE OR REPLACE EVENT TABLE IF NOT EXISTS my_events",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		{
			"Column definitions not allowed",
			"CREATE EVENT TABLE my_events (col1 VARCHAR, col2 INT)",
			[]string{"Event tables have a fixed schema and do not support column definitions"},
		},
		{
			"CLUSTER BY not supported",
			"CREATE EVENT TABLE my_events CLUSTER BY (timestamp)",
			[]string{"CLUSTER BY is not supported for EVENT TABLE"},
		},
		{
			"Invalid DATA_RETENTION_TIME_IN_DAYS",
			"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = abc",
			[]string{"DATA_RETENTION_TIME_IN_DAYS must be a non-negative integer"},
		},
		{
			"Negative DATA_RETENTION_TIME_IN_DAYS",
			"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = -1",
			[]string{"DATA_RETENTION_TIME_IN_DAYS must be a non-negative integer"},
		},
		{
			"Invalid MAX_DATA_EXTENSION_TIME_IN_DAYS",
			"CREATE EVENT TABLE my_events MAX_DATA_EXTENSION_TIME_IN_DAYS = xyz",
			[]string{"MAX_DATA_EXTENSION_TIME_IN_DAYS must be a non-negative integer"},
		},
		{
			"Negative MAX_DATA_EXTENSION_TIME_IN_DAYS",
			"CREATE EVENT TABLE my_events MAX_DATA_EXTENSION_TIME_IN_DAYS = -1",
			[]string{"MAX_DATA_EXTENSION_TIME_IN_DAYS must be a non-negative integer"},
		},
		{
			"Invalid CHANGE_TRACKING value",
			"CREATE EVENT TABLE my_events CHANGE_TRACKING = MAYBE",
			[]string{"CHANGE_TRACKING must be TRUE or FALSE"},
		},
		{
			"Unexpected property AUTO_REFRESH",
			"CREATE EVENT TABLE my_events AUTO_REFRESH = TRUE",
			[]string{"Unexpected property 'AUTO_REFRESH'"},
		},
		{
			"Missing name",
			"CREATE EVENT TABLE",
			[]string{"Unexpected syntax in CREATE EVENT TABLE"},
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warns := getWarnings(markers)

			for _, wantMsg := range tt.wantMsgs {
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, wantMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning for %q, but got none matching %q. Warnings: %v", tt.sql, wantMsg, warns)
				}
			}
		})
	}

	// Verify that the OR REPLACE + IF NOT EXISTS conflict triggers exactly one
	// warning (proving the early return works and no additional checks run).
	t.Run("OR REPLACE and IF NOT EXISTS emits exactly one marker", func(t *testing.T) {
		sql := "CREATE OR REPLACE EVENT TABLE IF NOT EXISTS my_events AUTO_REFRESH = TRUE"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning (early return), got %d: %v", len(warns), warns)
		}
	})
}

// ── ALTER SESSION SET / UNSET ─────────────────────────────────────────────────


func TestValidateSnowflakePatterns_AlterTableSearchOptimization(t *testing.T) {
	t.Run("valid ALTER TABLE SEARCH OPTIMIZATION", func(t *testing.T) {
		validQueries := []string{
			// Bare ADD SEARCH OPTIMIZATION (no ON clause)
			"ALTER TABLE my_table ADD SEARCH OPTIMIZATION",
			"ALTER TABLE db.schema.my_table ADD SEARCH OPTIMIZATION",
			// IF EXISTS form
			"ALTER TABLE IF EXISTS my_table ADD SEARCH OPTIMIZATION",
			"ALTER TABLE IF EXISTS my_table ADD SEARCH OPTIMIZATION ON EQUALITY(c1)",
			// Bare DROP SEARCH OPTIMIZATION
			"ALTER TABLE my_table DROP SEARCH OPTIMIZATION",
			"ALTER TABLE db.schema.my_table DROP SEARCH OPTIMIZATION",
			// ON clause with EQUALITY
			"ALTER TABLE my_table ADD SEARCH OPTIMIZATION ON EQUALITY(col1)",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(col1, col2)",
			// ON clause with SUBSTRING
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON SUBSTRING(col1)",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON SUBSTRING(col1, col2)",
			// ON clause with GEO
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON GEO(geo_col)",
			// ON clause with FULL_TEXT
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON FULL_TEXT(col1)",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON FULL_TEXT(col1, col2)",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON FULL_TEXT(col1, LANGUAGE => 'en')",
			// Multiple expression types
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(c1), SUBSTRING(c2)",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(c1), SUBSTRING(c2), GEO(c3), FULL_TEXT(c4)",
			// DROP with ON clause
			"ALTER TABLE t DROP SEARCH OPTIMIZATION ON EQUALITY(col1)",
			"ALTER TABLE t DROP SEARCH OPTIMIZATION ON EQUALITY(c1), SUBSTRING(c2)",
			// Case insensitive
			"ALTER TABLE t ADD search optimization ON equality(c1)",
			"alter table t add search optimization on substring(c1), geo(c2)",
			// With trailing semicolons / whitespace
			"ALTER TABLE t ADD SEARCH OPTIMIZATION;",
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(c1);",
			"ALTER TABLE t DROP SEARCH OPTIMIZATION ON SUBSTRING(c1), GEO(c2);  ",
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid ALTER TABLE SEARCH OPTIMIZATION", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// Unknown expression type
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON FUZZY(col1)",
				wantMsg: "Unknown search optimization type",
			},
			// Another unknown type
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON HASH(col1)",
				wantMsg: "Unknown search optimization type",
			},
			// Empty ON clause
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON",
				wantMsg: "SEARCH OPTIMIZATION ON requires at least one expression",
			},
			// Mixed valid and invalid
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(c1), FUZZY(c2)",
				wantMsg: "Unknown search optimization type",
			},
			// DROP with unknown expression type
			{
				sql:     "ALTER TABLE t DROP SEARCH OPTIMIZATION ON UNKNOWN(col1)",
				wantMsg: "Unknown search optimization type",
			},
			// IF EXISTS with unknown expression type
			{
				sql:     "ALTER TABLE IF EXISTS t ADD SEARCH OPTIMIZATION ON FUZZY(col1)",
				wantMsg: "Unknown search optimization type",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

func TestValidateSnowflakePatterns_AlterDynamicTable(t *testing.T) {
	t.Run("valid ALTER DYNAMIC TABLE", func(t *testing.T) {
		validQueries := []string{
			// REFRESH
			"ALTER DYNAMIC TABLE my_dt REFRESH",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt REFRESH",
			"ALTER DYNAMIC TABLE db.schema.my_dt REFRESH",
			// SUSPEND
			"ALTER DYNAMIC TABLE my_dt SUSPEND",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt SUSPEND",
			// RESUME
			"ALTER DYNAMIC TABLE my_dt RESUME",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt RESUME",
			// SET TARGET_LAG
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 minute'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '5 minutes'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 hour'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '2 hours'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '30 seconds'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 day'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '7 days'",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = DOWNSTREAM",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt SET TARGET_LAG = DOWNSTREAM",
			// SET WAREHOUSE
			"ALTER DYNAMIC TABLE my_dt SET WAREHOUSE = my_wh",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt SET WAREHOUSE = my_wh",
			// SET COMMENT (string literal stripped by preprocessing — exercises non-TARGET_LAG SET path)
			"ALTER DYNAMIC TABLE my_dt SET COMMENT = 'hello world'",
			// UNSET
			"ALTER DYNAMIC TABLE my_dt UNSET DATA_RETENTION_TIME_IN_DAYS",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt UNSET DATA_RETENTION_TIME_IN_DAYS",
			// SWAP WITH
			"ALTER DYNAMIC TABLE my_dt SWAP WITH other_dt",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt SWAP WITH db.schema.other_dt",
			// RENAME TO
			"ALTER DYNAMIC TABLE my_dt RENAME TO new_name",
			"ALTER DYNAMIC TABLE IF EXISTS my_dt RENAME TO db.schema.new_name",
			// Case insensitive
			"alter dynamic table my_dt refresh",
			"ALTER DYNAMIC TABLE my_dt set target_lag = downstream",
			"Alter Dynamic Table my_dt Suspend",
			// Table name collides with a sub-command keyword — must not false-positive
			"ALTER DYNAMIC TABLE suspend SET TARGET_LAG = DOWNSTREAM",
			"ALTER DYNAMIC TABLE resume SET WAREHOUSE = my_wh",
			"ALTER DYNAMIC TABLE refresh SUSPEND",
			"ALTER DYNAMIC TABLE set RESUME",
			// With trailing semicolons / whitespace
			"ALTER DYNAMIC TABLE my_dt REFRESH;",
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 minute';  ",
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid ALTER DYNAMIC TABLE", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// Missing table name
			{
				sql:     "ALTER DYNAMIC TABLE",
				wantMsg: "ALTER DYNAMIC TABLE requires a table name",
			},
			// Unknown sub-command
			{
				sql:     "ALTER DYNAMIC TABLE my_dt TRUNCATE",
				wantMsg: "Unknown ALTER DYNAMIC TABLE sub-command",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt DROP",
				wantMsg: "Unknown ALTER DYNAMIC TABLE sub-command",
			},
			// SWAP WITH without target name
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SWAP WITH",
				wantMsg: "SWAP WITH requires a target table name",
			},
			// RENAME TO without new name
			{
				sql:     "ALTER DYNAMIC TABLE my_dt RENAME TO",
				wantMsg: "RENAME TO requires a new table name",
			},
			// Multiple sub-commands in one statement
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SUSPEND RESUME",
				wantMsg: "ALTER DYNAMIC TABLE supports only one sub-command per statement",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt REFRESH SUSPEND",
				wantMsg: "ALTER DYNAMIC TABLE supports only one sub-command per statement",
			},
			// Invalid TARGET_LAG value
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = 'invalid'",
				wantMsg: "Invalid TARGET_LAG value",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 fortnight'",
				wantMsg: "Invalid TARGET_LAG value",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = 42",
				wantMsg: "Invalid TARGET_LAG value",
			},
			// Zero-duration TARGET_LAG (Snowflake requires positive integer)
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '0 seconds'",
				wantMsg: "Invalid TARGET_LAG value",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '0 minutes'",
				wantMsg: "Invalid TARGET_LAG value",
			},
			// Bare SET / UNSET without a property name
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET",
				wantMsg: "SET requires at least one property",
			},
			{
				sql:     "ALTER DYNAMIC TABLE my_dt UNSET",
				wantMsg: "UNSET requires at least one property name",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

// ── ALTER TABLE … SWAP WITH Tests ────────────────────────────────────────────

func TestValidateSnowflakePatterns_AlterTableSwapWith(t *testing.T) {
	t.Run("valid ALTER TABLE SWAP WITH", func(t *testing.T) {
		validQueries := []string{
			// Basic
			"ALTER TABLE orders SWAP WITH orders_backup",
			"ALTER TABLE t1 SWAP WITH t2",
			// IF EXISTS
			"ALTER TABLE IF EXISTS t1 SWAP WITH t2",
			"ALTER TABLE IF EXISTS orders SWAP WITH orders_backup",
			// Three-part names
			"ALTER TABLE db1.schema1.t1 SWAP WITH db1.schema1.t2",
			"ALTER TABLE mydb.public.orders SWAP WITH mydb.public.orders_backup",
			// Two-part names
			"ALTER TABLE schema1.t1 SWAP WITH schema1.t2",
			// Mixed part counts
			"ALTER TABLE db.schema.t1 SWAP WITH t2",
			"ALTER TABLE t1 SWAP WITH db.schema.t2",
			// IF EXISTS with multi-part names
			"ALTER TABLE IF EXISTS db.schema.t1 SWAP WITH db.schema.t2",
			// Quoted identifiers
			`ALTER TABLE "MY_TABLE" SWAP WITH "OTHER_TABLE"`,
			`ALTER TABLE "my table" SWAP WITH "other table"`,
			`ALTER TABLE db."SCHEMA"."TABLE" SWAP WITH db."SCHEMA"."OTHER"`,
			// Case insensitive
			"alter table t1 swap with t2",
			"Alter Table T1 Swap With T2",
			"ALTER TABLE t1 swap WITH t2",
			// With trailing semicolons / whitespace
			"ALTER TABLE t1 SWAP WITH t2;",
			"ALTER TABLE t1 SWAP WITH t2;  ",
			"ALTER TABLE t1 SWAP WITH t2 ;",
			// Table name collides with a keyword
			"ALTER TABLE swap SWAP WITH other_t",
			`ALTER TABLE "select" SWAP WITH "from"`,
		}
		for _, sql := range validQueries {
			t.Run(sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid ALTER TABLE SWAP WITH", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// Missing target table name
			{
				sql:     "ALTER TABLE orders SWAP WITH",
				wantMsg: "SWAP WITH requires a target table name",
			},
			{
				sql:     "ALTER TABLE IF EXISTS orders SWAP WITH",
				wantMsg: "SWAP WITH requires a target table name",
			},
			{
				sql:     "ALTER TABLE db.schema.t1 SWAP WITH",
				wantMsg: "SWAP WITH requires a target table name",
			},
			// Same table (no-op)
			{
				sql:     "ALTER TABLE orders SWAP WITH orders",
				wantMsg: "SWAP WITH the same table",
			},
			{
				sql:     "ALTER TABLE t1 SWAP WITH t1",
				wantMsg: "SWAP WITH the same table",
			},
			// Extra clause after target
			{
				sql:     "ALTER TABLE orders SWAP WITH backup CLUSTER BY (id)",
				wantMsg: "Unexpected clause after SWAP WITH target table",
			},
			{
				sql:     "ALTER TABLE orders SWAP WITH backup SET DATA_RETENTION_TIME_IN_DAYS = 1",
				wantMsg: "Unexpected clause after SWAP WITH target table",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql, func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

