package sqlgrammar

import "testing"

// Tests for the data-loading / file-staging and CALL / EXECUTE / EXPLAIN /
// COMMENT grammar rules (data_loading.go + execute.go), issue #556.

func TestParseCopyFiles(t *testing.T) {
	assertValid(t, (*Validator).ParseCopyFiles,
		`COPY FILES INTO @mydb.public.dest/data/ FROM @mydb.public.src/data/`,
		`COPY FILES INTO @dest FROM @src FILES = ('a.csv', 'b.csv') PATTERN = '.*[.]csv'`,
		`COPY FILES INTO @dest FROM (SELECT url FROM @src) DETAILED_OUTPUT = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseCopyFiles,
		``,
		`COPY FILES INTO @dest`,                // missing FROM
		`SELECT * FROM @src`,                   // wrong leading keyword
		`COPY FILES FROM @src INTO @dest`,      // INTO/FROM out of order
	)
}

func TestParseCopyIntoLocation(t *testing.T) {
	assertValid(t, (*Validator).ParseCopyIntoLocation,
		`COPY INTO @mystage/unload/ FROM mytable`,
		`COPY INTO 's3://bucket/path/' FROM (SELECT * FROM t) FILE_FORMAT = (TYPE = CSV) SINGLE = TRUE`,
		`COPY INTO @stage FROM mytable PARTITION BY (col1) MAX_FILE_SIZE = 1000000 OVERWRITE = FALSE`,
	)
	assertInvalid(t, (*Validator).ParseCopyIntoLocation,
		``,
		`COPY INTO @stage`,            // missing FROM <source>
		`DROP INTO @stage FROM t`,     // wrong leading keyword
	)
}

func TestParseCopyIntoTable(t *testing.T) {
	assertValid(t, (*Validator).ParseCopyIntoTable,
		`COPY INTO mydb.public.mytable FROM @mystage/data/`,
		`COPY INTO mytable FROM @stage FILES = ('a.csv') PATTERN = '.*' FILE_FORMAT = (TYPE = CSV)`,
		`COPY INTO mytable (c1, c2) FROM (SELECT $1, $2 FROM @stage) ON_ERROR = CONTINUE`,
	)
	assertInvalid(t, (*Validator).ParseCopyIntoTable,
		``,
		`COPY INTO mytable`,         // missing FROM
		`SELECT INTO mytable FROM @s`,
	)
}

func TestParseGet(t *testing.T) {
	assertValid(t, (*Validator).ParseGet,
		`GET @mystage/path/ file:///tmp/data/`,
		`GET @~/path file:///tmp/ PARALLEL = 4`,
		`GET @mydb.public.stage/sub file:///tmp/ PATTERN = '.*[.]gz'`,
	)
	assertInvalid(t, (*Validator).ParseGet,
		``,
		`GET file:///tmp/ @stage`,   // stage/file order wrong
		`PUT @stage file:///tmp/`,   // wrong leading keyword
	)
}

func TestParseList(t *testing.T) {
	assertValid(t, (*Validator).ParseList,
		`LIST @mystage`,
		`LIST @mydb.public.stage/path/ PATTERN = '.*[.]csv'`,
		`LS @~/path`,
	)
	assertInvalid(t, (*Validator).ParseList,
		``,
		`LIST mystage`,        // no @ stage ref
		`SHOW @mystage`,       // wrong leading keyword
	)
}

func TestParsePut(t *testing.T) {
	assertValid(t, (*Validator).ParsePut,
		`PUT file:///tmp/data.csv @mystage`,
		`PUT file:///tmp/data.csv @mystage AUTO_COMPRESS = TRUE OVERWRITE = FALSE`,
		`PUT file:///tmp/data.csv @mystage PARALLEL = 8 SOURCE_COMPRESSION = GZIP`,
	)
	assertInvalid(t, (*Validator).ParsePut,
		``,
		`PUT @mystage file:///tmp/data.csv`, // order wrong
		`GET file:///tmp/ @mystage`,         // wrong leading keyword
	)
}

func TestParseRemove(t *testing.T) {
	assertValid(t, (*Validator).ParseRemove,
		`REMOVE @mystage`,
		`REMOVE @mydb.public.stage/path/ PATTERN = '.*'`,
		`RM @~/path`,
	)
	assertInvalid(t, (*Validator).ParseRemove,
		``,
		`REMOVE mystage`,    // no @ stage ref
		`LIST @mystage X`,   // wrong leading keyword + trailing
	)
}

