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
		// ── 23. Combined modifiers ───────────────────────────────────────────
		"CREATE TEMPORARY STAGE IF NOT EXISTS s",
		"CREATE OR REPLACE TEMPORARY STAGE IF NOT EXISTS s",
		"CREATE TEMPORARY STAGE IF NOT EXISTS s URL = 's3://bucket/' FILE_FORMAT = (TYPE = 'CSV')",
		// ── 24. Complex combined examples ─────────────────────────────────────
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
		{"AWS_TOKEN at top level", "CREATE STAGE s AWS_TOKEN = 'tok'", "AWS_TOKEN"},
		{"AWS_ROLE at top level", "CREATE STAGE s AWS_ROLE = 'arn:aws:iam::123:role/r'", "AWS_ROLE"},
		{"AZURE_SAS_TOKEN at top level", "CREATE STAGE s AZURE_SAS_TOKEN = 'sas-token'", "AZURE_SAS_TOKEN"},
		// DIRECTORY sub-options placed at top level
		{"ENABLE at top level", "CREATE STAGE s ENABLE = TRUE", "ENABLE"},
		{"AUTO_REFRESH at top level", "CREATE STAGE s AUTO_REFRESH = TRUE", "AUTO_REFRESH"},
		{"REFRESH_ON_CREATE at top level", "CREATE STAGE s REFRESH_ON_CREATE = TRUE", "REFRESH_ON_CREATE"},
		{"NOTIFICATION_INTEGRATION at top level", "CREATE STAGE s NOTIFICATION_INTEGRATION = 'my_notif'", "NOTIFICATION_INTEGRATION"},
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
		// ── 18. UNSET for regular properties (not TAG/DCM) ──────────────────
		"ALTER STAGE s UNSET COMMENT",
		"ALTER STAGE s UNSET FILE_FORMAT",
		"ALTER STAGE s UNSET COPY_OPTIONS",
		"ALTER STAGE IF EXISTS s UNSET COMMENT",
		"ALTER STAGE IF EXISTS s UNSET ENCRYPTION",
		// ── 19. Bare ALTER STAGE with no action ──────────────────────────────
		"ALTER STAGE s",
		"ALTER STAGE IF EXISTS s",
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

