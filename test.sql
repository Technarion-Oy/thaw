CREATE OR REPLACE TEMPORARY STAGE thaw_test_stage;
PUT 'file:///tmp/test.csv' @thaw_test_stage AUTO_COMPRESS=FALSE OVERWRITE=TRUE;
SELECT * FROM @thaw_test_stage (FILE_FORMAT => (TYPE=CSV, PARSE_HEADER=TRUE));
