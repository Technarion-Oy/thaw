package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_CreateExternalVolume(t *testing.T) {
	validCases := []string{
		// Minimal valid S3 volume
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'my_s3' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://bucket/path' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::123:role/r' )) ALLOW_WRITES = TRUE",
		// S3GOV provider
		"CREATE EXTERNAL VOLUME gov_vol STORAGE_LOCATIONS = (( NAME = 'gov' STORAGE_PROVIDER = 'S3GOV' STORAGE_BASE_URL = 's3://gov-bucket/' STORAGE_AWS_ROLE_ARN = 'arn:aws-us-gov:iam::123:role/r' ))",
		// S3CHINA provider
		"CREATE EXTERNAL VOLUME cn_vol STORAGE_LOCATIONS = (( NAME = 'cn' STORAGE_PROVIDER = 'S3CHINA' STORAGE_BASE_URL = 's3://cn-bucket/' STORAGE_AWS_ROLE_ARN = 'arn:aws-cn:iam::123:role/r' ))",
		// S3COMPAT provider (S3-compatible storage)
		"CREATE EXTERNAL VOLUME compat_vol STORAGE_LOCATIONS = (( NAME = 'compat' STORAGE_PROVIDER = 'S3COMPAT' STORAGE_BASE_URL = 's3compat://endpoint/bucket' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
		// GCS provider
		"CREATE EXTERNAL VOLUME gcs_vol STORAGE_LOCATIONS = (( NAME = 'gcs' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://bucket/path' ))",
		// AZURE provider
		"CREATE EXTERNAL VOLUME az_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://account.blob.core.windows.net/container/path' AZURE_TENANT_ID = 'tenant-id' ))",
		// OR REPLACE is valid for external volumes
		"CREATE OR REPLACE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
		// IF NOT EXISTS
		"CREATE EXTERNAL VOLUME IF NOT EXISTS my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
		// S3 with optional STORAGE_AWS_EXTERNAL_ID
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' STORAGE_AWS_EXTERNAL_ID = 'ext-id' ))",
		// S3COMPAT with STORAGE_AWS_EXTERNAL_ID (same as S3)
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3COMPAT' STORAGE_BASE_URL = 's3compat://ep/b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' STORAGE_AWS_EXTERNAL_ID = 'ext-id' ))",
		// S3 with ENCRYPTION TYPE = NONE
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'NONE') ))",
		// S3 with ENCRYPTION TYPE = AWS_SSE_S3
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'AWS_SSE_S3') ))",
		// S3 with ENCRYPTION TYPE = AWS_SSE_KMS
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'AWS_SSE_KMS') ))",
		// GCS with ENCRYPTION TYPE = GCS_SSE_KMS
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = (TYPE = 'GCS_SSE_KMS') ))",
		// GCS with ENCRYPTION TYPE = NONE (provider-agnostic, valid for GCS)
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'gcs' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = (TYPE = 'NONE') ))",
		// S3GOV with STORAGE_AWS_EXTERNAL_ID (S3-family, valid)
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3GOV' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws-us-gov:iam::1:role/r' STORAGE_AWS_EXTERNAL_ID = 'ext-id' ))",
		// S3CHINA with STORAGE_AWS_EXTERNAL_ID (S3-family, valid)
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3CHINA' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws-cn:iam::1:role/r' STORAGE_AWS_EXTERNAL_ID = 'ext-id' ))",
		// ALLOW_WRITES = FALSE
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' )) ALLOW_WRITES = FALSE",
		// COMMENT property
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) COMMENT = 'my volume'",
		// Multi-provider: S3 + GCS — both ARN (for S3) and GCS_SSE_KMS (for GCS) valid, no AZURE_TENANT_ID error
		"CREATE EXTERNAL VOLUME multi_vol STORAGE_LOCATIONS = (( NAME = 's3loc' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ) ( NAME = 'gcsloc' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ))",
		// Multi-provider: S3 + AZURE — STORAGE_AWS_EXTERNAL_ID remains valid (hasS3=true)
		"CREATE EXTERNAL VOLUME multi_vol STORAGE_LOCATIONS = (( NAME = 's3loc' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' STORAGE_AWS_EXTERNAL_ID = 'eid' ) ( NAME = 'azloc' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://acc.blob.core.windows.net/c/' AZURE_TENANT_ID = 'tid' ))",
		// Lowercase provider string — case-insensitive match should accept 's3'
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 's3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
		// Quoted volume name containing a dot — should NOT flag account-level prefix
		`CREATE EXTERNAL VOLUME "my.vol" STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))`,
		// Quoted reserved-keyword volume name — identPath regex must handle double-quoted identifiers
		`CREATE EXTERNAL VOLUME "S3" STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))`,
		// ALLOW_WRITES in a line comment must not trigger a false positive
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) -- ALLOW_WRITES = maybe",
		// ALLOW_WRITES inside a COMMENT string value must not trigger a false positive
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) COMMENT = 'do not set ALLOW_WRITES = MAYBE here'",
	}

	for _, sql := range validCases {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
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
			"Missing STORAGE_LOCATIONS",
			"CREATE EXTERNAL VOLUME my_vol ALLOW_WRITES = TRUE",
			[]string{"STORAGE_LOCATIONS is mandatory"},
		},
		{
			"Missing STORAGE_PROVIDER",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_BASE_URL = 's3://b/' ))",
			[]string{"Each storage location requires STORAGE_PROVIDER"},
		},
		{
			"Invalid STORAGE_PROVIDER",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'INVALID' STORAGE_BASE_URL = 's3://b/' ))",
			[]string{"Invalid STORAGE_PROVIDER 'INVALID'"},
		},
		{
			"Missing STORAGE_BASE_URL",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
			[]string{"Each storage location requires STORAGE_BASE_URL"},
		},
		{
			"Missing STORAGE_AWS_ROLE_ARN for S3",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' ))",
			[]string{"STORAGE_AWS_ROLE_ARN is required for S3"},
		},
		{
			"Missing STORAGE_AWS_ROLE_ARN for S3GOV",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3GOV' STORAGE_BASE_URL = 's3://b/' ))",
			[]string{"STORAGE_AWS_ROLE_ARN is required for S3"},
		},
		{
			"Missing STORAGE_AWS_ROLE_ARN for S3CHINA",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3CHINA' STORAGE_BASE_URL = 's3://b/' ))",
			[]string{"STORAGE_AWS_ROLE_ARN is required for S3"},
		},
		{
			"Missing STORAGE_AWS_ROLE_ARN for S3COMPAT",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3COMPAT' STORAGE_BASE_URL = 's3compat://ep/b/' ))",
			[]string{"STORAGE_AWS_ROLE_ARN is required for S3"},
		},
		{
			"Missing AZURE_TENANT_ID for AZURE",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://account.blob.core.windows.net/container/' ))",
			[]string{"AZURE_TENANT_ID is required for AZURE"},
		},
		{
			"STORAGE_AWS_EXTERNAL_ID with non-S3 provider",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' STORAGE_AWS_EXTERNAL_ID = 'id' ))",
			[]string{"STORAGE_AWS_EXTERNAL_ID is only valid for S3"},
		},
		{
			"Invalid ENCRYPTION TYPE",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'INVALID_ENC') ))",
			[]string{"Invalid ENCRYPTION TYPE 'INVALID_ENC'"},
		},
		{
			"AWS_SSE_S3 encryption with GCS provider",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = (TYPE = 'AWS_SSE_S3') ))",
			[]string{"ENCRYPTION TYPE 'AWS_SSE_S3' is only valid for S3"},
		},
		{
			"AWS_SSE_KMS encryption with GCS provider",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = (TYPE = 'AWS_SSE_KMS') ))",
			[]string{"ENCRYPTION TYPE 'AWS_SSE_KMS' is only valid for S3"},
		},
		{
			"GCS_SSE_KMS encryption with S3 provider",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'GCS_SSE_KMS') ))",
			[]string{"ENCRYPTION TYPE 'GCS_SSE_KMS' is only valid for GCS"},
		},
		{
			"Invalid ALLOW_WRITES value",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) ALLOW_WRITES = MAYBE",
			[]string{"ALLOW_WRITES must be TRUE or FALSE"},
		},
		{
			"Account-level prefix not allowed",
			"CREATE EXTERNAL VOLUME mydb.myschema.my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		{
			"OR REPLACE and IF NOT EXISTS conflict",
			"CREATE OR REPLACE EXTERNAL VOLUME IF NOT EXISTS my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
		},
		{
			"Cross-provider: GCS_SSE_KMS on S3 location when GCS also present",
			"CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 's3' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'GCS_SSE_KMS') ) ( NAME = 'gcs' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ))",
			[]string{"ENCRYPTION TYPE 'GCS_SSE_KMS' is only valid for GCS"},
		},
		{
			"Cross-provider: STORAGE_AWS_EXTERNAL_ID on GCS location when S3 also present",
			"CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 's3' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ) ( NAME = 'gcs' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' STORAGE_AWS_EXTERNAL_ID = 'id' ))",
			[]string{"STORAGE_AWS_EXTERNAL_ID is only valid for S3"},
		},
		{
			"AZURE location with invalid ENCRYPTION TYPE",
			"CREATE EXTERNAL VOLUME az_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://account.blob.core.windows.net/container/' AZURE_TENANT_ID = 'tid' ENCRYPTION = (TYPE = 'AZURE_CSE') ))",
			[]string{"AZURE storage locations do not support the ENCRYPTION parameter"},
		},
		{
			"AZURE location with AWS encryption type",
			"CREATE EXTERNAL VOLUME az_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://account.blob.core.windows.net/container/' AZURE_TENANT_ID = 'tid' ENCRYPTION = (TYPE = 'AWS_SSE_S3') ))",
			[]string{"AZURE storage locations do not support the ENCRYPTION parameter"},
		},
		{
			"AZURE location with ENCRYPTION TYPE = NONE",
			"CREATE EXTERNAL VOLUME az_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://account.blob.core.windows.net/container/' AZURE_TENANT_ID = 'tid' ENCRYPTION = (TYPE = 'NONE') ))",
			[]string{"AZURE storage locations do not support the ENCRYPTION parameter"},
		},
		{
			"Missing NAME in location block",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
			[]string{"Each storage location requires a NAME attribute"},
		},
		{
			"Missing both STORAGE_PROVIDER and STORAGE_BASE_URL reports both errors",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' ))",
			[]string{"Each storage location requires STORAGE_BASE_URL", "Each storage location requires STORAGE_PROVIDER"},
		},
		{
			"AZURE with ENCRYPTION block but no TYPE key",
			"CREATE EXTERNAL VOLUME az_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://acc.blob.core.windows.net/c/' AZURE_TENANT_ID = 'tid' ENCRYPTION = (KMS_KEY_ID = 'k') ))",
			[]string{"AZURE storage locations do not support the ENCRYPTION parameter"},
		},
		{
			"S3 with ENCRYPTION block but no TYPE key",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (KMS_KEY_ID = 'k') ))",
			[]string{"ENCRYPTION block must specify a TYPE key"},
		},
		{
			"Empty STORAGE_LOCATIONS block",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = ()",
			[]string{"STORAGE_LOCATIONS must contain at least one storage location block"},
		},
		{
			"Unmatched paren in STORAGE_LOCATIONS (missing closing paren)",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = ((NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/'",
			[]string{"STORAGE_LOCATIONS must contain at least one storage location block"},
		},
		{
			"OR REPLACE and IF NOT EXISTS returns early without extra markers",
			"CREATE OR REPLACE EXTERNAL VOLUME IF NOT EXISTS my_vol ALLOW_WRITES = TRUE",
			[]string{"Conflict between OR REPLACE and IF NOT EXISTS"},
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
		sql := "CREATE OR REPLACE EXTERNAL VOLUME IF NOT EXISTS my_vol ALLOW_WRITES = TRUE"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning (early return), got %d: %v", len(warns), warns)
		}
	})
}


