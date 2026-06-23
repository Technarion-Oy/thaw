package sqlgrammar

import "testing"

// CREATE TABLE column-list / body strictness (see #556 follow-up): a column must
// declare a data type, and the statement must have a real body (column list, AS
// query, LIKE, CLONE, USING TEMPLATE, or FROM ARCHIVE). The data-type *name* is
// validated by sqleditor.ValidateDataTypes, not here.
func TestParseCreateTable_Strict(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateTable,
		`CREATE TABLE t (id INT)`,
		`CREATE OR REPLACE TABLE db.s.t (id NUMBER(38,0), name VARCHAR(100) NOT NULL DEFAULT 'x')`,
		`CREATE OR ALTER TABLE t (id INT)`,
		`CREATE TABLE t (a INT, b STRING, PRIMARY KEY (a))`,
		`CREATE TABLE t (a INT, CONSTRAINT fk FOREIGN KEY (a) REFERENCES u (id))`,
		`CREATE TABLE t (id INT) CLUSTER BY (id) COMMENT = 'c'`,
		`CREATE TABLE t AS SELECT * FROM s`,
		`CREATE TABLE t (a, b) AS SELECT 1, 2`, // CTAS column-alias list (no types)
		`CREATE TABLE t LIKE src`,
		`CREATE TABLE t CLONE src`,
		`CREATE TRANSIENT TABLE t (id INT)`,
	)
	assertInvalid(t, (*Validator).ParseCreateTable,
		`CREATE TABLE t`,                 // no body
		`CREATE OR REPLACE TABLE db.s.t`, // no body
		`CREATE TABLE t (dsdfssf)`,       // column has no data type
		`CREATE TABLE t ()`,              // empty column list
		`CREATE TABLE t (a INT, b)`,      // second column has no data type
		`CREATE TABLE`,                   // no name
	)
}
