package sqlgrammar

import "testing"

func TestParseCreateDatabase_Valid(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateDatabase,
		`CREATE DATABASE my_db`,
		`CREATE OR REPLACE DATABASE my_db`,
		`CREATE OR ALTER DATABASE my_db`,
		`CREATE OR REPLACE TRANSIENT DATABASE IF NOT EXISTS my_db`,
		`CREATE TRANSIENT DATABASE my_db`,
		`CREATE DATABASE my_db COMMENT = 'hello world'`,
		`CREATE DATABASE my_db DATA_RETENTION_TIME_IN_DAYS = 7 MAX_DATA_EXTENSION_TIME_IN_DAYS = 14`,
		`CREATE DATABASE my_db EXTERNAL_VOLUME = vol CATALOG = cat ICEBERG_VERSION_DEFAULT = 2`,
		`CREATE DATABASE my_db REPLACE_INVALID_CHARACTERS = TRUE ENABLE_DATA_COMPACTION = FALSE`,
		`CREATE DATABASE my_db STORAGE_SERIALIZATION_POLICY = OPTIMIZED`,
		`CREATE DATABASE my_db CATALOG_SYNC_NAMESPACE_MODE = FLATTEN`,
		`CREATE DATABASE my_db WITH TAG (cost_center = 'sales', team = 'data')`,
		`CREATE DATABASE my_db TAG (cost_center = 'sales')`,
		`CREATE DATABASE my_db WITH CONTACT (support = my_contact, owner = other_contact)`,
		`CREATE DATABASE my_db CLONE source_db`,
		`CREATE DATABASE my_db CLONE source_db AT (TIMESTAMP => '2024-01-01 00:00:00')`,
		`CREATE DATABASE my_db CLONE source_db BEFORE (STATEMENT => '8e5d0ca9')`,
		`CREATE DATABASE my_db CLONE source_db AT (OFFSET => -60)`,
		`CREATE DATABASE my_db CLONE source_db IGNORE TABLES WITH INSUFFICIENT DATA RETENTION IGNORE HYBRID TABLES`,
		`CREATE DATABASE my_db CLONE source_db COMMENT = 'cloned'`,
		`CREATE DATABASE my_db FROM LISTING 'GLOBAL.listing_name'`,
		`CREATE DATABASE my_db FROM SHARE provider_acct.share_name`,
		`CREATE DATABASE my_db FROM BACKUP SET my_set IDENTIFIER 'abc-123'`,
		`CREATE DATABASE my_db AS REPLICA OF org_acct.primary_db`,
		`CREATE DATABASE my_db AS REPLICA OF org_acct.primary_db DATA_RETENTION_TIME_IN_DAYS = 3`,
		`CREATE DATABASE db1.schema_qualified`,
	)
}

func TestParseCreateDatabase_Invalid(t *testing.T) {
	assertInvalid(t, (*Validator).ParseCreateDatabase,
		`CREATE DATABASE`,                                     // missing name
		`CREATE my_db`,                                        // missing DATABASE keyword
		`CREATE OR DATABASE my_db`,                            // OR without REPLACE/ALTER
		`CREATE DATABASE my_db DATA_RETENTION_TIME_IN_DAYS =`, // option missing value
		`CREATE DATABASE my_db COMMENT 'x'`,                   // option missing '='
		`CREATE DATABASE my_db FROM`,                          // FROM without source
		`CREATE DATABASE my_db AS REPLICA my_src`,             // REPLICA missing OF
		`CREATE DATABASE my_db CLONE`,                         // CLONE missing source
		`CREATE DATABASE my_db garbage_trailing`,              // trailing junk
		`CREATE DATABASE my_db WITH TAG (k = 'v'`,             // unclosed paren
	)
}
