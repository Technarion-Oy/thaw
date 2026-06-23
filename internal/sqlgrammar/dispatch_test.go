package sqlgrammar

import "testing"

func topLevelOK(sql string) bool {
	v := New(sql)
	return v.Recognized() && v.ParseTopLevel()
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
			t.Errorf("expected %q to be Recognized (leading keyword is modelled)", sql)
		}
		if v.ParseTopLevel() {
			t.Errorf("expected top-level FAILURE for malformed %q", sql)
		}
	}
}

// TestParseTopLevel_CatchAllLeniency documents the deliberate conservatism of the
// dispatcher: the generic "CREATE/ALTER/DROP/… <object>" index rules accept any
// roughly-well-formed statement (leader + name + arbitrary tail). The grammar
// diagnostic therefore never flags valid-but-imperfectly-modelled SQL — it only
// fires on the clearly-broken cases above. If these begin to be rejected (e.g.
// after removing the catch-alls), that is a deliberate tightening, not a bug.
func TestParseTopLevel_CatchAllLeniency(t *testing.T) {
	lenient := []string{
		`ALTER TABLE t`,
		`DROP DATABASE d`,
		`CREATE WIDGET w FOO BAR`, // unknown object type, accepted by ParseCreateObj
	}
	for _, sql := range lenient {
		if !topLevelOK(sql) {
			t.Errorf("expected catch-all to ACCEPT %q (conservative design)", sql)
		}
	}
}

func TestRecognized_Unmodelled(t *testing.T) {
	// Leading keywords with no implemented grammar must not be Recognized, so the
	// grammar validator stays silent on them.
	cases := []string{
		``,
		`   `,
		`PUT file:///tmp/a @stage`, // PUT is modelled — sanity excluded below
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
