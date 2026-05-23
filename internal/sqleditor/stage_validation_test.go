package sqleditor

import (
	"strings"
	"testing"
)

func TestCreateStage_Valid(t *testing.T) {
	valid := []string{
		// ── 1. Modifiers ─────────────────────────────────────────────────────
		"CREATE STAGE s",
		"CREATE OR REPLACE STAGE s",
		"CREATE TEMPORARY STAGE s",
		"CREATE OR REPLACE TEMPORARY STAGE s",
		"CREATE STAGE IF NOT EXISTS s",
		// ── 2. Object naming ─────────────────────────────────────────────────
		"CREATE STAGE db.schema.s",
		`CREATE STAGE "my stage"`,
		"CREATE OR REPLACE STAGE db.schema.my_stage",
		// ── 3. COMMENT only ──────────────────────────────────────────────────
		"CREATE STAGE s COMMENT = 'test stage'",
		"CREATE OR REPLACE STAGE s COMMENT = 'production stage'",
		// ── 4. URL variants ──────────────────────────────────────────────────
		"CREATE STAGE s URL = 's3://my-bucket/'",
		"CREATE STAGE s URL = 's3://my-bucket/my-prefix/'",
		"CREATE STAGE s URL = 'gcs://my-bucket/'",
		"CREATE STAGE s URL = 'gcs://my-bucket/path/'",
		"CREATE STAGE s URL = 'azure://myaccount.blob.core.windows.net/mycontainer/'",
		"CREATE STAGE s URL = 'azure://myaccount.blob.core.windows.net/mycontainer/path/'",
		"CREATE STAGE s URL = 's3compat://my-bucket/' ENDPOINT = 'storage.example.com'",
		"CREATE STAGE s URL = 'azure://onelake.blob.fabric.microsoft.com/ws-id/item-id/Files/'",
		// ── 5. STORAGE_INTEGRATION ───────────────────────────────────────────
		"CREATE STAGE s STORAGE_INTEGRATION = my_int",
		"CREATE STAGE s URL = 's3://bucket/' STORAGE_INTEGRATION = my_s3_int",
		"CREATE STAGE s URL = 'gcs://bucket/' STORAGE_INTEGRATION = my_gcs_int",
		"CREATE STAGE s URL = 'azure://acct.blob.core.windows.net/cont/' STORAGE_INTEGRATION = my_az_int",
		// ── 6. CREDENTIALS ───────────────────────────────────────────────────
		"CREATE STAGE s URL = 's3://bucket/' CREDENTIALS = (AWS_KEY_ID = 'key' AWS_SECRET_KEY = 'secret')",
		"CREATE STAGE s URL = 's3://bucket/' CREDENTIALS = (AWS_KEY_ID = 'k' AWS_SECRET_KEY = 's' AWS_TOKEN = 'tok')",
		"CREATE STAGE s URL = 's3://bucket/' CREDENTIALS = (AWS_ROLE = 'arn:aws:iam::123:role/my-role')",
		"CREATE STAGE s URL = 'azure://acct.blob.core.windows.net/cont/' CREDENTIALS = (AZURE_SAS_TOKEN = 'sas-token')",
		// ── 7. ENCRYPTION – internal stages ──────────────────────────────────
		"CREATE STAGE s ENCRYPTION = (TYPE = 'SNOWFLAKE_FULL')",
		"CREATE STAGE s ENCRYPTION = (TYPE = 'SNOWFLAKE_SSE')",
		// ── 8. ENCRYPTION – S3 external stages ───────────────────────────────
		"CREATE STAGE s URL = 's3://bucket/' ENCRYPTION = (TYPE = 'AWS_SSE_S3')",
		"CREATE STAGE s URL = 's3://bucket/' ENCRYPTION = (TYPE = 'AWS_SSE_KMS')",
		"CREATE STAGE s URL = 's3://bucket/' ENCRYPTION = (TYPE = 'AWS_SSE_KMS' KMS_KEY_ID = 'arn:aws:kms:us-east-1:123:key/id')",
		"CREATE STAGE s URL = 's3://bucket/' ENCRYPTION = (TYPE = 'AWS_CSE' MASTER_KEY = 'base64key==')",
		"CREATE STAGE s URL = 's3://bucket/' ENCRYPTION = (TYPE = 'NONE')",
		// ── 9. ENCRYPTION – GCS external stages ──────────────────────────────
		"CREATE STAGE s URL = 'gcs://bucket/' ENCRYPTION = (TYPE = 'GCS_SSE_KMS')",
		"CREATE STAGE s URL = 'gcs://bucket/' ENCRYPTION = (TYPE = 'GCS_SSE_KMS' KMS_KEY_ID = 'projects/p/locations/l/keyRings/r/cryptoKeys/k')",
		"CREATE STAGE s URL = 'gcs://bucket/' ENCRYPTION = (TYPE = 'NONE')",
		// ── 10. ENCRYPTION – Azure external stages ────────────────────────────
		"CREATE STAGE s URL = 'azure://acct.blob.core.windows.net/cont/' ENCRYPTION = (TYPE = 'AZURE_CSE' MASTER_KEY = 'base64key==')",
		"CREATE STAGE s URL = 'azure://acct.blob.core.windows.net/cont/' ENCRYPTION = (TYPE = 'NONE')",
		// ── 11. FILE_FORMAT – by type (THE BUG CASE and variants) ─────────────
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'AVRO')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'ORC')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'PARQUET')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'XML')",
		// The exact failing case from the issue
		"CREATE OR REPLACE STAGE my_internal_test_stage FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',' SKIP_HEADER = 1) COMMENT = 'Internal stage for testing Snowpipe'",
		// ── 12. FILE_FORMAT – by name ─────────────────────────────────────────
		"CREATE STAGE s FILE_FORMAT = (FORMAT_NAME = 'my_csv_format')",
		"CREATE STAGE s FILE_FORMAT = (FORMAT_NAME = 'db.schema.my_format')",
		// ── 13. FILE_FORMAT – CSV options ─────────────────────────────────────
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' SKIP_HEADER = 1)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',' SKIP_HEADER = 1)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' COMPRESSION = 'GZIP')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' RECORD_DELIMITER = '\n')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' NULL_IF = ('NULL', 'N/A'))",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' SKIP_BLANK_LINES = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' TRIM_SPACE = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' ERROR_ON_COLUMN_COUNT_MISMATCH = FALSE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' DATE_FORMAT = 'YYYY-MM-DD')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' TIMESTAMP_FORMAT = 'YYYY-MM-DD HH24:MI:SS')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' BINARY_FORMAT = HEX)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' ESCAPE = '\\')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' FIELD_OPTIONALLY_ENCLOSED_BY = '\"')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' EMPTY_FIELD_AS_NULL = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' SKIP_BYTE_ORDER_MARK = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' ENCODING = 'UTF8')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' PARSE_HEADER = FALSE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' MULTI_LINE = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' FILE_EXTENSION = '.csv')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' REPLACE_INVALID_CHARACTERS = TRUE)",
		// ── 14. FILE_FORMAT – JSON options ────────────────────────────────────
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' STRIP_OUTER_ARRAY = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' ALLOW_DUPLICATE = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' STRIP_NULL_VALUES = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' ENABLE_OCTAL = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' IGNORE_UTF8_ERRORS = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'JSON' MULTI_LINE = TRUE)",
		// ── 15. FILE_FORMAT – PARQUET / AVRO / ORC / XML options ─────────────
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'PARQUET' BINARY_AS_TEXT = FALSE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'PARQUET' COMPRESSION = 'SNAPPY')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'PARQUET' USE_LOGICAL_TYPE = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'PARQUET' USE_VECTORIZED_SCANNER = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'AVRO' COMPRESSION = 'GZIP')",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'XML' IGNORE_UTF8_ERRORS = TRUE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'XML' PRESERVE_SPACE = FALSE)",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'XML' STRIP_OUTER_ELEMENT = TRUE)",
		// ── 16. COPY_OPTIONS ──────────────────────────────────────────────────
		"CREATE STAGE s COPY_OPTIONS = (ON_ERROR = CONTINUE)",
		"CREATE STAGE s COPY_OPTIONS = (ON_ERROR = ABORT_STATEMENT SIZE_LIMIT = 100)",
		"CREATE STAGE s COPY_OPTIONS = (FORCE = TRUE)",
		"CREATE STAGE s COPY_OPTIONS = (PURGE = TRUE)",
		// ── 17. DIRECTORY params – internal ───────────────────────────────────
		"CREATE STAGE s DIRECTORY = (ENABLE = TRUE)",
		"CREATE STAGE s DIRECTORY = (ENABLE = FALSE)",
		"CREATE STAGE s DIRECTORY = (ENABLE = TRUE AUTO_REFRESH = TRUE)",
		"CREATE STAGE s DIRECTORY = (ENABLE = TRUE REFRESH_ON_CREATE = FALSE)",
		// ── 18. DIRECTORY params – S3 external ───────────────────────────────
		"CREATE STAGE s URL = 's3://bucket/' DIRECTORY = (ENABLE = TRUE REFRESH_ON_CREATE = TRUE AUTO_REFRESH = TRUE)",
		// ── 19. DIRECTORY params – GCS external ──────────────────────────────
		"CREATE STAGE s URL = 'gcs://bucket/' DIRECTORY = (ENABLE = TRUE AUTO_REFRESH = TRUE NOTIFICATION_INTEGRATION = 'my_notif_int')",
		// ── 20. DIRECTORY params – Azure external ─────────────────────────────
		"CREATE STAGE s URL = 'azure://acct.blob.core.windows.net/cont/' DIRECTORY = (ENABLE = TRUE AUTO_REFRESH = TRUE NOTIFICATION_INTEGRATION = 'my_notif_int')",
		// ── 21. AWS optional params ────────────────────────────────────────────
		"CREATE STAGE s URL = 's3://bucket/' AWS_ACCESS_POINT_ARN = 'arn:aws:s3:::accesspoint/my-ap'",
		"CREATE STAGE s URL = 's3://bucket/' USE_PRIVATELINK_ENDPOINT = TRUE",
		"CREATE STAGE s URL = 's3://bucket/' USE_PRIVATELINK_ENDPOINT = FALSE",
		// ── 22. S3-compatible ─────────────────────────────────────────────────
		"CREATE STAGE s URL = 's3compat://bucket/' ENDPOINT = 'storage.example.com' CREDENTIALS = (AWS_KEY_ID = 'key' AWS_SECRET_KEY = 'secret')",
		// ── 23. Complex combined examples ─────────────────────────────────────
		"CREATE OR REPLACE STAGE my_s3_stage URL = 's3://my-bucket/path/' STORAGE_INTEGRATION = my_int FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',' SKIP_HEADER = 1) DIRECTORY = (ENABLE = TRUE AUTO_REFRESH = TRUE) COMMENT = 'S3 stage'",
		"CREATE STAGE my_gcs_stage URL = 'gcs://my-bucket/' STORAGE_INTEGRATION = gcs_int FILE_FORMAT = (TYPE = 'JSON') DIRECTORY = (ENABLE = TRUE) COMMENT = 'GCS stage'",
		"CREATE STAGE my_azure_stage URL = 'azure://myacct.blob.core.windows.net/mycontainer/' CREDENTIALS = (AZURE_SAS_TOKEN = 'token') FILE_FORMAT = (TYPE = 'CSV') COMMENT = 'Azure stage'",
		"CREATE STAGE internal_stage ENCRYPTION = (TYPE = 'SNOWFLAKE_FULL') FILE_FORMAT = (TYPE = 'JSON') COMMENT = 'internal encrypted'",
		"CREATE OR REPLACE TEMPORARY STAGE temp_stage FILE_FORMAT = (TYPE = 'CSV')",
		"CREATE STAGE s URL = 's3://bucket/' CREDENTIALS = (AWS_KEY_ID = 'k' AWS_SECRET_KEY = 's') ENCRYPTION = (TYPE = 'AWS_SSE_S3') FILE_FORMAT = (TYPE = 'JSON') COMMENT = 'stage'",
		"CREATE STAGE s URL = 'azure://onelake.blob.fabric.microsoft.com/ws-id/item-id/Files/' STORAGE_INTEGRATION = my_onelake_int",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',' SKIP_HEADER = 1 RECORD_DELIMITER = '\n' TRIM_SPACE = TRUE NULL_IF = ('NULL'))",
		"CREATE OR REPLACE STAGE full_stage URL = 's3://bucket/' STORAGE_INTEGRATION = si ENCRYPTION = (TYPE = 'AWS_SSE_KMS' KMS_KEY_ID = 'kms_key') FILE_FORMAT = (TYPE = 'CSV') COPY_OPTIONS = (ON_ERROR = CONTINUE) DIRECTORY = (ENABLE = TRUE) COMMENT = 'full'",
	}

	for _, sql := range valid {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}
}

