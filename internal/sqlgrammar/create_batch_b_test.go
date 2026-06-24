package sqlgrammar

import "testing"

func TestParseCreateObjClone(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateObjClone,
		`CREATE DATABASE D1 CLONE D0`,
		`CREATE OR REPLACE SCHEMA S1 CLONE S0`,
		`CREATE TABLE IF NOT EXISTS T1 CLONE T0 AT (TIMESTAMP => '2021-01-01')`,
		`CREATE DYNAMIC TABLE DT1 CLONE DT0 TARGET_LAG = '1 hour' WAREHOUSE = WH`,
		`CREATE DATABASE ROLE R1 CLONE R0`,
		`CREATE DATABASE D1 CLONE D0 IGNORE HYBRID TABLES INCLUDE INTERNAL STAGES`,
	)
	assertInvalid(t, (*Validator).ParseCreateObjClone,
		`CREATE DATABASE D1`,       // missing CLONE
		`CREATE DATABASE CLONE D0`, // missing target name
		`CREATE D1 CLONE D0`,       // missing object kind
		`DATABASE D1 CLONE D0`,     // missing CREATE
	)
}

func TestParseCreateComputePool(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateComputePool,
		`CREATE COMPUTE POOL CP MIN_NODES = 1 MAX_NODES = 2 INSTANCE_FAMILY = CPU_X64_XS`,
		`CREATE COMPUTE POOL IF NOT EXISTS CP MIN_NODES = 1 MAX_NODES = 5 INSTANCE_FAMILY = STANDARD_1 AUTO_RESUME = TRUE COMMENT = 'pool'`,
		`CREATE COMPUTE POOL CP FOR APPLICATION APP MIN_NODES = 1 MAX_NODES = 1 INSTANCE_FAMILY = X`,
	)
	assertInvalid(t, (*Validator).ParseCreateComputePool,
		`CREATE COMPUTE POOL`,             // missing name
		`CREATE COMPUTE CP MIN_NODES = 1`, // missing POOL
		`COMPUTE POOL CP MIN_NODES = 1`,   // missing CREATE
	)
}

func TestParseCreateConnection(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateConnection,
		`CREATE CONNECTION C1`,
		`CREATE CONNECTION IF NOT EXISTS C1 COMMENT = 'conn'`,
		`CREATE CONNECTION C1 AS REPLICA OF ORG.ACCT.C0 COMMENT = 'rep'`,
	)
	assertInvalid(t, (*Validator).ParseCreateConnection,
		`CREATE CONNECTION`,                  // missing name
		`CREATE C1`,                          // missing CONNECTION
		`CREATE CONNECTION C1 AS REPLICA C0`, // missing OF
	)
}

func TestParseCreateContact(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateContact,
		`CREATE CONTACT CT`,
		`CREATE OR REPLACE CONTACT IF NOT EXISTS CT URL = 'https://x' COMMENT = 'c'`,
		`CREATE CONTACT CT USERS = ('a', 'b')`,
		`CREATE CONTACT CT EMAIL_DISTRIBUTION_LIST = 'team@x.com'`,
	)
	assertInvalid(t, (*Validator).ParseCreateContact,
		`CREATE CONTACT`,            // missing name
		`CREATE CT URL = 'x'`,       // missing CONTACT
		`CREATE CONTACT CT URL 'x'`, // missing =
	)
}

func TestParseCreateCortexSearchService(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateCortexSearchService,
		`CREATE CORTEX SEARCH SERVICE CSS ON BODY ATTRIBUTES C1, C2 WAREHOUSE = WH TARGET_LAG = '1 hour' AS SELECT * FROM T`,
		`CREATE OR REPLACE CORTEX SEARCH SERVICE CSS ON BODY PRIMARY KEY (ID) ATTRIBUTES C1 WAREHOUSE = WH TARGET_LAG = '1 minute' COMMENT = 'c' AS SELECT 1`,
		`CREATE CORTEX SEARCH SERVICE CSS TEXT INDEXES T1, T2 VECTOR INDEXES V1 ATTRIBUTES A1 WAREHOUSE = WH TARGET_LAG = '5 minutes' AS SELECT 1`,
	)
	assertInvalid(t, (*Validator).ParseCreateCortexSearchService,
		`CREATE CORTEX SEARCH SERVICE CSS`,                                                                // missing body
		`CREATE CORTEX SEARCH SERVICE CSS ON BODY WAREHOUSE = WH`,                                         // missing ATTRIBUTES + AS
		`CREATE CORTEX SERVICE CSS ON BODY ATTRIBUTES A WAREHOUSE = WH TARGET_LAG = '1 hour' AS SELECT 1`, // missing SEARCH
	)
}

