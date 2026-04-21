-- FAIL
-- WARNING — Unexpected property 'ENABLE' in statement.
-- Should not complain
CREATE STAGE RAND_DB_EAE9EE1B29D941C28FCDF788722D7747.PUBLIC.GOOD_S3_STAGE
    URL = 's3://my-company-bucket/data/'
    STORAGE_INTEGRATION = s3_int
    DIRECTORY = (ENABLE = TRUE);

-- ==============================================================================
-- ✅ VALID STATEMENTS (Expected to pass validation)
-- ==============================================================================

-- FAIL
-- WARNING — Unexpected property 'WAREHOUSE_TYPE' in statement.
-- Should not complain that
CREATE OR REPLACE WAREHOUSE my_analytics_wh
    WITH WAREHOUSE_SIZE = 'X-LARGE'
    WAREHOUSE_TYPE = 'STANDARD'
    AUTO_SUSPEND = 300
    AUTO_RESUME = TRUE
    MIN_CLUSTER_COUNT = 1
    MAX_CLUSTER_COUNT = 3
    SCALING_POLICY = 'ECONOMY';