// TestCreateStage_Invalid covers CREATE STAGE statements with top-level
// property names that belong inside nested blocks (FILE_FORMAT, ENCRYPTION,
// DIRECTORY, CREDENTIALS, COPY_OPTIONS) or are simply not valid stage params.
// Each case must produce at least one warning.
func TestCreateStage_Invalid(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectedMatch string
	}{
		// Wrong top-level URL names
		{"Invalid BUCKET_URL", "CREATE STAGE s BUCKET_URL = 's3://bucket/'", "BUCKET_URL"},
		{"Invalid S3_URL", "CREATE STAGE s S3_URL = 's3://bucket/'", "S3_URL"},
		// Invalid non-stage params at top level
		{"Invalid REGION", "CREATE STAGE s URL = 's3://bucket/' REGION = 'us-east-1'", "REGION"},
		{"Invalid WAREHOUSE", "CREATE STAGE s WAREHOUSE = 'WH'", "WAREHOUSE"},
		{"Invalid MAX_FILE_SIZE", "CREATE STAGE s MAX_FILE_SIZE = 100", "MAX_FILE_SIZE"},
		// FILE_FORMAT sub-options placed at top level
		{"TYPE at top level", "CREATE STAGE s TYPE = 'CSV'", "TYPE"},
		{"FIELD_DELIMITER at top level", "CREATE STAGE s FIELD_DELIMITER = ','", "FIELD_DELIMITER"},
		{"SKIP_HEADER at top level", "CREATE STAGE s SKIP_HEADER = 1", "SKIP_HEADER"},
		{"COMPRESSION at top level", "CREATE STAGE s COMPRESSION = 'GZIP'", "COMPRESSION"},
		{"RECORD_DELIMITER at top level", "CREATE STAGE s RECORD_DELIMITER = '\n'", "RECORD_DELIMITER"},
		{"NULL_IF at top level", "CREATE STAGE s NULL_IF = ('NULL')", "NULL_IF"},
		{"DATE_FORMAT at top level", "CREATE STAGE s DATE_FORMAT = 'YYYY-MM-DD'", "DATE_FORMAT"},
		{"TIMESTAMP_FORMAT at top level", "CREATE STAGE s TIMESTAMP_FORMAT = 'AUTO'", "TIMESTAMP_FORMAT"},
		{"TRIM_SPACE at top level", "CREATE STAGE s TRIM_SPACE = TRUE", "TRIM_SPACE"},
		{"SKIP_BLANK_LINES at top level", "CREATE STAGE s SKIP_BLANK_LINES = TRUE", "SKIP_BLANK_LINES"},
		{"ERROR_ON_COLUMN_COUNT_MISMATCH at top level", "CREATE STAGE s ERROR_ON_COLUMN_COUNT_MISMATCH = TRUE", "ERROR_ON_COLUMN_COUNT_MISMATCH"},
		{"STRIP_OUTER_ARRAY at top level", "CREATE STAGE s STRIP_OUTER_ARRAY = TRUE", "STRIP_OUTER_ARRAY"},
		{"BINARY_AS_TEXT at top level", "CREATE STAGE s BINARY_AS_TEXT = FALSE", "BINARY_AS_TEXT"},
		{"FORMAT_NAME at top level", "CREATE STAGE s FORMAT_NAME = 'my_format'", "FORMAT_NAME"},
		// ENCRYPTION sub-options placed at top level
		{"MASTER_KEY at top level", "CREATE STAGE s MASTER_KEY = 'key'", "MASTER_KEY"},
		{"KMS_KEY_ID at top level", "CREATE STAGE s KMS_KEY_ID = 'key-arn'", "KMS_KEY_ID"},
		// CREDENTIALS sub-options placed at top level
		{"AWS_KEY_ID at top level", "CREATE STAGE s AWS_KEY_ID = 'key'", "AWS_KEY_ID"},
		{"AWS_SECRET_KEY at top level", "CREATE STAGE s AWS_SECRET_KEY = 'secret'", "AWS_SECRET_KEY"},
		// DIRECTORY sub-options placed at top level
		{"ENABLE at top level", "CREATE STAGE s ENABLE = TRUE", "ENABLE"},
		{"AUTO_REFRESH at top level", "CREATE STAGE s AUTO_REFRESH = TRUE", "AUTO_REFRESH"},
		{"REFRESH_ON_CREATE at top level", "CREATE STAGE s REFRESH_ON_CREATE = TRUE", "REFRESH_ON_CREATE"},
		// COPY_OPTIONS sub-options placed at top level
		{"ON_ERROR at top level", "CREATE STAGE s ON_ERROR = CONTINUE", "ON_ERROR"},
		{"SIZE_LIMIT at top level", "CREATE STAGE s SIZE_LIMIT = 100", "SIZE_LIMIT"},
		{"FORCE at top level", "CREATE STAGE s FORCE = TRUE", "FORCE"},
		{"PURGE at top level", "CREATE STAGE s PURGE = TRUE", "PURGE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) == 0 {
				t.Fatalf("Expected warnings for %q, got 0", tt.sql)
			}
			found := false
			for _, w := range warnings {
				if strings.Contains(strings.ToUpper(w.Message), strings.ToUpper(tt.expectedMatch)) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected warning matching %q, got: %v", tt.expectedMatch, warnings[0].Message)
			}
		})
	}
}