// TestCreateStage_CaseInsensitive verifies that keyword matching is
// case-insensitive for both valid and invalid statements.
func TestCreateStage_CaseInsensitive(t *testing.T) {
	validCases := []string{
		"create stage s",
		"Create Or Replace Stage s",
		"create or replace temporary stage s",
		"CREATE stage s url = 's3://bucket/'",
		"create stage s URL = 's3://bucket/' storage_integration = my_int",
		"create stage s file_format = (type = 'CSV' field_delimiter = ',')",
		"Create Stage s Encryption = (Type = 'SNOWFLAKE_FULL')",
		"create stage s copy_options = (on_error = continue)",
		"create stage s directory = (enable = true)",
		"create stage s comment = 'test'",
	}
	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}

	invalidCases := []struct {
		sql           string
		expectedMatch string
	}{
		{"create stage s type = 'CSV'", "TYPE"},
		{"Create Stage s field_delimiter = ','", "FIELD_DELIMITER"},
		{"create stage s skip_header = 1", "SKIP_HEADER"},
		{"CREATE stage s master_key = 'key'", "MASTER_KEY"},
	}
	for _, tt := range invalidCases {
		t.Run("invalid/"+tt.sql[:min(len(tt.sql), 50)], func(t *testing.T) {
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

// TestAlterStage_CaseInsensitive verifies that ALTER STAGE keyword matching
// is case-insensitive for both valid and invalid statements.
func TestAlterStage_CaseInsensitive(t *testing.T) {
	validCases := []string{
		"alter stage s set comment = 'test'",
		"Alter Stage s Set Comment = 'test'",
		"ALTER stage s set url = 's3://bucket/'",
		"alter stage s set file_format = (type = 'CSV')",
		"alter stage s set storage_integration = my_int",
		"alter stage s set encryption = (type = 'NONE')",
		"alter stage s set credentials = (aws_key_id = 'k' aws_secret_key = 's')",
		"alter stage s set directory = (enable = true)",
		"alter stage s rename to new_s",
		"alter stage s refresh",
		"alter stage s refresh subpath = 'data/'",
		"alter stage if exists s set comment = 'updated'",
		"Alter Stage If Exists s Set Url = 's3://bucket/'",
	}
	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnMsgs(warnings))
			}
		})
	}

	invalidCases := []struct {
		sql           string
		expectedMatch string
	}{
		{"alter stage s set type = 'CSV'", "TYPE"},
		{"Alter Stage s Set field_delimiter = ','", "FIELD_DELIMITER"},
		{"alter stage s set skip_header = 1", "SKIP_HEADER"},
		{"ALTER stage s set master_key = 'key'", "MASTER_KEY"},
		{"alter stage if exists s set on_error = continue", "ON_ERROR"},
	}
	for _, tt := range invalidCases {
		t.Run("invalid/"+tt.sql[:min(len(tt.sql), 50)], func(t *testing.T) {
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

// TestStage_MultiLine verifies that stage statements spanning multiple lines
// are validated correctly.
func TestStage_MultiLine(t *testing.T) {
	validCases := []string{
		"CREATE STAGE s\n  URL = 's3://bucket/'",
		"CREATE STAGE s\n  URL = 's3://bucket/'\n  STORAGE_INTEGRATION = my_int",
		"CREATE OR REPLACE STAGE s\n  FILE_FORMAT = (\n    TYPE = 'CSV'\n    FIELD_DELIMITER = ','\n    SKIP_HEADER = 1\n  )\n  COMMENT = 'multiline stage'",
		"ALTER STAGE s SET\n  FILE_FORMAT = (TYPE = 'CSV')\n  COMMENT = 'updated'",
		"ALTER STAGE s SET\n  URL = 's3://new-bucket/'\n  STORAGE_INTEGRATION = new_int",
	}
	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}

	invalidCases := []struct {
		name          string
		sql           string
		expectedMatch string
	}{
		{"multiline TYPE at top level", "CREATE STAGE s\n  TYPE = 'CSV'", "TYPE"},
		{"multiline FIELD_DELIMITER at top level", "CREATE STAGE s\n  FIELD_DELIMITER = ','", "FIELD_DELIMITER"},
		{"multiline ALTER SET SKIP_HEADER", "ALTER STAGE s SET\n  SKIP_HEADER = 1", "SKIP_HEADER"},
	}
	for _, tt := range invalidCases {
		t.Run("invalid/"+tt.name, func(t *testing.T) {
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

// TestStage_TrailingSemicolon verifies that trailing semicolons don't interfere
// with validation.
func TestStage_TrailingSemicolon(t *testing.T) {
	validCases := []string{
		"CREATE STAGE s;",
		"CREATE STAGE s URL = 's3://bucket/';",
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV');",
		"ALTER STAGE s SET COMMENT = 'test';",
		"ALTER STAGE s REFRESH;",
	}
	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}

	invalidCases := []struct {
		sql           string
		expectedMatch string
	}{
		{"CREATE STAGE s TYPE = 'CSV';", "TYPE"},
		{"ALTER STAGE s SET FIELD_DELIMITER = ',';", "FIELD_DELIMITER"},
	}
	for _, tt := range invalidCases {
		t.Run("invalid/"+tt.sql[:min(len(tt.sql), 50)], func(t *testing.T) {
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

// TestStage_EmbeddedComments verifies that line comments (--) and block
// comments (/* */) inside stage statements are stripped before validation.
func TestStage_EmbeddedComments(t *testing.T) {
	validCases := []string{
		"CREATE STAGE s /* internal stage */ URL = 's3://bucket/'",
		"CREATE STAGE s\n  -- set the URL\n  URL = 's3://bucket/'",
		"ALTER STAGE s SET /* update format */ FILE_FORMAT = (TYPE = 'CSV')",
		"ALTER STAGE s SET\n  -- changing URL\n  URL = 's3://new-bucket/'",
		"CREATE STAGE s URL = 's3://bucket/' /* with integration */ STORAGE_INTEGRATION = my_int",
	}
	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}
}

// TestStage_MultiStatement verifies that multiple stage statements in a single
// SQL input are each validated independently.
func TestStage_MultiStatement(t *testing.T) {
	// Both valid
	sql := "CREATE STAGE s1 URL = 's3://bucket/'; CREATE STAGE s2 URL = 'gcs://bucket/'"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warnings := getWarnings(markers)
	if len(warnings) > 0 {
		t.Errorf("Expected 0 warnings for two valid stages, got %d: %v", len(warnings), warnings)
	}

	// First valid, second invalid
	sql = "CREATE STAGE s1 URL = 's3://bucket/'; CREATE STAGE s2 TYPE = 'CSV'"
	ranges = GetStatementRanges(sql)
	markers = ValidateSnowflakePatterns(sql, ranges)
	warnings = getWarnings(markers)
	if len(warnings) != 1 {
		t.Fatalf("Expected 1 warning for second invalid stage, got %d", len(warnings))
	}
	if !strings.Contains(strings.ToUpper(warnings[0].Message), "TYPE") {
		t.Errorf("Expected warning about TYPE, got: %v", warnings[0].Message)
	}

	// Both invalid
	sql = "CREATE STAGE s1 SKIP_HEADER = 1; CREATE STAGE s2 TYPE = 'CSV'"
	ranges = GetStatementRanges(sql)
	markers = ValidateSnowflakePatterns(sql, ranges)
	warnings = getWarnings(markers)
	if len(warnings) != 2 {
		t.Fatalf("Expected 2 warnings for two invalid stages, got %d", len(warnings))
	}

	// Mix of CREATE and ALTER
	sql = "CREATE STAGE s1 URL = 's3://bucket/'; ALTER STAGE s1 SET COMMENT = 'updated'"
	ranges = GetStatementRanges(sql)
	markers = ValidateSnowflakePatterns(sql, ranges)
	warnings = getWarnings(markers)
	if len(warnings) > 0 {
		t.Errorf("Expected 0 warnings for valid CREATE+ALTER, got %d: %v", len(warnings), warnings)
	}
}

// TestStage_LeadingWhitespace verifies that leading whitespace and indentation
// don't prevent pattern matching.
func TestStage_LeadingWhitespace(t *testing.T) {
	validCases := []string{
		"  CREATE STAGE s",
		"\tCREATE STAGE s",
		"\n\nCREATE STAGE s",
		"   ALTER STAGE s SET COMMENT = 'test'",
		"\t\tALTER STAGE s REFRESH",
	}
	for _, sql := range validCases {
		t.Run(strings.TrimSpace(sql)[:min(len(strings.TrimSpace(sql)), 50)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}
}

// TestStage_MultipleInvalidProperties verifies that the validator reports ALL
// invalid properties in a single statement, not just the first.
func TestStage_MultipleInvalidProperties(t *testing.T) {
	// Two invalid properties in one CREATE STAGE
	sql := "CREATE STAGE s SKIP_HEADER = 1 TYPE = 'CSV'"
	ranges := GetStatementRanges(sql)
	markers := ValidateSnowflakePatterns(sql, ranges)
	warnings := getWarnings(markers)
	if len(warnings) != 2 {
		t.Fatalf("Expected 2 warnings for two invalid props, got %d: %v", len(warnings), warnMsgs(warnings))
	}

	// Three invalid properties in one CREATE STAGE
	sql = "CREATE STAGE s FIELD_DELIMITER = ',' SKIP_HEADER = 1 MASTER_KEY = 'key'"
	ranges = GetStatementRanges(sql)
	markers = ValidateSnowflakePatterns(sql, ranges)
	warnings = getWarnings(markers)
	if len(warnings) != 3 {
		t.Fatalf("Expected 3 warnings for three invalid props, got %d: %v", len(warnings), warnMsgs(warnings))
	}

	// Two invalid properties in ALTER STAGE SET
	sql = "ALTER STAGE s SET SKIP_HEADER = 1 TYPE = 'CSV'"
	ranges = GetStatementRanges(sql)
	markers = ValidateSnowflakePatterns(sql, ranges)
	warnings = getWarnings(markers)
	if len(warnings) != 2 {
		t.Fatalf("Expected 2 warnings for two invalid ALTER props, got %d: %v", len(warnings), warnMsgs(warnings))
	}
}

// TestStage_MixedValidAndInvalidProperties verifies that valid top-level
// properties pass while invalid ones in the same statement are flagged.
func TestStage_MixedValidAndInvalidProperties(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		wantCount     int
		expectedMatch string
	}{
		{
			"CREATE valid URL + invalid SKIP_HEADER",
			"CREATE STAGE s URL = 's3://bucket/' SKIP_HEADER = 1",
			1, "SKIP_HEADER",
		},
		{
			"CREATE valid URL + COMMENT + invalid TYPE",
			"CREATE STAGE s URL = 's3://bucket/' COMMENT = 'x' TYPE = 'CSV'",
			1, "TYPE",
		},
		{
			"ALTER valid COMMENT + invalid FIELD_DELIMITER",
			"ALTER STAGE s SET COMMENT = 'test' FIELD_DELIMITER = ','",
			1, "FIELD_DELIMITER",
		},
		{
			"CREATE valid FILE_FORMAT block + invalid MASTER_KEY",
			"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' SKIP_HEADER = 1) MASTER_KEY = 'key'",
			1, "MASTER_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) != tt.wantCount {
				t.Fatalf("Expected %d warning(s) for %q, got %d: %v", tt.wantCount, tt.sql, len(warnings), warnMsgs(warnings))
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

// TestStage_ModifiersWithInvalidProperty verifies that CREATE/ALTER modifiers
// (IF NOT EXISTS, OR REPLACE, TEMPORARY, IF EXISTS) don't prevent detection
// of invalid top-level properties.
func TestStage_ModifiersWithInvalidProperty(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectedMatch string
	}{
		{"IF NOT EXISTS + invalid", "CREATE STAGE IF NOT EXISTS s TYPE = 'CSV'", "TYPE"},
		{"OR REPLACE + invalid", "CREATE OR REPLACE STAGE s SKIP_HEADER = 1", "SKIP_HEADER"},
		{"TEMPORARY + invalid", "CREATE TEMPORARY STAGE s FIELD_DELIMITER = ','", "FIELD_DELIMITER"},
		{"OR REPLACE TEMPORARY + invalid", "CREATE OR REPLACE TEMPORARY STAGE s MASTER_KEY = 'k'", "MASTER_KEY"},
		{"OR REPLACE IF NOT EXISTS + invalid", "CREATE STAGE IF NOT EXISTS s ON_ERROR = CONTINUE", "ON_ERROR"},
		{"ALTER IF EXISTS + invalid TYPE", "ALTER STAGE IF EXISTS s SET TYPE = 'CSV'", "TYPE"},
		{"ALTER IF EXISTS + invalid SKIP_HEADER", "ALTER STAGE IF EXISTS s SET SKIP_HEADER = 1", "SKIP_HEADER"},
		{"ALTER IF EXISTS + invalid MASTER_KEY", "ALTER STAGE IF EXISTS s SET MASTER_KEY = 'key'", "MASTER_KEY"},
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

// TestStage_EmptyAndWhitespaceInput verifies that empty, whitespace-only, and
// non-stage SQL inputs produce zero stage-related warnings.
func TestStage_EmptyAndWhitespaceInput(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"empty string", ""},
		{"whitespace only", "   \t\n  "},
		{"SELECT statement", "SELECT 1"},
		{"CREATE TABLE", "CREATE TABLE t (id INT)"},
		{"INSERT", "INSERT INTO t VALUES (1)"},
		{"DROP STAGE", "DROP STAGE IF EXISTS s"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for non-stage SQL %q, got %d: %v", tt.sql, len(warnings), warnMsgs(warnings))
			}
		})
	}
}

// TestStage_PropertyPatternsInsideStringValues verifies that property-like
// patterns (KEY = value) inside single-quoted string literals do not trigger
// false-positive warnings. This tests the reStripStringLiterals preprocessing.
func TestStage_PropertyPatternsInsideStringValues(t *testing.T) {
	validCases := []string{
		// COMMENT values containing property-like patterns
		"CREATE STAGE s COMMENT = 'TYPE = CSV'",
		"CREATE STAGE s COMMENT = 'SKIP_HEADER = 1 and FIELD_DELIMITER = comma'",
		"CREATE STAGE s URL = 's3://bucket/' COMMENT = 'ENCRYPTION = NONE is recommended'",
		"ALTER STAGE s SET COMMENT = 'set TYPE = JSON for best results'",
		// Escaped quotes inside string values with property-like content
		"CREATE STAGE s COMMENT = 'it''s a TYPE = CSV test'",
		"ALTER STAGE s SET COMMENT = 'field''s DELIMITER = none'",
		// Multiple string values with property-like content
		"CREATE STAGE s URL = 's3://bucket/KEY_ID=test/' COMMENT = 'MASTER_KEY = hidden'",
		// Property-like patterns inside nested block string values
		"CREATE STAGE s FILE_FORMAT = (TYPE = 'CSV' FIELD_DELIMITER = 'SKIP_HEADER = 1')",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnMsgs(warnings))
			}
		})
	}
}

// TestAlterStage_SkipValidationWithPropertyLikeNames verifies that ALTER STAGE
// forms excluded from property validation (RENAME TO, SET TAG, UNSET TAG) work
// correctly even when identifiers resemble invalid property names.
func TestAlterStage_SkipValidationWithPropertyLikeNames(t *testing.T) {
	validCases := []string{
		// SET TAG where tag_name = value would match reProp without the skip
		"ALTER STAGE s SET TAG type = 'value'",
		"ALTER STAGE s SET TAG compression = 'gzip', skip_header = '1'",
		"ALTER STAGE IF EXISTS s SET TAG field_delimiter = 'comma'",
		// RENAME TO with property-keyword-like destination
		"ALTER STAGE s RENAME TO type_stage",
		"ALTER STAGE s RENAME TO skip_header_stage",
		// UNSET TAG with property-like tag names
		"ALTER STAGE s UNSET TAG type",
		"ALTER STAGE s UNSET TAG field_delimiter, skip_header",
	}
	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnMsgs(warnings))
			}
		})
	}
}

