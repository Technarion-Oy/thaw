package sqlgrammar

import (
	"reflect"
	"testing"

	"thaw/internal/sqltok"
)

func collect(t *testing.T, sql string) []string {
	t.Helper()
	return CollectCTENames(sql, sqltok.SignificantTokens(sql))
}

func TestCollectCTENames(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want []string
	}{
		{"no WITH", `SELECT * FROM t`, nil},
		{"single CTE", `WITH c AS (SELECT 1) SELECT * FROM c`, []string{"c"}},
		{"comma list", `WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a JOIN b`, []string{"a", "b"}},
		{"commas inside body are not separators",
			`WITH a AS (SELECT x, y FROM t), b AS (SELECT 2) SELECT 1`, []string{"a", "b"}},
		{"WITH RECURSIVE", `WITH RECURSIVE r AS (SELECT 1 UNION ALL SELECT n+1 FROM r) SELECT * FROM r`, []string{"r"}},
		{"CTE named recursive", `WITH recursive AS (SELECT 1) SELECT * FROM recursive`, []string{"recursive"}},
		{"column list", `WITH c (a, b) AS (SELECT 1, 2) SELECT * FROM c`, []string{"c"}},
		{"quoted name kept raw", `WITH "My CTE" AS (SELECT 1) SELECT * FROM "My CTE"`, []string{`"My CTE"`}},
		{"nested WITH in body",
			`WITH outer_c AS (WITH inner_c AS (SELECT 1) SELECT * FROM inner_c) SELECT * FROM outer_c`,
			[]string{"outer_c", "inner_c"}},
		{"unterminated body while typing", `WITH a AS (SELECT 1), b AS (SELECT`, []string{"a", "b"}},
		{"SWAP WITH is not a CTE", `ALTER TABLE a SWAP WITH b`, nil},
		{"STARTS WITH is not a CTE", `SELECT * FROM t START WITH x = 1 CONNECT BY PRIOR id = pid`, nil},
		{"WITH TAG is not a CTE", `CREATE VIEW v WITH TAG (env = 'prod') AS (SELECT 1)`, nil},
		{"WITH ROW ACCESS POLICY is not a CTE",
			`CREATE VIEW v WITH ROW ACCESS POLICY p ON (c) AS SELECT * FROM t`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := collect(t, tc.sql)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("CollectCTENames(%q) = %v, want %v", tc.sql, got, tc.want)
			}
		})
	}
}

// TestCollectCTEDefsSpans checks that the byte-offset spans slice out exactly
// the name, column list, and body the sqleditor projection extractor needs —
// and that a `CREATE VIEW v AS (…)` never masquerades as a CTE named v.
func TestCollectCTEDefsSpans(t *testing.T) {
	sql := `WITH c (a, b) AS (SELECT 1, 2) SELECT * FROM c`
	defs := CollectCTEDefs(sql, sqltok.SignificantTokens(sql))
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1", len(defs))
	}
	d := defs[0]
	if d.Name != "c" || !d.Closed {
		t.Fatalf("name=%q closed=%v, want c/true", d.Name, d.Closed)
	}
	if cols := sql[d.ColsStart:d.ColsEnd]; cols != "(a, b)" {
		t.Errorf("column-list span = %q, want %q", cols, "(a, b)")
	}
	if body := sql[d.BodyStart:d.BodyEnd]; body != "(SELECT 1, 2)" {
		t.Errorf("body span = %q, want %q", body, "(SELECT 1, 2)")
	}

	// CREATE VIEW … AS (…) is not a CTE: the old reCTEDef regex matched it.
	if got := CollectCTEDefs(`CREATE VIEW v AS (SELECT 1)`, sqltok.SignificantTokens(`CREATE VIEW v AS (SELECT 1)`)); len(got) != 0 {
		t.Errorf("CREATE VIEW matched as CTE: %+v", got)
	}

	// No column list → ColsStart is -1.
	nc := CollectCTEDefs(`WITH c AS (SELECT 1) SELECT * FROM c`, sqltok.SignificantTokens(`WITH c AS (SELECT 1) SELECT * FROM c`))
	if len(nc) != 1 || nc[0].ColsStart != -1 {
		t.Errorf("no-column-list ColsStart = %d, want -1", nc[0].ColsStart)
	}
}