// TestAlterStage_Valid covers all ALTER STAGE forms: RENAME TO, SET (external
// params, FILE_FORMAT, COMMENT, DIRECTORY), REFRESH, SET TAG, UNSET TAG, and
// UNSET DCM PROJECT.  Each case must produce zero warnings.
func TestAlterStage_Valid(t *testing.T) {
	valid := []string{
		// ── 1. RENAME TO ─────────────────────────────────────────────────────
		"ALTER STAGE s RENAME TO new_s",
		"ALTER STAGE IF EXISTS s RENAME TO new_s",
		"ALTER STAGE my_db.my_schema.s RENAME TO new_s",
		`ALTER STAGE "my stage" RENAME TO "my new stage"`,
		// ── 2. SET COMMENT ───────────────────────────────────────────────────
		"ALTER STAGE s SET COMMENT = 'updated comment'",
		"ALTER STAGE IF EXISTS s SET COMMENT = 'updated'",
		"ALTER STAGE s SET COMMENT = ''",
		// ── 3. SET FILE_FORMAT – by type ─────────────────────────────────────
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'JSON')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'AVRO')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'ORC')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'PARQUET')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'XML')",
		// ── 4. SET FILE_FORMAT – by name ─────────────────────────────────────
		"ALTER STAGE s SET FILE_FORMAT = (FORMAT_NAME = 'my_format')",
		"ALTER STAGE s SET FILE_FORMAT = (FORMAT_NAME = 'db.schema.my_format')",
		// ── 5. SET FILE_FORMAT – CSV options (nested, not top-level) ─────────
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = ',' SKIP_HEADER = 1)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' COMPRESSION = 'GZIP')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' NULL_IF = ('NULL', 'N/A'))",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' SKIP_BLANK_LINES = TRUE TRIM_SPACE = TRUE)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' DATE_FORMAT = 'YYYY-MM-DD' TIMESTAMP_FORMAT = 'YYYY-MM-DD HH24:MI:SS')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' ERROR_ON_COLUMN_COUNT_MISMATCH = FALSE)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV' RECORD_DELIMITER = '\n' FIELD_DELIMITER = ',' SKIP_HEADER = 1 TRIM_SPACE = TRUE)",
		// ── 6. SET FILE_FORMAT – JSON / PARQUET / XML options ────────────────
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'JSON' STRIP_OUTER_ARRAY = TRUE)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'JSON' ALLOW_DUPLICATE = FALSE)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'PARQUET' BINARY_AS_TEXT = FALSE)",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'PARQUET' COMPRESSION = 'SNAPPY')",
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'XML' IGNORE_UTF8_ERRORS = TRUE)",
		// ── 7. SET URL ────────────────────────────────────────────────────────
		"ALTER STAGE s SET URL = 's3://new-bucket/'",
		"ALTER STAGE s SET URL = 'gcs://new-bucket/'",
		"ALTER STAGE s SET URL = 'azure://newacct.blob.core.windows.net/newcont/'",
		// ── 8. SET STORAGE_INTEGRATION ────────────────────────────────────────
		"ALTER STAGE s SET STORAGE_INTEGRATION = new_int",
		"ALTER STAGE IF EXISTS s SET STORAGE_INTEGRATION = new_gcs_int",
		// ── 9. SET CREDENTIALS ────────────────────────────────────────────────
		"ALTER STAGE s SET CREDENTIALS = (AWS_KEY_ID = 'new_key' AWS_SECRET_KEY = 'new_secret')",
		"ALTER STAGE s SET CREDENTIALS = (AWS_ROLE = 'arn:aws:iam::123:role/new-role')",
		"ALTER STAGE s SET CREDENTIALS = (AZURE_SAS_TOKEN = 'new-sas-token')",
		// ── 10. SET ENCRYPTION ────────────────────────────────────────────────
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'AWS_SSE_S3')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'AWS_SSE_KMS')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'AWS_SSE_KMS' KMS_KEY_ID = 'new-key-arn')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'NONE')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'GCS_SSE_KMS' KMS_KEY_ID = 'gcs-key')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'AZURE_CSE' MASTER_KEY = 'az-key==')",
		"ALTER STAGE IF EXISTS s SET ENCRYPTION = (TYPE = 'SNOWFLAKE_FULL')",
		"ALTER STAGE s SET ENCRYPTION = (TYPE = 'SNOWFLAKE_SSE')",
		// ── 11. SET DIRECTORY ─────────────────────────────────────────────────
		"ALTER STAGE s SET DIRECTORY = (ENABLE = TRUE)",
		"ALTER STAGE s SET DIRECTORY = (ENABLE = FALSE)",
		"ALTER STAGE IF EXISTS s SET DIRECTORY = (ENABLE = TRUE)",
		// ── 12. SET multiple properties ───────────────────────────────────────
		"ALTER STAGE s SET FILE_FORMAT = (TYPE = 'CSV') COMMENT = 'updated stage'",
		"ALTER STAGE s SET URL = 's3://new-bucket/' STORAGE_INTEGRATION = new_int",
		"ALTER STAGE s SET URL = 's3://bucket/' ENCRYPTION = (TYPE = 'AWS_SSE_S3')",
		"ALTER STAGE s SET CREDENTIALS = (AWS_KEY_ID = 'k' AWS_SECRET_KEY = 's') ENCRYPTION = (TYPE = 'AWS_SSE_S3')",
		"ALTER STAGE s SET COMMENT = 'test' FILE_FORMAT = (TYPE = 'CSV')",
		"ALTER STAGE s SET DIRECTORY = (ENABLE = TRUE) COMMENT = 'with directory'",
		"ALTER STAGE IF EXISTS my_s3_stage SET URL = 's3://new-bucket/' STORAGE_INTEGRATION = new_si ENCRYPTION = (TYPE = 'AWS_SSE_KMS' KMS_KEY_ID = 'key') FILE_FORMAT = (TYPE = 'JSON') COMMENT = 'updated'",
		// ── 13. SET AWS optional params ───────────────────────────────────────
		"ALTER STAGE s SET AWS_ACCESS_POINT_ARN = 'arn:aws:s3:::accesspoint/my-ap'",
		"ALTER STAGE s SET USE_PRIVATELINK_ENDPOINT = TRUE",
		"ALTER STAGE s SET USE_PRIVATELINK_ENDPOINT = FALSE",
		// ── 14. SET COPY_OPTIONS ──────────────────────────────────────────────
		"ALTER STAGE s SET COPY_OPTIONS = (ON_ERROR = CONTINUE)",
		"ALTER STAGE s SET COPY_OPTIONS = (ON_ERROR = SKIP_FILE SIZE_LIMIT = 100)",
		// ── 15. REFRESH ───────────────────────────────────────────────────────
		"ALTER STAGE s REFRESH",
		"ALTER STAGE IF EXISTS s REFRESH",
		"ALTER STAGE s REFRESH SUBPATH = 'my/subfolder/'",
		"ALTER STAGE IF EXISTS s REFRESH SUBPATH = 'data/'",
		// ── 16. TAG operations – skipped (dynamic tag names) ──────────────────
		"ALTER STAGE s UNSET TAG my_tag",
		"ALTER STAGE s UNSET TAG tag1, tag2",
		"ALTER STAGE s UNSET DCM PROJECT",
		"ALTER STAGE s SET TAG my_tag = 'my_value'",
		"ALTER STAGE s SET TAG tag1 = 'val1', tag2 = 'val2'",
		"ALTER STAGE IF EXISTS s SET TAG dept = 'finance'",
		// ── 17. Quoted stage names ─────────────────────────────────────────────
		`ALTER STAGE "my stage" SET COMMENT = 'quoted name'`,
		`ALTER STAGE IF EXISTS "My Stage" REFRESH`,
	}

	for _, sql := range valid {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}
}

