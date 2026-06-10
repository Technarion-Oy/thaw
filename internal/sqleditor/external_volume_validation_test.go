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
		// ALLOW_WRITES in a block comment must not trigger a false positive
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) /* ALLOW_WRITES = MAYBE */",
		// Multi-line block comment must not trigger a false positive
		`CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) /*
  ALLOW_WRITES = MAYBE
  STORAGE_PROVIDER = 'INVALID'
*/`,
		// Fully lowercase SQL keywords — case-insensitive regex must still match
		"create external volume my_vol storage_locations = (( name = 'n' storage_provider = 'S3' storage_base_url = 's3://b/' storage_aws_role_arn = 'arn:aws:iam::1:role/r' ))",
		// Mixed-case SQL keywords
		"Create Or Replace External Volume my_vol Storage_Locations = (( Name = 'n' Storage_Provider = 'S3' Storage_Base_Url = 's3://b/' Storage_Aws_Role_Arn = 'arn:aws:iam::1:role/r' ))",
		// Lowercase encryption type — case-insensitive validation should accept it
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'aws_sse_kms') ))",
		// Mixed-case encryption type
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'Aws_Sse_S3') ))",
		// S3COMPAT with AWS_SSE_S3 encryption — encryption must work for all S3-family providers
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3COMPAT' STORAGE_BASE_URL = 's3compat://ep/b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'AWS_SSE_S3') ))",
		// S3GOV with AWS_SSE_KMS encryption
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3GOV' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws-us-gov:iam::1:role/r' ENCRYPTION = (TYPE = 'AWS_SSE_KMS') ))",
		// S3CHINA with ENCRYPTION TYPE = NONE
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3CHINA' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws-cn:iam::1:role/r' ENCRYPTION = (TYPE = 'NONE') ))",
		// Three-provider multi-location: S3 + GCS + AZURE all valid together
		"CREATE EXTERNAL VOLUME tri_vol STORAGE_LOCATIONS = (( NAME = 's3' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ) ( NAME = 'gcs' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ) ( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://acc.blob.core.windows.net/c/' AZURE_TENANT_ID = 'tid' ))",
		// Lowercase ALLOW_WRITES value — validateBoolProp uppercases before comparison
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) ALLOW_WRITES = true",
		// Mixed-case ALLOW_WRITES value
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) ALLOW_WRITES = False",
		// Extra STORAGE_AWS_ROLE_ARN on GCS location — validator is permissive about extra attributes
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
		// Extra AZURE_TENANT_ID on S3 location — validator is permissive about extra attributes
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' AZURE_TENANT_ID = 'tid' ))",
		// Multi-line formatting — real-world DDL style
		`CREATE EXTERNAL VOLUME my_vol
  STORAGE_LOCATIONS = ((
    NAME = 'my_s3'
    STORAGE_PROVIDER = 'S3'
    STORAGE_BASE_URL = 's3://bucket/path'
    STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::123:role/r'
  ))
  ALLOW_WRITES = TRUE`,
		// Parentheses inside STORAGE_BASE_URL string value — splitLocationBlocks must not treat them as block delimiters
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://bucket/path(1)/data' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::123:role/r' ))",
		// Parentheses inside NAME string value — findMatchingParen must skip parens inside single-quoted strings
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'loc(1)' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
		// Trailing semicolon — GetStatementRanges strips it before validation
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ));",
		// S3CHINA with AWS_SSE_S3 encryption — completes S3-family × encryption matrix
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3CHINA' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws-cn:iam::1:role/r' ENCRYPTION = (TYPE = 'AWS_SSE_S3') ))",
		// S3CHINA with AWS_SSE_KMS encryption
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3CHINA' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws-cn:iam::1:role/r' ENCRYPTION = (TYPE = 'AWS_SSE_KMS') ))",
		// S3COMPAT with AWS_SSE_KMS encryption
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3COMPAT' STORAGE_BASE_URL = 's3compat://ep/b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'AWS_SSE_KMS') ))",
		// S3 with ENCRYPTION containing extra KMS_KEY_ID alongside TYPE — extra attributes must not cause false positives
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'AWS_SSE_KMS' KMS_KEY_ID = 'arn:aws:kms:us-east-1:123:key/abc') ))",
		// GCS with ENCRYPTION containing extra KMS_KEY_ID alongside TYPE
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'gcs' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = (TYPE = 'GCS_SSE_KMS' KMS_KEY_ID = 'projects/p/locations/l/keyRings/kr/cryptoKeys/k') ))",
		// Double-quoted parentheses in STORAGE_BASE_URL — findMatchingParen must skip quoted strings
		`CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://bucket/path(1)/data' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::123:role/r' ))`,
		// Empty STORAGE_BASE_URL string — structural validator accepts it
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = '' ))",
		// Double-quoted identifier volume name with spaces
		`CREATE EXTERNAL VOLUME "My Volume" STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))`,
		// COMMENT containing STORAGE_PROVIDER keyword — must not interfere with validation
		"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) COMMENT = 'uses STORAGE_PROVIDER S3'",
		// Tab and newline separated attributes — whitespace-agnostic regex
		"CREATE EXTERNAL VOLUME my_vol\n\tSTORAGE_LOCATIONS = ((\n\t\tNAME = 'n'\n\t\tSTORAGE_PROVIDER = 'S3'\n\t\tSTORAGE_BASE_URL = 's3://b/'\n\t\tSTORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r'\n\t))",
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
			"S3 with empty ENCRYPTION block",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = () ))",
			[]string{"ENCRYPTION block must specify a TYPE key"},
		},
		{
			"GCS with empty ENCRYPTION block",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = () ))",
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
		{
			"GCS with ENCRYPTION block but no TYPE key",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = (KMS_KEY_ID = 'k') ))",
			[]string{"ENCRYPTION block must specify a TYPE key"},
		},
		{
			"STORAGE_AWS_EXTERNAL_ID with AZURE provider only (no S3 present)",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://acc.blob.core.windows.net/c/' AZURE_TENANT_ID = 'tid' STORAGE_AWS_EXTERNAL_ID = 'eid' ))",
			[]string{"STORAGE_AWS_EXTERNAL_ID is only valid for S3"},
		},
		{
			"Multi-location: second location missing STORAGE_AWS_ROLE_ARN",
			"CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 'ok' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ) ( NAME = 'bad' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b2/' ))",
			[]string{"STORAGE_AWS_ROLE_ARN is required for S3"},
		},
		{
			"Multi-location: one valid, one with invalid provider",
			"CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 'ok' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ) ( NAME = 'bad' STORAGE_PROVIDER = 'INVALID' STORAGE_BASE_URL = 's3://b2/' ))",
			[]string{"Invalid STORAGE_PROVIDER 'INVALID'"},
		},
		{
			"Location missing all three required attributes",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
			[]string{"Each storage location requires a NAME attribute", "Each storage location requires STORAGE_BASE_URL", "Each storage location requires STORAGE_PROVIDER"},
		},
		{
			"GCS_SSE_KMS encryption with AZURE provider",
			"CREATE EXTERNAL VOLUME az_vol STORAGE_LOCATIONS = (( NAME = 'az' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://acc.blob.core.windows.net/c/' AZURE_TENANT_ID = 'tid' ENCRYPTION = (TYPE = 'GCS_SSE_KMS') ))",
			[]string{"AZURE storage locations do not support the ENCRYPTION parameter"},
		},
		{
			"Two-part prefix (schema.name) not allowed for account-level object",
			"CREATE EXTERNAL VOLUME myschema.my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		{
			"Empty provider string",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = '' STORAGE_BASE_URL = 's3://b/' ))",
			[]string{"Invalid STORAGE_PROVIDER ''"},
		},
		{
			"Single-paren structure (missing inner parens)",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = ( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )",
			[]string{"STORAGE_LOCATIONS must contain at least one storage location block"},
		},
		{
			"GCS_SSE_KMS encryption with S3GOV provider",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3GOV' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws-us-gov:iam::1:role/r' ENCRYPTION = (TYPE = 'GCS_SSE_KMS') ))",
			[]string{"ENCRYPTION TYPE 'GCS_SSE_KMS' is only valid for GCS"},
		},
		{
			"GCS_SSE_KMS encryption with S3CHINA provider",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3CHINA' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws-cn:iam::1:role/r' ENCRYPTION = (TYPE = 'GCS_SSE_KMS') ))",
			[]string{"ENCRYPTION TYPE 'GCS_SSE_KMS' is only valid for GCS"},
		},
		{
			"GCS_SSE_KMS encryption with S3COMPAT provider",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3COMPAT' STORAGE_BASE_URL = 's3compat://ep/b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (TYPE = 'GCS_SSE_KMS') ))",
			[]string{"ENCRYPTION TYPE 'GCS_SSE_KMS' is only valid for GCS"},
		},
		{
			"Multi-location: both locations missing STORAGE_AWS_ROLE_ARN",
			"CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 'a' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b1/' ) ( NAME = 'b' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b2/' ))",
			[]string{"STORAGE_AWS_ROLE_ARN is required for S3"},
		},
		{
			"Multi-location: first missing provider (continue), second missing ARN still reported",
			"CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 'a' STORAGE_BASE_URL = 's3://b1/' ) ( NAME = 'b' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b2/' ))",
			[]string{"Each storage location requires STORAGE_PROVIDER", "STORAGE_AWS_ROLE_ARN is required for S3"},
		},
		{
			"Multi-location: first has invalid provider (continue), second missing AZURE_TENANT_ID still reported",
			"CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 'a' STORAGE_PROVIDER = 'INVALID' STORAGE_BASE_URL = 'x://b/' ) ( NAME = 'b' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://acc.blob.core.windows.net/c/' ))",
			[]string{"Invalid STORAGE_PROVIDER 'INVALID'", "AZURE_TENANT_ID is required for AZURE"},
		},
		{
			"Missing provider continue skips encryption validation in same location",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_BASE_URL = 's3://b/' ENCRYPTION = (TYPE = 'INVALID_TYPE') ))",
			[]string{"Each storage location requires STORAGE_PROVIDER"},
		},
		{
			"Invalid provider continue skips encryption and ARN validation in same location",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'INVALID' STORAGE_BASE_URL = 's3://b/' ENCRYPTION = (TYPE = 'BAD') STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))",
			[]string{"Invalid STORAGE_PROVIDER 'INVALID'"},
		},
		{
			"Missing STORAGE_LOCATIONS early return skips ALLOW_WRITES validation",
			"CREATE EXTERNAL VOLUME my_vol ALLOW_WRITES = MAYBE",
			[]string{"STORAGE_LOCATIONS is mandatory"},
		},
		// Note: TYPE not being the first key inside ENCRYPTION = (...) is now
		// correctly handled by the token-based parser. The test case was removed
		// because TYPE is found regardless of position.
		{
			"AWS_SSE_S3 encryption on GCS location in multi-provider with S3",
			"CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 's3' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ) ( NAME = 'gcs' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ENCRYPTION = (TYPE = 'AWS_SSE_S3') ))",
			[]string{"ENCRYPTION TYPE 'AWS_SSE_S3' is only valid for S3"},
		},
		{
			"Quoted three-part identifier triggers account-level prefix error",
			`CREATE EXTERNAL VOLUME "mydb"."myschema"."my_vol" STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))`,
			[]string{"account-level objects and cannot have a database or schema prefix"},
		},
		{
			"ALLOW_WRITES numeric string invalid",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) ALLOW_WRITES = 1",
			[]string{"ALLOW_WRITES must be TRUE or FALSE"},
		},
		{
			"ALLOW_WRITES YES invalid",
			"CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) ALLOW_WRITES = YES",
			[]string{"ALLOW_WRITES must be TRUE or FALSE"},
		},
		{
			"Duplicate NAME in separate locations still validates each independently",
			"CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 'dup' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' ) ( NAME = 'dup' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ))",
			[]string{"STORAGE_AWS_ROLE_ARN is required for S3"},
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

	// Multi-statement: valid external volume preceded by unrelated statement.
	t.Run("Multi-statement: valid external volume with adjacent SELECT", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		if warns := getWarnings(markers); len(warns) > 0 {
			t.Errorf("Expected 0 warnings, got %d: %v", len(warns), warns)
		}
	})

	// Multi-statement: error only in the external volume statement, not the adjacent one.
	t.Run("Multi-statement: error in external volume only", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "STORAGE_AWS_ROLE_ARN is required") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected warning about missing STORAGE_AWS_ROLE_ARN, got: %v", warns)
		}
	})

	// Missing provider continue produces exactly one warning (no encryption error leaked).
	t.Run("Missing provider continue emits only provider warning", func(t *testing.T) {
		sql := "CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_BASE_URL = 's3://b/' ENCRYPTION = (TYPE = 'INVALID_TYPE') ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning (missing provider only, encryption skipped), got %d: %v", len(warns), warns)
		}
	})

	// Invalid provider continue produces exactly one warning (encryption and ARN errors skipped).
	t.Run("Invalid provider continue emits only provider warning", func(t *testing.T) {
		sql := "CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'INVALID' STORAGE_BASE_URL = 's3://b/' ENCRYPTION = (TYPE = 'BAD') STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning (invalid provider only, encryption/ARN skipped), got %d: %v", len(warns), warns)
		}
	})

	// Missing STORAGE_LOCATIONS early return produces exactly one warning (invalid ALLOW_WRITES skipped).
	t.Run("Missing STORAGE_LOCATIONS early return skips ALLOW_WRITES", func(t *testing.T) {
		sql := "CREATE EXTERNAL VOLUME my_vol ALLOW_WRITES = MAYBE"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning (STORAGE_LOCATIONS only, ALLOW_WRITES skipped), got %d: %v", len(warns), warns)
		}
	})

	// Multi-location where both S3 locations are missing ARN produces exactly two warnings.
	t.Run("Multi-location both missing ARN emits exactly two warnings", func(t *testing.T) {
		sql := "CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 'a' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b1/' ) ( NAME = 'b' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b2/' ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 2 {
			t.Errorf("Expected exactly 2 warnings (one per location), got %d: %v", len(warns), warns)
		}
	})

	// Multi-location where first location triggers continue (missing provider) and
	// second location is still independently validated.
	t.Run("Continue on missing provider does not skip subsequent locations", func(t *testing.T) {
		sql := "CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 'a' STORAGE_BASE_URL = 's3://b/' ) ( NAME = 'b' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b2/' ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		// Expect: "requires STORAGE_PROVIDER" for loc 1 + "STORAGE_AWS_ROLE_ARN is required" for loc 2
		if len(warns) != 2 {
			t.Errorf("Expected exactly 2 warnings, got %d: %v", len(warns), warns)
		}
	})

	// Multi-location where first location triggers continue (invalid provider) and
	// second location is still independently validated.
	t.Run("Continue on invalid provider does not skip subsequent locations", func(t *testing.T) {
		sql := "CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 'a' STORAGE_PROVIDER = 'INVALID' STORAGE_BASE_URL = 'x://b/' ) ( NAME = 'b' STORAGE_PROVIDER = 'AZURE' STORAGE_BASE_URL = 'azure://acc.blob.core.windows.net/c/' ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		// Expect: "Invalid STORAGE_PROVIDER" for loc 1 + "AZURE_TENANT_ID is required" for loc 2
		if len(warns) != 2 {
			t.Errorf("Expected exactly 2 warnings, got %d: %v", len(warns), warns)
		}
	})

	// Location missing all three required attributes produces exactly three warnings.
	t.Run("Missing all three location attributes emits exactly three warnings", func(t *testing.T) {
		sql := "CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 3 {
			t.Errorf("Expected exactly 3 warnings (NAME, STORAGE_BASE_URL, STORAGE_PROVIDER), got %d: %v", len(warns), warns)
		}
	})

	// ENCRYPTION TYPE not first key — token-based parser correctly finds TYPE
	// regardless of position inside the ENCRYPTION block, so no warning is emitted.
	t.Run("ENCRYPTION TYPE not first key produces no warning", func(t *testing.T) {
		sql := "CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ENCRYPTION = (KMS_KEY_ID = 'k' TYPE = 'AWS_SSE_KMS') ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 0 {
			t.Errorf("Expected 0 warnings (TYPE found regardless of position), got %d: %v", len(warns), warns)
		}
	})

	// Duplicate NAME in separate locations — validator does not check for
	// uniqueness, so only provider-specific errors are reported.
	t.Run("Duplicate NAME across locations does not produce name conflict error", func(t *testing.T) {
		sql := "CREATE EXTERNAL VOLUME v STORAGE_LOCATIONS = (( NAME = 'dup' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ) ( NAME = 'dup' STORAGE_PROVIDER = 'GCS' STORAGE_BASE_URL = 'gcs://b/' ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 0 {
			t.Errorf("Expected 0 warnings (duplicate NAME is allowed), got %d: %v", len(warns), warns)
		}
	})

	// Account-level prefix with OR REPLACE — prefix error still reported
	// (OR REPLACE + IF NOT EXISTS conflict takes precedence, but OR REPLACE
	// alone does not suppress prefix validation).
	t.Run("OR REPLACE with account-level prefix still reports prefix error", func(t *testing.T) {
		sql := "CREATE OR REPLACE EXTERNAL VOLUME mydb.my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "account-level objects and cannot have a database or schema prefix") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected account-level prefix warning, got: %v", warns)
		}
	})

	// ALLOW_WRITES = YES reports invalid (only TRUE/FALSE accepted).
	t.Run("ALLOW_WRITES YES emits exactly one warning beyond valid volume", func(t *testing.T) {
		sql := "CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = (( NAME = 'n' STORAGE_PROVIDER = 'S3' STORAGE_BASE_URL = 's3://b/' STORAGE_AWS_ROLE_ARN = 'arn:aws:iam::1:role/r' )) ALLOW_WRITES = YES"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning (invalid ALLOW_WRITES), got %d: %v", len(warns), warns)
		}
	})

	// Completely empty location block (just whitespace between inner parens).
	t.Run("Whitespace-only location block reports all three missing attributes", func(t *testing.T) {
		sql := "CREATE EXTERNAL VOLUME my_vol STORAGE_LOCATIONS = ((   ))"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warns := getWarnings(markers)
		// NAME, STORAGE_BASE_URL, STORAGE_PROVIDER all missing → 3 warnings.
		if len(warns) != 3 {
			t.Errorf("Expected exactly 3 warnings, got %d: %v", len(warns), warns)
		}
	})
}


