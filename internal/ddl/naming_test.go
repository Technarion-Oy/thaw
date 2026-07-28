// SPDX-License-Identifier: GPL-3.0-or-later

package ddl

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// fn builds a parsed-looking FUNCTION object; sig is the raw argument list.
func fn(name, sig string) Object {
	return Object{
		Kind: KindFunction, Database: "DB", Schema: "SCH", Name: name,
		ArgSig:     parseArgSig(sig),
		ArgSigFull: parseArgSigFull(sig),
		SQL:        "create or replace function DB.SCH." + name + sig + " returns float as $$1$$",
	}
}

// pathsOf flattens a plan list into "path → statement count" for comparison.
func pathsOf(plans []filePlan) map[string]int {
	m := make(map[string]int, len(plans))
	for _, p := range plans {
		m[p.Path] = len(p.Objects)
	}
	return m
}

func TestPlanFiles_ArgTypesKeepsHistoricalNames(t *testing.T) {
	objs := []Object{
		fn("FOO", "(X FLOAT)"),
		fn("FOO", "(X VARCHAR(256))"),
		fn("BAR", "()"),
	}
	got := pathsOf(planFiles(objs, "", "DB", OverloadNamingArgTypes))
	want := map[string]int{
		filepath.Join("DB", "SCH", "functions", "FOO__FLOAT.sql"):   1,
		filepath.Join("DB", "SCH", "functions", "FOO__VARCHAR.sql"): 1,
		filepath.Join("DB", "SCH", "functions", "BAR__noargs.sql"):  1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
}

func TestPlanFiles_SignatureSeparatesSizeOnlyOverloads(t *testing.T) {
	objs := []Object{
		fn("FOO", "(X VARCHAR(16))"),
		fn("FOO", "(X VARCHAR(256))"),
	}
	got := pathsOf(planFiles(objs, "", "DB", OverloadNamingSignature))
	want := map[string]int{
		filepath.Join("DB", "SCH", "functions", "FOO__VARCHAR_16.sql"):  1,
		filepath.Join("DB", "SCH", "functions", "FOO__VARCHAR_256.sql"): 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
}

func TestPlanFiles_GroupedMergesAllOverloadsOfOneName(t *testing.T) {
	objs := []Object{
		fn("FOO", "(X FLOAT)"),
		fn("FOO", "(X VARCHAR(16))"),
		fn("FOO", "()"),
		fn("BAR", "(X FLOAT)"),
	}
	plans := planFiles(objs, "", "DB", OverloadNamingGrouped)
	got := pathsOf(plans)
	want := map[string]int{
		filepath.Join("DB", "SCH", "functions", "FOO.sql"): 3,
		filepath.Join("DB", "SCH", "functions", "BAR.sql"): 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}

	// The merged file holds every overload, each terminated with a semicolon.
	for _, p := range plans {
		if !strings.HasSuffix(p.Path, "FOO.sql") {
			continue
		}
		body := string(p.content())
		for _, sig := range []string{"(X FLOAT)", "(X VARCHAR(16))", "FOO()"} {
			if !strings.Contains(body, sig) {
				t.Errorf("grouped file is missing overload %s:\n%s", sig, body)
			}
		}
		if n := strings.Count(body, ";\n"); n != 3 {
			t.Errorf("grouped file has %d terminated statements, want 3:\n%s", n, body)
		}
	}
}

// Grouping only ever merges genuine overloads. Unrelated objects that a
// schema-less template flattens onto one path still get numbered suffixes.
func TestPlanFiles_GroupedDoesNotMergeUnrelatedObjects(t *testing.T) {
	a := fn("FOO", "(X FLOAT)")
	b := fn("FOO", "(X VARCHAR(16))")
	b.Schema = "OTHER"
	tbl := Object{Kind: KindTable, Database: "DB", Schema: "SCH", Name: "FOO", SQL: "create table FOO(i int)"}

	template := "{database}/{object_name}.sql" // no {schema}, no {object_type}
	got := pathsOf(planFiles([]Object{a, b, tbl}, template, "DB", OverloadNamingGrouped))
	want := map[string]int{
		filepath.Join("DB", "FOO.sql"):   1,
		filepath.Join("DB", "FOO_2.sql"): 1,
		filepath.Join("DB", "FOO_3.sql"): 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
}

// The numeric-suffix fallback must depend only on the object set, not on the
// order Snowflake returned the statements in — otherwise a re-export rewrites
// unrelated files and pollutes the git diff.
func TestPlanFiles_NumberingIsOrderIndependent(t *testing.T) {
	// Three overloads that all sanitize to FOO__VARCHAR under argtypes.
	objs := []Object{
		fn("FOO", "(X VARCHAR(16))"),
		fn("FOO", "(X VARCHAR(256))"),
		fn("FOO", "(X VARCHAR)"),
	}

	assign := func(in []Object) map[string]string {
		m := make(map[string]string, len(in))
		for _, p := range planFiles(in, "", "DB", OverloadNamingArgTypes) {
			m[p.Objects[0].SQL] = p.Path
		}
		return m
	}

	want := assign(objs)
	if len(want) != 3 {
		t.Fatalf("expected 3 distinct paths, got %v", want)
	}
	for _, perm := range [][]int{{2, 0, 1}, {1, 2, 0}, {2, 1, 0}} {
		shuffled := []Object{objs[perm[0]], objs[perm[1]], objs[perm[2]]}
		if got := assign(shuffled); !reflect.DeepEqual(got, want) {
			t.Errorf("order %v produced %v, want %v", perm, got, want)
		}
	}
}

// Grouped-file statement order is likewise signature-derived, so the file
// content is byte-identical no matter how the statements arrived.
func TestPlanFiles_GroupedContentIsOrderIndependent(t *testing.T) {
	objs := []Object{fn("FOO", "(X FLOAT)"), fn("FOO", "(X VARCHAR(16))"), fn("FOO", "()")}
	body := func(in []Object) string {
		plans := planFiles(in, "", "DB", OverloadNamingGrouped)
		if len(plans) != 1 {
			t.Fatalf("expected 1 plan, got %d", len(plans))
		}
		return string(plans[0].content())
	}
	want := body(objs)
	reversed := []Object{objs[2], objs[1], objs[0]}
	if got := body(reversed); got != want {
		t.Errorf("content differs by input order:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// A numbered candidate must never steal the path of a real object of that name.
func TestPlanFiles_NumberedSuffixSkipsRealNames(t *testing.T) {
	objs := []Object{
		fn("FOO", "(X VARCHAR(16))"),
		fn("FOO", "(X VARCHAR(256))"),
		fn("FOO_2", "(X VARCHAR(16))"), // occupies FOO_2__VARCHAR.sql
	}
	plans := planFiles(objs, "", "DB", OverloadNamingArgTypes)

	paths := make([]string, 0, len(plans))
	for _, p := range plans {
		paths = append(paths, p.Path)
	}
	sort.Strings(paths)
	want := []string{
		filepath.Join("DB", "SCH", "functions", "FOO_2__VARCHAR.sql"),
		filepath.Join("DB", "SCH", "functions", "FOO__VARCHAR.sql"),
		filepath.Join("DB", "SCH", "functions", "FOO__VARCHAR_2.sql"),
	}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("paths = %v, want %v", paths, want)
	}
}

func TestPlanFiles_NonOverloadableCollisionsStillNumbered(t *testing.T) {
	t1 := Object{Kind: KindTable, Database: "DB", Schema: "A", Name: "T", SQL: "create table A.T(i int)"}
	t2 := Object{Kind: KindTable, Database: "DB", Schema: "B", Name: "T", SQL: "create table B.T(i int)"}
	got := pathsOf(planFiles([]Object{t1, t2}, "{object_name}.sql", "DB", OverloadNamingGrouped))
	want := map[string]int{"T.sql": 1, "T_2.sql": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
}

func TestPlanFiles_SingleObjectContentUnchanged(t *testing.T) {
	obj := Object{Kind: KindTable, Database: "DB", Schema: "SCH", Name: "T", SQL: "create table T(i int)"}
	plans := planFiles([]Object{obj}, "", "DB", "") // zero value = default strategy
	if len(plans) != 1 {
		t.Fatalf("plans = %d, want 1", len(plans))
	}
	if got, want := string(plans[0].content()), "create table T(i int);\n"; got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestPlanFiles_EmptyInput(t *testing.T) {
	if plans := planFiles(nil, "", "DB", OverloadNamingGrouped); len(plans) != 0 {
		t.Errorf("plans = %v, want empty", plans)
	}
}

func TestOverloadNamingNormalize(t *testing.T) {
	tests := map[OverloadNaming]OverloadNaming{
		"":                        DefaultOverloadNaming,
		"argtypes":                OverloadNamingArgTypes,
		"signature":               OverloadNamingSignature,
		"grouped":                 OverloadNamingGrouped,
		"ARGTYPES":                DefaultOverloadNaming, // case-sensitive by design
		OverloadNaming("bogus"):   DefaultOverloadNaming,
		OverloadNaming("name"):    DefaultOverloadNaming,
		OverloadNaming("Grouped"): DefaultOverloadNaming,
	}
	for in, want := range tests {
		if got := in.normalize(); got != want {
			t.Errorf("OverloadNaming(%q).normalize() = %q, want %q", in, got, want)
		}
	}
}