func TestParseCall(t *testing.T) {
	assertValid(t, (*Validator).ParseCall,
		`CALL myproc()`,
		`CALL mydb.public.myproc(1, 'x', col => 2)`,
		`CALL myproc(1) INTO :result`,
	)
	assertInvalid(t, (*Validator).ParseCall,
		``,
		`CALL myproc`,         // missing arg list parens
		`SELECT myproc()`,     // wrong leading keyword
	)
}

func TestParseCallWithAnonymousProcedure(t *testing.T) {
	assertValid(t, (*Validator).ParseCallWithAnonymousProcedure,
		`WITH p AS PROCEDURE () RETURNS INT LANGUAGE JAVA CALL p()`,
		`WITH p AS PROCEDURE (x INT) RETURNS TABLE (a INT) AS 'body' CALL p(1)`,
		`WITH p AS PROCEDURE () RETURNS INT AS 'b' CALL p() INTO :r`,
	)
	assertInvalid(t, (*Validator).ParseCallWithAnonymousProcedure,
		``,
		`WITH p AS PROCEDURE () RETURNS INT AS 'b'`, // no trailing CALL
		`SELECT p AS PROCEDURE () CALL p()`,         // wrong leading keyword
	)
}

func TestParseComment(t *testing.T) {
	assertValid(t, (*Validator).ParseComment,
		`COMMENT ON TABLE mytable IS 'a table'`,
		`COMMENT IF EXISTS ON VIEW myview IS 'a view'`,
		`COMMENT ON COLUMN mytable.mycol IS 'a column'`,
	)
	assertInvalid(t, (*Validator).ParseComment,
		``,
		`COMMENT ON TABLE mytable`,        // missing IS '...'
		`ALTER ON TABLE mytable IS 'x'`,   // wrong leading keyword
	)
}

func TestParseExecuteAlert(t *testing.T) {
	assertValid(t, (*Validator).ParseExecuteAlert,
		`EXECUTE ALERT myalert`,
		`EXECUTE ALERT mydb.public.myalert`,
		`EXECUTE ALERT "MyAlert"`,
	)
	assertInvalid(t, (*Validator).ParseExecuteAlert,
		``,
		`EXECUTE ALERT`,        // missing name
		`EXECUTE TASK myalert`, // wrong object word
	)
}

func TestParseExecuteDbtProject(t *testing.T) {
	assertValid(t, (*Validator).ParseExecuteDbtProject,
		`EXECUTE DBT PROJECT myproject`,
		`EXECUTE DBT PROJECT IF EXISTS myproject ARGS = 'run' DBT_VERSION = '1.5'`,
		`EXECUTE DBT PROJECT FROM WORKSPACE myws ARGS = 'test' PROJECT_ROOT = 'sub'`,
	)
	assertInvalid(t, (*Validator).ParseExecuteDbtProject,
		``,
		`EXECUTE DBT myproject`,            // missing PROJECT
		`EXECUTE PIPELINE PROJECT myproj`,  // wrong object word
	)
}

func TestParseExecuteDcmProject(t *testing.T) {
	assertValid(t, (*Validator).ParseExecuteDcmProject,
		`EXECUTE DCM PROJECT myproj PLAN FROM '@stage/files'`,
		`EXECUTE DCM PROJECT myproj DEPLOY AS "dep1" FROM 'src'`,
		`EXECUTE DCM PROJECT myproj REFRESH ALL`,
	)
	assertInvalid(t, (*Validator).ParseExecuteDcmProject,
		``,
		`EXECUTE DCM PROJECT myproj`,         // missing action
		`EXECUTE DCM myproj PLAN FROM 'src'`, // missing PROJECT
	)
}

func TestParseExecuteImmediate(t *testing.T) {
	assertValid(t, (*Validator).ParseExecuteImmediate,
		`EXECUTE IMMEDIATE 'SELECT 1'`,
		`EXECUTE IMMEDIATE $stmt`,
		`EXECUTE IMMEDIATE 'SELECT ?' USING (a, b)`,
	)
	assertInvalid(t, (*Validator).ParseExecuteImmediate,
		``,
		`EXECUTE IMMEDIATE`,            // missing body
		`EXECUTE 'SELECT 1'`,          // missing IMMEDIATE
	)
}