func TestParseCreateDataMetricFunction(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateDataMetricFunction,
		`CREATE DATA METRIC FUNCTION DMF (ARG TABLE(C NUMBER)) RETURNS NUMBER AS 'SELECT 1'`,
		`CREATE OR REPLACE SECURE DATA METRIC FUNCTION IF NOT EXISTS DMF (T TABLE(C VARCHAR)) RETURNS NUMBER NOT NULL LANGUAGE SQL COMMENT = 'c' AS 'COUNT(*)'`,
		`CREATE DATA METRIC FUNCTION DMF (T1 TABLE(C1 NUMBER), T2 TABLE(C2 NUMBER)) RETURNS NUMBER AS 'X'`,
	)
	assertInvalid(t, (*Validator).ParseCreateDataMetricFunction,
		`CREATE DATA METRIC FUNCTION DMF (ARG TABLE(C NUMBER)) RETURNS NUMBER`, // missing AS body
		`CREATE DATA METRIC FUNCTION DMF RETURNS NUMBER AS 'X'`,                // missing arg list
		`CREATE DATA FUNCTION DMF (A TABLE(C NUMBER)) RETURNS NUMBER AS 'X'`,   // missing METRIC
	)
}

func TestParseCreateDatabaseCatalogLinked(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateDatabaseCatalogLinked,
		`CREATE DATABASE D LINKED_CATALOG = (CATALOG = 'cat')`,
		`CREATE DATABASE D LINKED_CATALOG = (CATALOG = 'cat', ALLOWED_NAMESPACES = ('ns1', 'ns2')), EXTERNAL_VOLUME = 'ev' COMMENT = 'c'`,
		`CREATE DATABASE D LINKED_CATALOG = (CATALOG = 'cat', ALLOWED_WRITE_OPERATIONS = ALL, SYNC_INTERVAL_SECONDS = 60), CATALOG_CASE_SENSITIVITY = CASE_SENSITIVE`,
	)
	assertInvalid(t, (*Validator).ParseCreateDatabaseCatalogLinked,
		`CREATE DATABASE D`,                                // missing LINKED_CATALOG
		`CREATE DATABASE D LINKED_CATALOG = ()`,            // empty catalog params
		`CREATE DATABASE LINKED_CATALOG = (CATALOG = 'c')`, // missing name
	)
}

func TestParseCreateDatabaseRole(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateDatabaseRole,
		`CREATE DATABASE ROLE R`,
		`CREATE OR REPLACE DATABASE ROLE IF NOT EXISTS R COMMENT = 'r'`,
		`CREATE OR ALTER DATABASE ROLE R COMMENT = 'r'`,
		`CREATE DATABASE ROLE R CLONE R0`,
	)
	assertInvalid(t, (*Validator).ParseCreateDatabaseRole,
		`CREATE DATABASE ROLE`, // missing name
		`CREATE DATABASE R`,    // missing ROLE
		`CREATE ROLE R`,        // missing DATABASE
	)
}

func TestParseCreateDataset(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateDataset,
		`CREATE DATASET DS`,
		`CREATE OR REPLACE DATASET DS`,
		`CREATE OR REPLACE IF NOT EXISTS DATASET DS`,
	)
	assertInvalid(t, (*Validator).ParseCreateDataset,
		`CREATE DATASET`, // missing name
		`CREATE DS`,      // missing DATASET
		`DATASET DS`,     // missing CREATE
	)
}

func TestParseCreateDbtProject(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateDbtProject,
		`CREATE DBT PROJECT DP`,
		`CREATE OR REPLACE DBT PROJECT IF NOT EXISTS DP FROM '@stage' COMMENT = 'c' DBT_VERSION = '1.5'`,
		`CREATE DBT PROJECT DP DEFAULT_TARGET = DEV EXTERNAL_ACCESS_INTEGRATIONS = (EAI1, EAI2)`,
	)
	assertInvalid(t, (*Validator).ParseCreateDbtProject,
		`CREATE DBT PROJECT`, // missing name
		`CREATE DBT DP`,      // missing PROJECT
		`CREATE PROJECT DP`,  // missing DBT
	)
}

