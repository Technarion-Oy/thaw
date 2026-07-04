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
		{"star after comma", "SELECT a, * FROM t", true, ""},
		{"alias star", "SELECT t.* FROM tbl t", true, "t"},
		{"quoted alias star", `SELECT "my table".* FROM x "my table"`, true, `"my table"`},
		{"count star skipped", "SELECT COUNT(*) FROM t", false, ""},
		{"multiplication skipped", "SELECT a * b FROM t", false, ""},
		{"number multiplication skipped", "SELECT 2 * n FROM t", false, ""},
		{"star inside quoted table name", `SELECT "ID" FROM "DB"."PUBLIC"."Testin*table"`, false, ""},
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
