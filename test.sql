-- PASS
COPY INTO my_table FROM @my_stage;

-- PASS
COPY INTO @my_stage FROM my_table;

-- PASS
COPY INTO my_table FROM @my_stage FILES = ('f1.csv', 'f2.csv') ON_ERROR = SKIP_FILE_10;

-- PASS
COPY INTO @my_stage FROM (SELECT * FROM t) OVERWRITE = TRUE SINGLE = FALSE MAX_FILE_SIZE = 1048576;

-- FAIL: Missing FROM clause
COPY INTO my_table;

-- FAIL: FILES and PATTERN are mutually exclusive
COPY INTO my_table FROM @my_stage FILES = ('f1.csv') PATTERN = '.*\.csv';

-- FAIL: Invalid ON_ERROR value
COPY INTO my_table FROM @my_stage ON_ERROR = INVALID_VAL;

-- FAIL: PURGE must be TRUE or FALSE
COPY INTO my_table FROM @my_stage PURGE = YES;

-- FAIL: MAX_FILE_SIZE must be a positive integer
COPY INTO @my_stage FROM t MAX_FILE_SIZE = -100;

-- FAIL: Invalid FILE_FORMAT TYPE
COPY INTO my_table FROM @my_stage FILE_FORMAT = (TYPE = 'EXCEL');

-- FAIL: Table does not exist
SELECT
    *
FROM "LINEAGE_SOURCE_DB"."RAW_DATA".this_table_does_not_exists;

-- FAIL: Table does not exist
SELECT
    *
FROM "LINEAGE_SOURCE_DB"."RAW_DATA"."this_table_does_not_exists";

-- PASS
CREATE EXTERNAL TABLE et (col1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV);

-- PASS
CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (FORMAT_NAME = my_format);

-- PASS
CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = JSON) AUTO_REFRESH = TRUE;

-- PASS
CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) PARTITION BY (c1) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = PARQUET);

-- PASS
CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV) TABLE_FORMAT = DELTA;

-- FAIL: WITH LOCATION is mandatory
CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) FILE_FORMAT = (TYPE = CSV);

-- FAIL: FILE_FORMAT is mandatory
CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/;

-- FAIL: AUTO_REFRESH must be TRUE or FALSE
CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV) AUTO_REFRESH = YES;

-- FAIL: OR REPLACE is not supported
CREATE OR REPLACE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV);

-- FAIL: CLUSTER BY is not supported
CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) CLUSTER BY (c1) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV);

-- FAIL: DATA_RETENTION_TIME_IN_DAYS is not supported
CREATE EXTERNAL TABLE et (c1 int as (value:c1::int)) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV) DATA_RETENTION_TIME_IN_DAYS = 1;

-- FAIL: Column definitions must use AS <expr>
CREATE EXTERNAL TABLE et (c1 int) WITH LOCATION = @s1/path/ FILE_FORMAT = (TYPE = CSV);
