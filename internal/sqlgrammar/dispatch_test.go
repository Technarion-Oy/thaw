package sqlgrammar

import (
	"reflect"
	"strings"
	"testing"
)

func topLevelOK(sql string) bool {
	v := New(sql)
	return v.Recognized() && v.ParseTopLevel()
}

// TestDispatch_NoGenericIndexRules is a tripwire for the hand-maintained
// dispatchExclude denylist: a generic "CREATE/ALTER/DROP/… <object>" index-page
// rule (its method name ends in "Obj" or "Objs") must never reach a dispatch
// bucket — if one does, it would re-introduce the catch-all leniency this design
// removes (masking malformed statements). A future such rule added without being
// listed in dispatchExclude fails here, forcing a conscious decision. (Specific
// rules like ParseShowObjects — "...Objects" — are unaffected.)
func TestDispatch_NoGenericIndexRules(t *testing.T) {
	reg := rules()
	vt := reflect.TypeFor[*Validator]()
	for i := 0; i < vt.NumMethod(); i++ {
		name := vt.Method(i).Name
		if !strings.HasSuffix(name, "Obj") && !strings.HasSuffix(name, "Objs") {
			continue
		}
		fn, ok := vt.Method(i).Func.Interface().(ruleFn)
		if !ok {
			continue
		}
		for kw, bucket := range reg {
			for _, r := range bucket {
				if reflect.ValueOf(r).Pointer() == reflect.ValueOf(fn).Pointer() {
					t.Errorf("generic index rule %s leaked into dispatch bucket %q — add it to dispatchExclude", name, kw)
				}
			}
		}
	}
}

func TestParseTopLevel_Valid(t *testing.T) {
	cases := []string{
		`CREATE DATABASE my_db`,
		`CREATE OR REPLACE TABLE t (a INT, b STRING)`,
		`CREATE DATABASE ROLE my_db.r`, // disambiguates from CREATE DATABASE
		`ALTER TABLE t RENAME TO t2`,
		`DROP TABLE IF EXISTS t CASCADE`,
		`UNDROP DATABASE d`,
		`SHOW TABLES IN SCHEMA db.s LIKE 'foo%'`,
		`DESCRIBE TABLE t`,
		`DESC USER u`,
		`GRANT ROLE r1 TO ROLE r2`,
		`REVOKE ROLE r1 FROM USER u`,
		`SELECT * FROM t WHERE x = 1`,
		`INSERT INTO t VALUES (1, 2)`,
		`UPDATE t SET a = 1 WHERE id = 2`,
		`DELETE FROM t WHERE id = 1`,
		`MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET t.a = s.a`,
		`TRUNCATE TABLE t`,
		`USE WAREHOUSE wh`,
		`USE ROLE sysadmin`,
		`SET v = 1`,
		`BEGIN`,
		`COMMIT`,
		`ROLLBACK`,
		`COMMENT ON TABLE t IS 'hi'`,
		`EXPLAIN SELECT 1`,
		`WITH cte AS (SELECT 1) SELECT * FROM cte`,
		`CREATE DATABASE my_db;`, // trailing semicolon tolerated
	}
	for _, sql := range cases {
		if !topLevelOK(sql) {
			v := New(sql)
			v.ParseTopLevel()
			t.Errorf("expected top-level VALID for %q: recognized=%v failure=%s",
				sql, New(sql).Recognized(), v.Failure().Message())
		}
	}
}

func TestParseTopLevel_Malformed(t *testing.T) {
	// Recognized leading keyword, but the statement is structurally broken — should
	// fail to parse (so diagnostics can flag it). These fail even the lenient
	// generic catch-all rules: a bare command word, or a required object name that
	// is simply absent.
	cases := []string{
		`CREATE`,           // bare keyword
		`CREATE DATABASE`,  // missing name
		`ALTER`,            // bare keyword
		`DROP`,             // bare keyword
		`SHOW`,             // missing object
		`GRANT ROLE r1 TO`, // missing grantee
		`INSERT INTO`,      // missing target
	}
	for _, sql := range cases {
		v := New(sql)
		if !v.Recognized() {
			t.Errorf("expected %q to be Recognized (leading keyword is modeled)", sql)
		}
		if v.ParseTopLevel() {
			t.Errorf("expected top-level FAILURE for malformed %q", sql)
		}
	}
}

// TestParseTopLevel_NoCatchAllLeniency documents that the generic
// "CREATE/ALTER/DROP/… <object>" index rules are excluded from dispatch
// (see dispatchExclude): the specific per-command rules govern, so statements
// for unknown object types or with a missing required action/body are flagged
// rather than silently accepted by a catch-all.
func TestParseTopLevel_NoCatchAllLeniency(t *testing.T) {
	flagged := []string{
		`CREATE WIDGET w FOO BAR`,  // unknown object type
		`DROP FROBNICATE x`,        // unknown object type
		`SHOW WIDGETS`,             // unknown object type
		`ALTER TABLE t`,            // missing action
		`CREATE TABLE t`,           // missing column list / AS / LIKE / CLONE
		`CREATE TABLE t (dsdfssf)`, // column without a data type
		`CREATE TABLE t ()`,        // empty column list
	}
	for _, sql := range flagged {
		if topLevelOK(sql) {
			t.Errorf("expected %q to be flagged (no catch-all), but it parsed", sql)
		}
	}
	// Well-formed statements for modeled objects are still accepted.
	valid := []string{
		`DROP DATABASE d`,
		`ALTER TABLE t RENAME TO t2`,
		`CREATE TABLE t (id INT)`,
		`CREATE OR ALTER TABLE t (id INT)`,
		`CREATE TABLE t (a, b) AS SELECT 1, 2`,
	}
	for _, sql := range valid {
		if !topLevelOK(sql) {
			v := New(sql)
			v.ParseTopLevel()
			t.Errorf("expected %q to be accepted: %s", sql, v.Failure().Message())
		}
	}
}

func TestRecognized_Unmodeled(t *testing.T) {
	// Leading keywords with no implemented grammar must not be Recognized, so the
	// grammar validator stays silent on them.
	cases := []string{
		``,
		`   `,
		`PUT file:///tmp/a @stage`, // PUT is modeled — sanity excluded below
		`FOOBAR baz qux`,
		`WHERE x = 1`, // a bare sub-clause is not a statement
	}
	wantRecognized := map[string]bool{
		`PUT file:///tmp/a @stage`: true,
	}
	for _, sql := range cases {
		got := New(sql).Recognized()
		if got != wantRecognized[sql] {
			t.Errorf("Recognized(%q) = %v, want %v", sql, got, wantRecognized[sql])
		}
	}
}