// TestStage_CompactEqualsSyntax verifies that property detection works when
// there is no whitespace around the = sign (e.g. URL='s3://bucket/').
func TestStage_CompactEqualsSyntax(t *testing.T) {
	validCases := []string{
		"CREATE STAGE s URL='s3://bucket/'",
		"CREATE STAGE s URL= 's3://bucket/'",
		"CREATE STAGE s URL ='s3://bucket/'",
		"CREATE STAGE s COMMENT='test stage'",
		"CREATE STAGE s FILE_FORMAT=(TYPE='CSV')",
		"CREATE STAGE s URL='s3://bucket/' STORAGE_INTEGRATION=my_int",
		"ALTER STAGE s SET URL='s3://bucket/'",
		"ALTER STAGE s SET COMMENT='updated'",
		"ALTER STAGE s SET FILE_FORMAT=(TYPE='CSV' FIELD_DELIMITER=',')",
	}
	for _, sql := range validCases {
		t.Run("valid/"+sql[:min(len(sql), 55)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnMsgs(warnings))
			}
		})
	}

	invalidCases := []struct {
		sql           string
		expectedMatch string
	}{
		{"CREATE STAGE s TYPE='CSV'", "TYPE"},
		{"CREATE STAGE s SKIP_HEADER=1", "SKIP_HEADER"},
		{"ALTER STAGE s SET FIELD_DELIMITER=','", "FIELD_DELIMITER"},
		{"ALTER STAGE s SET MASTER_KEY='key'", "MASTER_KEY"},
	}
	for _, tt := range invalidCases {
		t.Run("invalid/"+tt.sql[:min(len(tt.sql), 55)], func(t *testing.T) {
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

// TestStage_PropertyListDivergence verifies that properties valid for CREATE
// STAGE but not ALTER STAGE (and vice versa) are correctly flagged.
// ENDPOINT is only valid in CREATE STAGE; SUBPATH is only valid in ALTER STAGE.
func TestStage_PropertyListDivergence(t *testing.T) {
	// ENDPOINT valid in CREATE, invalid in ALTER
	t.Run("ENDPOINT valid in CREATE", func(t *testing.T) {
		sql := "CREATE STAGE s URL = 's3compat://bucket/' ENDPOINT = 'storage.example.com'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings, got %d: %v", len(warnings), warnMsgs(warnings))
		}
	})
	t.Run("ENDPOINT invalid in ALTER", func(t *testing.T) {
		sql := "ALTER STAGE s SET ENDPOINT = 'storage.example.com'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected warning for ENDPOINT in ALTER STAGE, got 0")
		}
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToUpper(w.Message), "ENDPOINT") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected warning matching ENDPOINT, got: %v", warnings[0].Message)
		}
	})

	// SUBPATH valid in ALTER (REFRESH SUBPATH), invalid in CREATE
	t.Run("SUBPATH valid in ALTER", func(t *testing.T) {
		sql := "ALTER STAGE s REFRESH SUBPATH = 'data/2024/'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings, got %d: %v", len(warnings), warnMsgs(warnings))
		}
	})
	t.Run("SUBPATH invalid in CREATE", func(t *testing.T) {
		sql := "CREATE STAGE s SUBPATH = 'data/'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected warning for SUBPATH in CREATE STAGE, got 0")
		}
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToUpper(w.Message), "SUBPATH") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected warning matching SUBPATH, got: %v", warnings[0].Message)
		}
	})
}

