package sqleditor

import "testing"

// cursorCol returns the 1-based column of the first `*` in a single-line SQL.
func starCol(sql string) int {
	for i := 0; i < len(sql); i++ {
		if sql[i] == '*' {
			return i + 1 // 1-based
		}
	}
	return -1
}

func TestStarSelectAt(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		wantHit bool
		alias   string
	}{
		{"bare star", "SELECT * FROM t", true, ""},
		{"distinct star", "SELECT DISTINCT * FROM t", true, ""},
		{"all star", "SELECT ALL * FROM t", true, ""},
		{"star after comma", "SELECT a, * FROM t", true, ""},
		{"alias star", "SELECT t.* FROM tbl t", true, "t"},
		{"quoted alias star", `SELECT "my table".* FROM x "my table"`, true, `"my table"`},
		{"count star skipped", "SELECT COUNT(*) FROM t", false, ""},
		{"multiplication skipped", "SELECT a * b FROM t", false, ""},
		{"number multiplication skipped", "SELECT 2 * n FROM t", false, ""},
		{"keyword operand multiplication skipped", "SELECT CASE WHEN a THEN 1 ELSE 0 END * 100 FROM t", false, ""},
		{"star inside quoted table name", `SELECT "ID" FROM "DB"."PUBLIC"."Testin*table"`, false, ""},
		{"multi-part qualifier keeps last segment as alias", "SELECT mydb.myschema.mytbl.* FROM mydb.myschema.mytbl", true, "mytbl"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			col := starCol(c.sql)
			got := StarSelectAt(c.sql, 1, col)
			if c.wantHit != (got != nil) {
				t.Fatalf("hit=%v, want %v (sql=%q col=%d got=%+v)", got != nil, c.wantHit, c.sql, col, got)
			}
			if got != nil && got.Alias != c.alias {
				t.Fatalf("alias=%q, want %q", got.Alias, c.alias)
			}
		})
	}
}

// A multi-part qualifier's replace range must start at the first segment so the
// whole `db.schema.tbl.*` is overwritten (not just the trailing `tbl.*`).
func TestStarSelectAt_MultiPartRange(t *testing.T) {
	sql := "SELECT mydb.myschema.mytbl.* FROM mydb.myschema.mytbl"
	got := StarSelectAt(sql, 1, starCol(sql))
	if got == nil {
		t.Fatal("expected a hit")
	}
	if got.StartCol != 8 { // `mydb` starts right after "SELECT "
		t.Fatalf("StartCol=%d, want 8 (start of `mydb`)", got.StartCol)
	}
}

// sqltok reports byte columns; the returned/compared columns must be Monaco
// UTF-16 columns, so a non-ASCII char earlier on the line can't shift them.
func TestStarSelectAt_NonASCII(t *testing.T) {
	sql := "SELECT 'café', * FROM t"
	// Monaco column of the `*` is 16 (é is one UTF-16 unit); its byte column is 17.
	// The old byte-based code missed the star at Monaco col 16 entirely.
	got := StarSelectAt(sql, 1, 16)
	if got == nil {
		t.Fatalf("expected a hit at UTF-16 col 16, got nil")
	}
	if got.StartCol != 16 || got.EndCol != 17 {
		t.Fatalf("range = [%d,%d), want [16,17) in UTF-16 columns", got.StartCol, got.EndCol)
	}
	if StarSelectAt(sql, 1, 18) != nil { // one past the star's right edge
		t.Fatal("UTF-16 col 18 is past the star, want nil")
	}
}

func TestFromSourceCount(t *testing.T) {
	cases := []struct {
		sql  string
		want int
	}{
		{"SELECT * FROM t", 1},
		{"SELECT * FROM a JOIN b ON a.id=b.id", 2},
		{"SELECT * FROM a LEFT JOIN b ON a.id=b.id JOIN c ON b.k=c.k", 3},
		{"SELECT * FROM t1, t2", 2},
		{"SELECT * FROM t WHERE x = 1", 1},
		{"SELECT * FROM a JOIN b ON a.id = fn(b.x)", 2},               // fn() in ON is a predicate, not a source
		{"SELECT * FROM a JOIN b USING (id, name)", 2},               // USING(...) is not a source paren
		{"SELECT * FROM TABLE(FLATTEN(input => x, path => 'a'))", -1}, // table function → refuse
		{"SELECT * FROM t1 JOIN sales PIVOT(SUM(amt) FOR q IN ('Q1')) p ON t1.id=p.id", -1}, // PIVOT → refuse
		{"SELECT * FROM t WHERE x IN (SELECT y FROM z)", -1},         // subquery → refuse
		{"SELECT * FROM (SELECT 1 x) sub JOIN r ON r.id=sub.x", -1},  // derived table → refuse
		{"WITH c AS (SELECT 1 x) SELECT * FROM c JOIN r ON r.id=c.x", -1}, // CTE → refuse
		{"UPDATE t SET a=1", -1},                                     // no FROM
	}
	for _, c := range cases {
		if got := FromSourceCount(c.sql); got != c.want {
			t.Errorf("FromSourceCount(%q) = %d, want %d", c.sql, got, c.want)
		}
	}
}

// The cursor lands on either edge of the star; both must resolve.
func TestStarSelectAt_BothEdges(t *testing.T) {
	sql := "SELECT * FROM t"
	col := starCol(sql) // left edge (on the star)
	if StarSelectAt(sql, 1, col) == nil {
		t.Fatal("left edge: expected hit")
	}
	if StarSelectAt(sql, 1, col+1) == nil { // right edge (after the star)
		t.Fatal("right edge: expected hit")
	}
	if StarSelectAt(sql, 1, col+2) != nil { // past the star
		t.Fatal("past star: expected no hit")
	}
}