// TestAlterStage_Invalid covers ALTER STAGE SET statements that use sub-option
// property names at the top level instead of inside the correct nested block.
func TestAlterStage_Invalid(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectedMatch string
	}{
		// Wrong URL-like names
		{"Invalid BUCKET_URL in SET", "ALTER STAGE s SET BUCKET_URL = 's3://bucket/'", "BUCKET_URL"},
		{"Invalid REGION in SET", "ALTER STAGE s SET REGION = 'us-east-1'", "REGION"},
		// FILE_FORMAT sub-options at top level
		{"TYPE at top level in SET", "ALTER STAGE s SET TYPE = 'CSV'", "TYPE"},
		{"FIELD_DELIMITER at top level", "ALTER STAGE s SET FIELD_DELIMITER = ','", "FIELD_DELIMITER"},
		{"SKIP_HEADER at top level", "ALTER STAGE s SET SKIP_HEADER = 1", "SKIP_HEADER"},
		{"COMPRESSION at top level", "ALTER STAGE s SET COMPRESSION = 'GZIP'", "COMPRESSION"},
		{"RECORD_DELIMITER at top level", "ALTER STAGE s SET RECORD_DELIMITER = '\n'", "RECORD_DELIMITER"},
		{"NULL_IF at top level", "ALTER STAGE s SET NULL_IF = ('NULL')", "NULL_IF"},
		{"DATE_FORMAT at top level", "ALTER STAGE s SET DATE_FORMAT = 'YYYY-MM-DD'", "DATE_FORMAT"},
		{"TRIM_SPACE at top level", "ALTER STAGE s SET TRIM_SPACE = TRUE", "TRIM_SPACE"},
		{"SKIP_BLANK_LINES at top level", "ALTER STAGE s SET SKIP_BLANK_LINES = TRUE", "SKIP_BLANK_LINES"},
		{"STRIP_OUTER_ARRAY at top level", "ALTER STAGE s SET STRIP_OUTER_ARRAY = TRUE", "STRIP_OUTER_ARRAY"},
		{"BINARY_AS_TEXT at top level", "ALTER STAGE s SET BINARY_AS_TEXT = FALSE", "BINARY_AS_TEXT"},
		{"FORMAT_NAME at top level", "ALTER STAGE s SET FORMAT_NAME = 'my_fmt'", "FORMAT_NAME"},
		{"ERROR_ON_COLUMN_COUNT_MISMATCH at top", "ALTER STAGE s SET ERROR_ON_COLUMN_COUNT_MISMATCH = TRUE", "ERROR_ON_COLUMN_COUNT_MISMATCH"},
		{"IGNORE_UTF8_ERRORS at top level", "ALTER STAGE s SET IGNORE_UTF8_ERRORS = TRUE", "IGNORE_UTF8_ERRORS"},
		{"SKIP_BYTE_ORDER_MARK at top level", "ALTER STAGE s SET SKIP_BYTE_ORDER_MARK = TRUE", "SKIP_BYTE_ORDER_MARK"},
		{"ALLOW_DUPLICATE at top level", "ALTER STAGE s SET ALLOW_DUPLICATE = TRUE", "ALLOW_DUPLICATE"},
		{"REPLACE_INVALID_CHARACTERS at top", "ALTER STAGE s SET REPLACE_INVALID_CHARACTERS = TRUE", "REPLACE_INVALID_CHARACTERS"},
		// ENCRYPTION sub-options at top level
		{"MASTER_KEY at top level", "ALTER STAGE s SET MASTER_KEY = 'key'", "MASTER_KEY"},
		{"KMS_KEY_ID at top level", "ALTER STAGE s SET KMS_KEY_ID = 'key-arn'", "KMS_KEY_ID"},
		// CREDENTIALS sub-options at top level
		{"AWS_KEY_ID at top level", "ALTER STAGE s SET AWS_KEY_ID = 'key'", "AWS_KEY_ID"},
		// DIRECTORY sub-options at top level
		{"ENABLE at top level in SET", "ALTER STAGE s SET ENABLE = TRUE", "ENABLE"},
		{"AUTO_REFRESH at top level", "ALTER STAGE s SET AUTO_REFRESH = TRUE", "AUTO_REFRESH"},
		{"REFRESH_ON_CREATE at top level", "ALTER STAGE s SET REFRESH_ON_CREATE = TRUE", "REFRESH_ON_CREATE"},
		// COPY_OPTIONS sub-options at top level
		{"ON_ERROR at top level", "ALTER STAGE s SET ON_ERROR = CONTINUE", "ON_ERROR"},
		{"SIZE_LIMIT at top level", "ALTER STAGE s SET SIZE_LIMIT = 100", "SIZE_LIMIT"},
		// XML sub-options at top level
		{"PRESERVE_SPACE at top level", "ALTER STAGE s SET PRESERVE_SPACE = TRUE", "PRESERVE_SPACE"},
		{"STRIP_OUTER_ELEMENT at top level", "ALTER STAGE s SET STRIP_OUTER_ELEMENT = TRUE", "STRIP_OUTER_ELEMENT"},
		// Invalid REFRESH parameter
		{"Invalid REFRESH param", "ALTER STAGE s REFRESH MAX_SIZE = 100", "MAX_SIZE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) == 0 {
				t.Fatalf("Expected warnings for %q, got 0", tt.sql)
			}
			found := false
			for _, w := range warnings {
				if strings.Contains(strings.ToUpper(w.Message), strings.ToUpper(tt.expectedMatch)) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected warning matching %q, got: %v", tt.expectedMatch, warnings[0].Message)
			}
		})
	}
}