// TestStage_EmptySetClause verifies that ALTER STAGE SET with no properties
// after the SET keyword produces zero warnings.
func TestStage_EmptySetClause(t *testing.T) {
	cases := []string{
		"ALTER STAGE s SET",
		"ALTER STAGE IF EXISTS s SET",
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnMsgs(warnings))
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
		{"AWS_SECRET_KEY at top level", "ALTER STAGE s SET AWS_SECRET_KEY = 'secret'", "AWS_SECRET_KEY"},
		{"AWS_TOKEN at top level", "ALTER STAGE s SET AWS_TOKEN = 'tok'", "AWS_TOKEN"},
		{"AWS_ROLE at top level", "ALTER STAGE s SET AWS_ROLE = 'arn:role'", "AWS_ROLE"},
		{"AZURE_SAS_TOKEN at top level", "ALTER STAGE s SET AZURE_SAS_TOKEN = 'sas'", "AZURE_SAS_TOKEN"},
		// DIRECTORY sub-options at top level
		{"ENABLE at top level in SET", "ALTER STAGE s SET ENABLE = TRUE", "ENABLE"},
		{"AUTO_REFRESH at top level", "ALTER STAGE s SET AUTO_REFRESH = TRUE", "AUTO_REFRESH"},
		{"REFRESH_ON_CREATE at top level", "ALTER STAGE s SET REFRESH_ON_CREATE = TRUE", "REFRESH_ON_CREATE"},
		{"NOTIFICATION_INTEGRATION at top level", "ALTER STAGE s SET NOTIFICATION_INTEGRATION = 'ni'", "NOTIFICATION_INTEGRATION"},
		// COPY_OPTIONS sub-options at top level
		{"ON_ERROR at top level", "ALTER STAGE s SET ON_ERROR = CONTINUE", "ON_ERROR"},
		{"SIZE_LIMIT at top level", "ALTER STAGE s SET SIZE_LIMIT = 100", "SIZE_LIMIT"},
		// XML sub-options at top level
		{"PRESERVE_SPACE at top level", "ALTER STAGE s SET PRESERVE_SPACE = TRUE", "PRESERVE_SPACE"},
		{"STRIP_OUTER_ELEMENT at top level", "ALTER STAGE s SET STRIP_OUTER_ELEMENT = TRUE", "STRIP_OUTER_ELEMENT"},
		// Parity with CREATE invalid: properties missing from ALTER tests
		{"S3_URL in SET", "ALTER STAGE s SET S3_URL = 's3://bucket/'", "S3_URL"},
		{"WAREHOUSE in SET", "ALTER STAGE s SET WAREHOUSE = 'WH'", "WAREHOUSE"},
		{"MAX_FILE_SIZE in SET", "ALTER STAGE s SET MAX_FILE_SIZE = 100", "MAX_FILE_SIZE"},
		{"FORCE in SET", "ALTER STAGE s SET FORCE = TRUE", "FORCE"},
		{"PURGE in SET", "ALTER STAGE s SET PURGE = TRUE", "PURGE"},
		{"TIMESTAMP_FORMAT in SET", "ALTER STAGE s SET TIMESTAMP_FORMAT = 'AUTO'", "TIMESTAMP_FORMAT"},
		{"COMPRESSION in SET", "ALTER STAGE s SET COMPRESSION = 'GZIP'", "COMPRESSION"},
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