func TestParseExecuteImmediateFrom(t *testing.T) {
	assertValid(t, (*Validator).ParseExecuteImmediateFrom,
		`EXECUTE IMMEDIATE FROM @mystage/script.sql`,
		`EXECUTE IMMEDIATE FROM './script.sql'`,
		`EXECUTE IMMEDIATE FROM @stage/s.sql USING (x => 1) DRY_RUN = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseExecuteImmediateFrom,
		``,
		`EXECUTE IMMEDIATE FROM`,        // missing path
		`EXECUTE IMMEDIATE @stage/x`,    // missing FROM
	)
}

func TestParseExecuteJobService(t *testing.T) {
	assertValid(t, (*Validator).ParseExecuteJobService,
		`EXECUTE JOB SERVICE IN COMPUTE POOL mypool FROM SPECIFICATION 'spec'`,
		`EXECUTE JOB SERVICE IN COMPUTE POOL mypool FROM @stage SPECIFICATION_FILE = 'job.yaml'`,
		`EXECUTE JOB SERVICE IN COMPUTE POOL mypool FROM SPECIFICATION_TEMPLATE 'tpl' USING (x => 1)`,
	)
	assertInvalid(t, (*Validator).ParseExecuteJobService,
		``,
		`EXECUTE JOB SERVICE IN COMPUTE POOL mypool`, // missing FROM ...
		`EXECUTE JOB SERVICE mypool FROM SPECIFICATION 'spec'`,
	)
}

func TestParseExecuteNotebook(t *testing.T) {
	assertValid(t, (*Validator).ParseExecuteNotebook,
		`EXECUTE NOTEBOOK mynb()`,
		`EXECUTE NOTEBOOK mydb.public.mynb('p1', 'p2')`,
		`EXECUTE NOTEBOOK mynb('param')`,
	)
	assertInvalid(t, (*Validator).ParseExecuteNotebook,
		``,
		`EXECUTE NOTEBOOK mynb`,    // missing arg-list parens
		`EXECUTE TASK mynb()`,     // wrong object word
	)
}

func TestParseExecuteNotebookProject(t *testing.T) {
	assertValid(t, (*Validator).ParseExecuteNotebookProject,
		`EXECUTE NOTEBOOK PROJECT db.sch.proj MAIN_FILE = 'notebook.ipynb' COMPUTE_POOL = 'pool' QUERY_WAREHOUSE = 'wh' RUNTIME = '1.0'`,
		`EXECUTE NOTEBOOK PROJECT db.sch.proj MAIN_FILE = 'nb.ipynb' ARGUMENTS = 'args'`,
		`EXECUTE NOTEBOOK PROJECT myproj RUNTIME = '2.0' SECRETS = (db.sch.s1)`,
	)
	assertInvalid(t, (*Validator).ParseExecuteNotebookProject,
		``,
		`EXECUTE NOTEBOOK db.sch.proj`,      // missing PROJECT
		`EXECUTE PROJECT NOTEBOOK myproj`,   // words swapped
	)
}

func TestParseExecuteTask(t *testing.T) {
	assertValid(t, (*Validator).ParseExecuteTask,
		`EXECUTE TASK mytask`,
		`EXECUTE TASK mytask RETRY LAST`,
		`EXECUTE TASK mytask RETRY GRAPH RUN GROUP 'grp123'`,
	)
	assertInvalid(t, (*Validator).ParseExecuteTask,
		``,
		`EXECUTE TASK`,            // missing name
		`EXECUTE ALERT mytask`,    // wrong object word
	)
}

func TestParseExplain(t *testing.T) {
	assertValid(t, (*Validator).ParseExplain,
		`EXPLAIN SELECT * FROM t`,
		`EXPLAIN USING JSON SELECT 1`,
		`EXPLAIN USING TABULAR SELECT a, b FROM t WHERE a > 1`,
	)
	assertInvalid(t, (*Validator).ParseExplain,
		``,
		`EXPLAIN`,                  // missing inner statement
		`EXPLAIN USING JSON`,       // USING form still needs a statement
	)
}