func TestParseCreateDcmProject(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateDcmProject,
		`CREATE DCM PROJECT DP`,
		`CREATE OR REPLACE DCM PROJECT IF NOT EXISTS DP LOG_LEVEL = DEBUG COMMENT = 'c'`,
		`CREATE DCM PROJECT DP LOG_LEVEL = ERROR`,
	)
	assertInvalid(t, (*Validator).ParseCreateDcmProject,
		`CREATE DCM PROJECT`, // missing name
		`CREATE DCM DP`,      // missing PROJECT
		`CREATE PROJECT DP`,  // missing DCM
	)
}

func TestParseCreateDynamicTable(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateDynamicTable,
		`CREATE DYNAMIC TABLE DT TARGET_LAG = '1 hour' WAREHOUSE = WH AS SELECT * FROM T`,
		`CREATE OR REPLACE TRANSIENT DYNAMIC TABLE IF NOT EXISTS DT (C1 NUMBER, C2 VARCHAR) TARGET_LAG = DOWNSTREAM WAREHOUSE = WH REFRESH_MODE = FULL CLUSTER BY (C1) AS SELECT 1`,
		`CREATE DYNAMIC TABLE DT WAREHOUSE = WH TARGET_LAG = '5 minutes' REFRESH USING (DELETE FROM X)`,
	)
	assertInvalid(t, (*Validator).ParseCreateDynamicTable,
		`CREATE DYNAMIC TABLE DT WAREHOUSE = WH TARGET_LAG = '1 hour'`, // missing AS / REFRESH
		`CREATE DYNAMIC DT WAREHOUSE = WH AS SELECT 1`,                 // missing TABLE
		`CREATE TABLE DT WAREHOUSE = WH AS SELECT 1`,                   // missing DYNAMIC
	)
}

func TestParseCreateEventTable(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateEventTable,
		`CREATE EVENT TABLE ET`,
		`CREATE OR REPLACE EVENT TABLE IF NOT EXISTS ET CHANGE_TRACKING = TRUE COMMENT = 'c' CLUSTER BY (C1)`,
		`CREATE EVENT TABLE ET CLONE ET0 AT (OFFSET => -60) COPY GRANTS`,
		`CREATE EVENT TABLE ET WITH ROW ACCESS POLICY RAP ON (C1)`,
	)
	assertInvalid(t, (*Validator).ParseCreateEventTable,
		`CREATE EVENT TABLE`, // missing name
		`CREATE EVENT ET`,    // missing TABLE
		`CREATE TABLE ET`,    // missing EVENT
	)
}

func TestParseCreateExperiment(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateExperiment,
		`CREATE EXPERIMENT EX`,
		`CREATE OR REPLACE EXPERIMENT EX`,
		`CREATE EXPERIMENT IF NOT EXISTS EX`,
	)
	assertInvalid(t, (*Validator).ParseCreateExperiment,
		`CREATE EXPERIMENT`, // missing name
		`CREATE EX`,         // missing EXPERIMENT
		`EXPERIMENT EX`,     // missing CREATE
	)
}

func TestParseCreateExternalAgent(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateExternalAgent,
		`CREATE EXTERNAL AGENT EA`,
		`CREATE OR REPLACE EXTERNAL AGENT IF NOT EXISTS EA WITH VERSION V1 COMMENT = 'c'`,
		`CREATE EXTERNAL AGENT EA COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseCreateExternalAgent,
		`CREATE EXTERNAL AGENT`, // missing name
		`CREATE EXTERNAL EA`,    // missing AGENT
		`CREATE AGENT EA`,       // missing EXTERNAL
	)
}

func TestParseCreateExternalAccessIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateExternalAccessIntegration,
		`CREATE EXTERNAL ACCESS INTEGRATION EAI ALLOWED_NETWORK_RULES = (NR1) ENABLED = TRUE`,
		`CREATE OR REPLACE EXTERNAL ACCESS INTEGRATION EAI ALLOWED_NETWORK_RULES = (NR1, NR2) ALLOWED_AUTHENTICATION_SECRETS = ALL ENABLED = FALSE COMMENT = 'c'`,
		`CREATE EXTERNAL ACCESS INTEGRATION EAI ENABLED = TRUE ALLOWED_NETWORK_RULES = (NR1) ALLOWED_API_AUTHENTICATION_INTEGRATIONS = NONE`,
	)
	assertInvalid(t, (*Validator).ParseCreateExternalAccessIntegration,
		`CREATE EXTERNAL ACCESS INTEGRATION`,                  // missing name
		`CREATE EXTERNAL INTEGRATION EAI ENABLED = TRUE`,      // missing ACCESS
		`CREATE EXTERNAL ACCESS INTEGRATION EAI ENABLED TRUE`, // missing =
	)
}
