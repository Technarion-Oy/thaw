-- FAIL: Table does not exist
SELECT
    *
FROM "LINEAGE_SOURCE_DB"."RAW_DATA".this_table_does_not_exists;

-- FAIL: Table does not exist
SELECT
    *
FROM "LINEAGE_SOURCE_DB"."RAW_DATA"."this_table_does_not_exists";