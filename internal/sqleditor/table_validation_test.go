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
		// Valid property values must not produce false-positive warnings
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' REFRESH_MODE = 'AUTO'",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' REFRESH_MODE = 'FULL'",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' REFRESH_MODE = 'INCREMENTAL'",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' INITIALIZE = 'ON_CREATE'",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' INITIALIZE = 'ON_SCHEDULE'",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' AUTO_REFRESH = TRUE",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' AUTO_REFRESH = FALSE",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' REPLACE_INVALID_CHARACTERS = TRUE",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' REPLACE_INVALID_CHARACTERS = FALSE",
		// OR REPLACE valid for Snowflake-managed
		"CREATE OR REPLACE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/'",
		// IF NOT EXISTS valid (without OR REPLACE)
		"CREATE ICEBERG TABLE IF NOT EXISTS t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/'",
		// Unquoted bare enum values are accepted (isValidEnumValue strips quotes)
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' REFRESH_MODE = AUTO",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' INITIALIZE = ON_CREATE",
		// CATALOG_TABLE_NAME and CATALOG_NAMESPACE are valid with non-Snowflake catalogs
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' CATALOG_TABLE_NAME = 'ctn'",
		"CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' CATALOG_NAMESPACE = 'cns'",
		// DATA_RETENTION_TIME_IN_DAYS and CLUSTER BY are valid for Snowflake-managed
		"CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://b/' DATA_RETENTION_TIME_IN_DAYS = 7",
		"CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://b/' CLUSTER BY (id)",
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
		{"Both CATALOG_TABLE_NAME and CATALOG_NAMESPACE with SNOWFLAKE", "CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = 's3://test/' CATALOG_TABLE_NAME = 'ctn' CATALOG_NAMESPACE = 'cns'", []string{"CATALOG_TABLE_NAME is only valid when CATALOG is not 'SNOWFLAKE'", "CATALOG_NAMESPACE is only valid when CATALOG is not 'SNOWFLAKE'"}},
		{"No CATALOG no EXTERNAL_VOLUME no BASE_LOCATION", "CREATE ICEBERG TABLE t (id int)", []string{"BASE_LOCATION is mandatory for all Iceberg tables", "EXTERNAL_VOLUME is mandatory for Iceberg tables with external catalogs", "CATALOG is mandatory for Iceberg tables with external catalogs"}},
		{"REFRESH_MODE INCREMENTAL lowercase", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' REFRESH_MODE = 'bad_value'", []string{"Invalid REFRESH_MODE value"}},
		{"INITIALIZE invalid ON_SOMETHING", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' INITIALIZE = 'ON_SOMETHING'", []string{"Invalid INITIALIZE value"}},
		{"Whitespace-only BASE_LOCATION", "CREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE' BASE_LOCATION = '   '", []string{"BASE_LOCATION is mandatory for all Iceberg tables"}},
		{"CLUSTER BY and DATA_RETENTION both on external catalog", "CREATE ICEBERG TABLE t (id int) EXTERNAL_VOLUME = 'ev' CATALOG = 'c' BASE_LOCATION = 'l' CLUSTER BY (id) DATA_RETENTION_TIME_IN_DAYS = 1", []string{"CLUSTER BY is supported only for Snowflake-managed", "DATA_RETENTION_TIME_IN_DAYS applies only to Snowflake-managed"}},
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
		// Quoted column identifiers
		`CREATE HYBRID TABLE t1 ("id" INT PRIMARY KEY NOT NULL)`,
		`CREATE HYBRID TABLE t1 ("MyCol" INT NOT NULL, PRIMARY KEY ("MyCol"))`,
		// Multiple columns, composite PK, all NOT NULL
		"CREATE HYBRID TABLE t1 (id INT NOT NULL, name VARCHAR NOT NULL, age INT NOT NULL, PRIMARY KEY (id, name))",
		// Out-of-line PK column with AUTOINCREMENT satisfies NOT NULL requirement
		"CREATE HYBRID TABLE t1 (id INT AUTOINCREMENT, PRIMARY KEY (id))",
		// Out-of-line PK column with IDENTITY satisfies NOT NULL requirement
		"CREATE HYBRID TABLE t1 (id INT IDENTITY (1, 1), PRIMARY KEY (id))",
		// Composite PK: one column NOT NULL, one AUTOINCREMENT — both satisfied
		"CREATE HYBRID TABLE t1 (id INT NOT NULL, seq INT AUTOINCREMENT, PRIMARY KEY (id, seq))",
		// A quoted column named INDEX in a regular table must not trigger the
		// "Secondary indexes (INDEX) are only supported on hybrid tables" warning.
		`CREATE TABLE t1 (id INT, "INDEX" INT)`,
		// NOTNULL (single word, no space) is a valid Snowflake synonym for NOT NULL.
		// The validator must recognise it as satisfying the hybrid-table NOT NULL requirement.
		"CREATE HYBRID TABLE t1 (id INT PRIMARY KEY NOTNULL)",
		"CREATE HYBRID TABLE t1 (id INT NOTNULL, PRIMARY KEY (id))",
		// Quoted identifier containing a space — strings.Fields must not split it.
		`CREATE HYBRID TABLE t1 ("MY COL" INT NOT NULL, PRIMARY KEY ("MY COL"))`,
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
		{"Composite PK all columns missing NOT NULL", "CREATE HYBRID TABLE t1 (id INT, name INT, PRIMARY KEY (id, name))", []string{"Primary key columns in a hybrid table must be NOT NULL (column 'ID' omits it).", "Primary key columns in a hybrid table must be NOT NULL (column 'NAME' omits it)."}},
		{"Multiple violations at once", "CREATE OR REPLACE TRANSIENT HYBRID TABLE t1 (id INT) CLUSTER BY (id) DATA_RETENTION_TIME_IN_DAYS = 7 CHANGE_TRACKING = TRUE", []string{"OR REPLACE is not supported for hybrid tables", "TRANSIENT is not supported for hybrid tables", "CLUSTER BY is not supported on hybrid tables", "DATA_RETENTION_TIME_IN_DAYS is not applicable to hybrid tables", "CHANGE_TRACKING is not supported on hybrid tables", "Hybrid tables must have a PRIMARY KEY"}},
		{"Quoted column in PK missing NOT NULL", `CREATE HYBRID TABLE t1 ("myCol" INT, PRIMARY KEY ("myCol"))`, []string{"Primary key columns in a hybrid table must be NOT NULL"}},
		{"COPY GRANTS and missing PK", "CREATE HYBRID TABLE t1 (id INT) COPY GRANTS", []string{"COPY GRANTS is not supported on hybrid tables", "Hybrid tables must have a PRIMARY KEY"}},
		// OR REPLACE and IF NOT EXISTS are mutually exclusive — the iceberg and event table
		// validators check this via checkOrReplaceConflict, the hybrid validator should too.
		{"OR REPLACE and IF NOT EXISTS conflict", "CREATE OR REPLACE HYBRID TABLE IF NOT EXISTS t1 (id INT PRIMARY KEY NOT NULL)", []string{"OR REPLACE is not supported for hybrid tables", "Conflict between OR REPLACE and IF NOT EXISTS"}},
		// A column named "AUTOINCREMENT" (quoted identifier) must not be confused with
		// the AUTOINCREMENT attribute — the PK column still lacks an explicit NOT NULL.
		{"Column named AUTOINCREMENT in PK missing NOT NULL", `CREATE HYBRID TABLE t1 ("AUTOINCREMENT" INT, PRIMARY KEY ("AUTOINCREMENT"))`, []string{"Primary key columns in a hybrid table must be NOT NULL"}},
		// Same flaw as AUTOINCREMENT above — a quoted identifier named "IDENTITY"
		// must not be confused with the IDENTITY column attribute.
		{"Column named IDENTITY in PK missing NOT NULL", `CREATE HYBRID TABLE t1 ("IDENTITY" INT, PRIMARY KEY ("IDENTITY"))`, []string{"Primary key columns in a hybrid table must be NOT NULL"}},
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
		// COPY GRANTS combined with IF NOT EXISTS (both valid)
		"CREATE EVENT TABLE IF NOT EXISTS my_events COPY GRANTS",
		// COPY GRANTS combined with OR REPLACE
		"CREATE OR REPLACE EVENT TABLE my_events COPY GRANTS",
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
		{
			"Very large DATA_RETENTION_TIME_IN_DAYS",
			"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = 99999999999999999999",
			[]string{"DATA_RETENTION_TIME_IN_DAYS must be a non-negative integer"},
		},
		{
			"Multiple invalid properties",
			"CREATE EVENT TABLE my_events AUTO_REFRESH = TRUE REPLACE_INVALID_CHARACTERS = TRUE",
			[]string{"Unexpected property"},
		},
		{
			"CLUSTER BY and column defs combined",
			"CREATE EVENT TABLE my_events (col1 INT) CLUSTER BY (col1)",
			[]string{"Event tables have a fixed schema and do not support column definitions", "CLUSTER BY is not supported for EVENT TABLE"},
		},
		{
			"Multiple unexpected properties each warned",
			"CREATE EVENT TABLE my_events AUTO_REFRESH = TRUE EXTERNAL_VOLUME = 'ev'",
			[]string{"Unexpected property 'AUTO_REFRESH'", "Unexpected property 'EXTERNAL_VOLUME'"},
		},
		// Fractional values are not valid non-negative integers — the regex
		// must not silently accept the integer portion of a decimal number.
		{
			"Decimal DATA_RETENTION_TIME_IN_DAYS",
			"CREATE EVENT TABLE my_events DATA_RETENTION_TIME_IN_DAYS = 1.5",
			[]string{"DATA_RETENTION_TIME_IN_DAYS must be a non-negative integer"},
		},
		{
			"Decimal MAX_DATA_EXTENSION_TIME_IN_DAYS",
			"CREATE EVENT TABLE my_events MAX_DATA_EXTENSION_TIME_IN_DAYS = 2.5",
			[]string{"MAX_DATA_EXTENSION_TIME_IN_DAYS must be a non-negative integer"},
		},
		// TRANSIENT is not supported for event tables — the guard regex must
		// route CREATE TRANSIENT EVENT TABLE to the event table validator.
		{
			"TRANSIENT event table",
			"CREATE TRANSIENT EVENT TABLE my_events",
			[]string{"TRANSIENT"},
		},
		// CREATE OR REPLACE TRANSIENT EVENT TABLE must also be routed to the
		// event table validator — the guard regex must handle TRANSIENT after OR REPLACE.
		{
			"OR REPLACE TRANSIENT event table",
			"CREATE OR REPLACE TRANSIENT EVENT TABLE my_events",
			[]string{"TRANSIENT"},
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
			// Trailing comma after last expression — empty segment is skipped
			"ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY(c1),",
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
			// Expression type without parentheses
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON EQUALITY",
				wantMsg: "Invalid search optimization expression",
			},
			// SUBSTRING without parentheses
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON SUBSTRING",
				wantMsg: "Invalid search optimization expression",
			},
			// GEO without parentheses
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON GEO",
				wantMsg: "Invalid search optimization expression",
			},
			// FULL_TEXT without parentheses
			{
				sql:     "ALTER TABLE t ADD SEARCH OPTIMIZATION ON FULL_TEXT",
				wantMsg: "Invalid search optimization expression",
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
			// Lowercase bare DOWNSTREAM is accepted (case-insensitive regex)
			"ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = downstream",
			// RENAME TO with quoted identifier
			"ALTER DYNAMIC TABLE my_dt RENAME TO \"new-name\"",
			// UNSET COMMENT is valid
			"ALTER DYNAMIC TABLE my_dt UNSET COMMENT",
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
			// Number without time unit in TARGET_LAG
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1'",
				wantMsg: "Invalid TARGET_LAG value",
			},
			// Negative number in TARGET_LAG
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '-5 minutes'",
				wantMsg: "Invalid TARGET_LAG value",
			},
			// Valid TARGET_LAG inside a line comment must not mask the invalid actual
			// value — the validator should check cleaned text, not raw text with comments.
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = 'invalid' -- TARGET_LAG = '1 minute'",
				wantMsg: "Invalid TARGET_LAG value",
			},
			// Same flaw with a block comment containing a valid TARGET_LAG value.
			{
				sql:     "ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = 42 /* TARGET_LAG = '1 minute' */",
				wantMsg: "Invalid TARGET_LAG value",
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
			// Two-part same-schema different tables
			"ALTER TABLE myschema.t1 SWAP WITH myschema.t2",
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
			// Different part counts — not flagged as same table
			"ALTER TABLE t1 SWAP WITH db.schema.t1",
			"ALTER TABLE db.schema.t1 SWAP WITH t1",
			// Trailing comment is stripped — no false positive
			"ALTER TABLE t1 SWAP WITH t2 -- this is a comment",
			// Quoted lowercase identifier is case-sensitive in Snowflake:
			// "orders" (exact lowercase) is a different table from ORDERS (unquoted → uppercase).
			`ALTER TABLE "orders" SWAP WITH ORDERS`,
			// Quoted identifier with an internal dot ("A.B") is a single 1-part name,
			// different from the 2-part path A.B — must not be flagged as same table.
			`ALTER TABLE "A.B" SWAP WITH A.B`,
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
			// Case-insensitive same table detection
			{
				sql:     "ALTER TABLE orders SWAP WITH ORDERS",
				wantMsg: "SWAP WITH the same table",
			},
			// Quoted vs unquoted same table
			{
				sql:     `ALTER TABLE "ORDERS" SWAP WITH ORDERS`,
				wantMsg: "SWAP WITH the same table",
			},
			// Three-part same table name
			{
				sql:     "ALTER TABLE db.schema.t1 SWAP WITH db.schema.t1",
				wantMsg: "SWAP WITH the same table",
			},
			// Two-part same table name
			{
				sql:     "ALTER TABLE schema.t1 SWAP WITH schema.t1",
				wantMsg: "SWAP WITH the same table",
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

// ── Multi-statement tests ────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_MultiStatement(t *testing.T) {
	t.Run("iceberg table among other statements", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE ICEBERG TABLE t (id int) CATALOG = 'SNOWFLAKE';\nSELECT 2;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "BASE_LOCATION is mandatory") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected BASE_LOCATION warning in multi-statement input, got: %v", warns)
		}
	})

	t.Run("hybrid table among other statements", func(t *testing.T) {
		sql := "USE DATABASE foo;\nCREATE HYBRID TABLE t1 (id INT);\nSELECT * FROM t1;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "Hybrid tables must have a PRIMARY KEY") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected PRIMARY KEY warning in multi-statement input, got: %v", warns)
		}
	})

	t.Run("valid statements produce no warnings", func(t *testing.T) {
		sql := "SELECT 1;\nSELECT 2;\nSELECT 3;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for plain SELECTs, got: %v", warns)
		}
	})

	t.Run("event table among other statements", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE EVENT TABLE my_events (col1 INT);\nSELECT 2;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "Event tables have a fixed schema") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected column definition warning in multi-statement input, got: %v", warns)
		}
	})

	t.Run("alter dynamic table among other statements", func(t *testing.T) {
		sql := "SELECT 1;\nALTER DYNAMIC TABLE dt TRUNCATE;\nSELECT 2;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "Unknown ALTER DYNAMIC TABLE sub-command") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected unknown sub-command warning in multi-statement input, got: %v", warns)
		}
	})

	t.Run("alter table swap with among other statements", func(t *testing.T) {
		sql := "SELECT 1;\nALTER TABLE t1 SWAP WITH t1;\nSELECT 2;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "SWAP WITH the same table") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected same-table warning in multi-statement input, got: %v", warns)
		}
	})
}


