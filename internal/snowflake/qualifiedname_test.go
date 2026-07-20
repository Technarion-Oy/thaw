// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"reflect"
	"testing"
)

// ── SplitQualifiedName ───────────────────────────────────────────────────────

func TestSplitQualifiedName(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		maxParts int
		want     []IdentPart
	}{
		{
			name:     "single bare part",
			in:       "MYDB",
			maxParts: 2,
			want:     []IdentPart{{Text: "MYDB"}},
		},
		{
			name:     "two bare parts",
			in:       "MYDB.PUBLIC",
			maxParts: 2,
			want:     []IdentPart{{Text: "MYDB"}, {Text: "PUBLIC"}},
		},
		{
			name:     "quoted part keeps case and records the quoting",
			in:       `"myDb".PUBLIC`,
			maxParts: 2,
			want:     []IdentPart{{Text: "myDb", Quoted: true}, {Text: "PUBLIC"}},
		},
		{
			// The whole point of tokenizing: strings.Split would yield 3 parts.
			name:     "quoted part containing a literal dot stays one part",
			in:       `"MY.DB".PUB`,
			maxParts: 2,
			want:     []IdentPart{{Text: "MY.DB", Quoted: true}, {Text: "PUB"}},
		},
		{
			name:     "doubled quotes are unescaped",
			in:       `"my""repo"`,
			maxParts: 2,
			want:     []IdentPart{{Text: `my"repo`, Quoted: true}},
		},
		{
			name:     "surrounding whitespace is trimmed",
			in:       "  DB.SCHEMA  ",
			maxParts: 2,
			want:     []IdentPart{{Text: "DB"}, {Text: "SCHEMA"}},
		},
		{
			name:     "unbounded maxParts reads three parts",
			in:       "DB.SCHEMA.OBJ",
			maxParts: 0,
			want:     []IdentPart{{Text: "DB"}, {Text: "SCHEMA"}, {Text: "OBJ"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SplitQualifiedName(tc.in, tc.maxParts)
			if err != nil {
				t.Fatalf("SplitQualifiedName(%q) unexpected error: %v", tc.in, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("SplitQualifiedName(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitQualifiedNameErrors(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		maxParts int
	}{
		{"empty", "", 2},
		{"trailing dot", "DB.", 2},
		{"leading dot", ".SCHEMA", 2},
		{"beyond maxParts", "DB.SCHEMA.OBJ", 2},
		{"unterminated quote", `"DB`, 2},
		{"stray token", "DB SCHEMA", 2},
		{"empty quoted segment", `"".PUB`, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := SplitQualifiedName(tc.in, tc.maxParts); err == nil {
				t.Errorf("SplitQualifiedName(%q) = %+v, want error", tc.in, got)
			}
		})
	}
}

// ── IdentEqual ───────────────────────────────────────────────────────────────

func TestIdentEqual(t *testing.T) {
	cases := []struct {
		raw  string
		name string
		want bool
	}{
		// Bare identifiers fold case (Snowflake uppercases them).
		{"MYSTAGE", "MYSTAGE", true},
		{"mystage", "MYSTAGE", true},
		{"MyStage", "MYSTAGE", true},
		{"OTHER", "MYSTAGE", false},
		// A quoted identifier preserves case, so the match is exact.
		{`"myStage"`, "myStage", true},
		{`"myStage"`, "MYSTAGE", false},
		{`"MYSTAGE"`, "MYSTAGE", true},
		// Doubled quotes are the escape for an embedded quote.
		{`"my""repo"`, `my"repo`, true},
		{`"my""repo"`, `my""repo`, false},
		// Surrounding whitespace is not significant.
		{"  MYSTAGE  ", "MYSTAGE", true},
	}
	for _, tc := range cases {
		t.Run(tc.raw+"/"+tc.name, func(t *testing.T) {
			if got := IdentEqual(tc.raw, tc.name); got != tc.want {
				t.Errorf("IdentEqual(%q, %q) = %v, want %v", tc.raw, tc.name, got, tc.want)
			}
		})
	}
}

// ── NormalizeStageRef ────────────────────────────────────────────────────────

func TestNormalizeStageRef(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "@MYSTAGE", want: "@MYSTAGE"},
		{in: "MYSTAGE", want: "@MYSTAGE"}, // '@' is added
		{in: "DB.SCHEMA.STAGE/a", want: "@DB.SCHEMA.STAGE/a"},
		{in: `"my stage"/a`, want: `@"my stage"/a`}, // quoted names may hold spaces
		{in: "stage/../etc", wantErr: true},         // traversal
		{in: "stage; DROP TABLE t", wantErr: true},  // injection
		{in: "stage/a -- comment", wantErr: true},   // SQL comment
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := NormalizeStageRef(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizeStageRef(%q) = %q, want error", tc.in, got)
				}
				if got != "" {
					t.Errorf("NormalizeStageRef(%q) returned %q alongside an error, want \"\"", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeStageRef(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("NormalizeStageRef(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// ── ColumnIndexes / StrVal ───────────────────────────────────────────────────

func TestColumnIndexes(t *testing.T) {
	res := &QueryResult{Columns: []string{"name", "SIZE", "Last_Modified"}}

	idxs := ColumnIndexes(res, "name", "size", "last_modified", "md5")
	want := map[string]int{"name": 0, "size": 1, "last_modified": 2, "md5": -1}
	if !reflect.DeepEqual(idxs, want) {
		t.Errorf("ColumnIndexes = %v, want %v", idxs, want)
	}

	// Requested names fold case too, so the caller may spell them either way.
	if got := ColumnIndexes(res, "NAME")["name"]; got != 0 {
		t.Errorf("ColumnIndexes(NAME)[name] = %d, want 0", got)
	}

	// A nil result yields all -1 so cells can be read unconditionally.
	if got := ColumnIndexes(nil, "name")["name"]; got != -1 {
		t.Errorf("ColumnIndexes(nil)[name] = %d, want -1", got)
	}
}

func TestStrVal(t *testing.T) {
	row := []interface{}{"  padded  ", nil, []byte("bytes"), 42, int64(7)}
	cases := []struct {
		name string
		idx  int
		want string
	}{
		{"string is trimmed", 0, "padded"},
		{"nil cell", 1, ""},
		{"[]byte decoded as text", 2, "bytes"},
		{"int formatted", 3, "42"},
		{"int64 formatted", 4, "7"},
		{"negative index", -1, ""},
		{"index past end", 99, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StrVal(row, tc.idx); got != tc.want {
				t.Errorf("StrVal(row, %d) = %q, want %q", tc.idx, got, tc.want)
			}
		})
	}
}

// ── LIST NAME prefix stripping ───────────────────────────────────────────────

func TestSplitStagePrefix(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantPrefix string
		wantRest   string
		wantOK     bool
	}{
		{"bare stage and path", "MYSTAGE/a/b.txt", "MYSTAGE", "a/b.txt", true},
		{"qualified", "DB.SCHEMA.MYSTAGE/a.txt", "DB.SCHEMA.MYSTAGE", "a.txt", true},
		{"no path", "MYSTAGE", "MYSTAGE", "", false},
		{
			// A '/' inside a quoted identifier is part of the name, not the
			// stage/path separator — strings.Index would split here.
			name:       "slash inside a quoted stage name",
			in:         `"my/stage"/a.txt`,
			wantPrefix: `"my/stage"`,
			wantRest:   "a.txt",
			wantOK:     true,
		},
		{
			name:       "quote in a filename does not open an identifier",
			in:         `MYSTAGE/we"ird/x.txt`,
			wantPrefix: "MYSTAGE",
			wantRest:   `we"ird/x.txt`,
			wantOK:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prefix, rest, ok := splitStagePrefix(tc.in)
			if prefix != tc.wantPrefix || rest != tc.wantRest || ok != tc.wantOK {
				t.Errorf("splitStagePrefix(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.in, prefix, rest, ok, tc.wantPrefix, tc.wantRest, tc.wantOK)
			}
		})
	}
}

func TestStagePrefixMatches(t *testing.T) {
	cases := []struct {
		prefix string
		stage  string
		want   bool
	}{
		{"MYSTAGE", "MYSTAGE", true},
		{"mystage", "MYSTAGE", true},
		{`"MYSTAGE"`, "MYSTAGE", true},
		{"@MYSTAGE", "MYSTAGE", true},
		{"DB.SCHEMA.MYSTAGE", "MYSTAGE", true},
		{`@DB.SCHEMA."MYSTAGE"`, "MYSTAGE", true},
		{`"MY.DB".SCHEMA.MYSTAGE`, "MYSTAGE", true},
		// A quoted prefix preserves case, so it must match exactly.
		{`"mystage"`, "MYSTAGE", false},
		{`"myStage"`, "myStage", true},
		// The '@' sigil alone is not proof of a stage prefix any more: the name
		// after it still has to match.
		{"@SOMETHINGELSE", "MYSTAGE", false},
		{"@", "MYSTAGE", false},
		{"NOTMYSTAGE", "MYSTAGE", false},
		{"DB.SCHEMA.OTHER", "MYSTAGE", false},
		// The implicit user stage: "~" is not a parseable identifier, so this
		// exercises the whole-segment IdentEqual fallback.
		{"~", "~", true},
		{"@~", "~", true},
		{"dir", "~", false},
	}
	for _, tc := range cases {
		t.Run(tc.prefix+"/"+tc.stage, func(t *testing.T) {
			if got := stagePrefixMatches(tc.prefix, tc.stage); got != tc.want {
				t.Errorf("stagePrefixMatches(%q, %q) = %v, want %v", tc.prefix, tc.stage, got, tc.want)
			}
		})
	}
}

func TestParseListEntries(t *testing.T) {
	listResult := func(names ...string) *QueryResult {
		res := &QueryResult{Columns: []string{"name", "size"}}
		for _, n := range names {
			res.Rows = append(res.Rows, []interface{}{n, "10"})
		}
		return res
	}

	t.Run("strips the stage prefix in each rendering", func(t *testing.T) {
		res := listResult(
			"MYREPO/branches/main/a.txt",
			"myrepo/branches/main/b.txt",
			"DB.SCHEMA.MYREPO/branches/main/c.txt",
			"@DB.SCHEMA.MYREPO/branches/main/d.txt",
		)
		entries := parseListEntries(res, "MYREPO", "branches/main/")
		var got []string
		for _, e := range entries {
			got = append(got, e.Name)
		}
		want := []string{"a.txt", "b.txt", "c.txt", "d.txt"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("names = %v, want %v", got, want)
		}
	})

	t.Run("directories are collapsed to a single entry", func(t *testing.T) {
		res := listResult("MYREPO/dir/a.txt", "MYREPO/dir/b.txt", "MYREPO/top.txt")
		entries := parseListEntries(res, "MYREPO", "")
		if len(entries) != 2 {
			t.Fatalf("len(entries) = %d, want 2 (%+v)", len(entries), entries)
		}
		if !entries[0].IsDir || entries[0].Name != "dir" {
			t.Errorf("entries[0] = %+v, want the directory 'dir'", entries[0])
		}
		if entries[1].IsDir || entries[1].Name != "top.txt" || entries[1].Size != 10 {
			t.Errorf("entries[1] = %+v, want the file 'top.txt' of size 10", entries[1])
		}
	})

	t.Run("a non-matching @ prefix is not stripped", func(t *testing.T) {
		// Previously any '@'-leading segment was treated as the stage prefix.
		res := listResult("@OTHERSTAGE/a.txt")
		if entries := parseListEntries(res, "MYREPO", ""); len(entries) != 1 || entries[0].Name != "@OTHERSTAGE" {
			t.Errorf("entries = %+v, want the un-stripped '@OTHERSTAGE' directory", entries)
		}
	})

	t.Run("missing NAME column yields no entries", func(t *testing.T) {
		res := &QueryResult{Columns: []string{"size"}, Rows: [][]interface{}{{"10"}}}
		if entries := parseListEntries(res, "MYREPO", ""); len(entries) != 0 {
			t.Errorf("entries = %+v, want none", entries)
		}
	})
}
