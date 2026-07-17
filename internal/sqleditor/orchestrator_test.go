// SPDX-License-Identifier: GPL-3.0-or-later

package sqleditor

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeProvider is an in-memory SchemaProvider for testing the schema-aware
// diagnostics path without a live Snowflake connection (issue #354). A missing
// map entry for ListSchemas/ListObjects returns an error, simulating a
// shared/unlistable DB whose catalog level was never fetched — which is what
// keeps the fetched* guards unset.
type fakeProvider struct {
	session    SessionContext
	databases  []string
	schemas    map[string][]string      // UC(db) → schema names
	objects    map[string][]StoreObject // UC(db)\0UC(schema) → objects
	columns    map[string][]ColInfo     // UC(db)\0UC(schema)\0UC(name) → cols
	sessionErr error
	dbErr      error
}

func (p *fakeProvider) SessionContext(context.Context) (SessionContext, error) {
	if p.sessionErr != nil {
		return SessionContext{}, p.sessionErr
	}
	return p.session, nil
}

func (p *fakeProvider) ListDatabases(context.Context) ([]string, error) {
	if p.dbErr != nil {
		return nil, p.dbErr
	}
	return p.databases, nil
}

func (p *fakeProvider) ListSchemas(_ context.Context, db string) ([]string, error) {
	s, ok := p.schemas[strings.ToUpper(db)]
	if !ok {
		return nil, errors.New("SHOW SCHEMAS failed")
	}
	return s, nil
}

func (p *fakeProvider) ListObjects(_ context.Context, db, schema string) ([]StoreObject, error) {
	o, ok := p.objects[schemaObjectKey(db, schema)]
	if !ok {
		return nil, errors.New("SHOW OBJECTS failed")
	}
	return o, nil
}

func (p *fakeProvider) TableColumns(_ context.Context, db, schema, name string) ([]ColInfo, error) {
	key := strings.ToUpper(db) + "\x00" + strings.ToUpper(schema) + "\x00" + strings.ToUpper(name)
	c, ok := p.columns[key]
	if !ok {
		return nil, errors.New("DESCRIBE TABLE failed")
	}
	return c, nil
}

// standardCatalog is a fully-fetched catalog: MYDB with schema PUBLIC holding an
// ORDERS table and a CUSTOMERS view.
func standardCatalog() diagCatalog {
	return diagCatalog{
		databases: []string{"MYDB"},
		schemas:   []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
		objects: []StoreObject{
			{DB: "MYDB", Schema: "PUBLIC", Name: "ORDERS", Kind: "TABLE"},
			{DB: "MYDB", Schema: "PUBLIC", Name: "CUSTOMERS", Kind: "VIEW"},
		},
		fetchedSchemas: map[string]bool{"MYDB": true},
		fetchedObjects: map[string]bool{schemaObjectKey("MYDB", "PUBLIC"): true},
	}
}

func hasResolved(refs []ResolvedRef, db, schema, name string) bool {
	for _, r := range refs {
		if strings.EqualFold(r.DB, db) && strings.EqualFold(r.Schema, schema) && strings.EqualFold(r.Name, name) {
			return true
		}
	}
	return false
}

// TestResolveRefsForDiagnostics locks in the ref-resolution semantics shared with
// the frontend editor: a fully-qualified ref is trusted only after catalog
// verification (typos dropped so ValidateTablesExist flags them), and an
// unqualified ref resolves only against fetched TABLE/VIEW objects.
func TestResolveRefsForDiagnostics(t *testing.T) {
	cat := standardCatalog()

	tests := []struct {
		name string
		ref  JoinTableRef
		keep bool // whether the ref should survive resolution
	}{
		{"valid fully-qualified table", JoinTableRef{DB: "MYDB", Schema: "PUBLIC", Name: "ORDERS"}, true},
		{"valid fully-qualified view (kind-agnostic)", JoinTableRef{DB: "MYDB", Schema: "PUBLIC", Name: "CUSTOMERS"}, true},
		{"typo database dropped", JoinTableRef{DB: "NODB", Schema: "PUBLIC", Name: "ORDERS"}, false},
		{"typo schema dropped", JoinTableRef{DB: "MYDB", Schema: "NOSCH", Name: "ORDERS"}, false},
		{"typo table dropped", JoinTableRef{DB: "MYDB", Schema: "PUBLIC", Name: "TYPO"}, false},
		{"unqualified resolved via store", JoinTableRef{Name: "ORDERS"}, true},
		{"unqualified view resolved via store", JoinTableRef{Name: "CUSTOMERS"}, true},
		{"unqualified miss dropped", JoinTableRef{Name: "NOPE"}, false},
		{"USE ref skipped", JoinTableRef{Name: ""}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveRefsForDiagnostics([]JoinTableRef{tc.ref}, cat)
			if tc.keep && len(got) != 1 {
				t.Fatalf("expected ref kept, got %d refs: %+v", len(got), got)
			}
			if !tc.keep && len(got) != 0 {
				t.Fatalf("expected ref dropped, got %+v", got)
			}
		})
	}
}

// TestResolveRefsForDiagnostics_CaseInsensitive verifies that lowercased
// qualifiers still match the uppercase catalog (Snowflake folds unquoted idents).
func TestResolveRefsForDiagnostics_CaseInsensitive(t *testing.T) {
	got := resolveRefsForDiagnostics(
		[]JoinTableRef{{DB: "mydb", Schema: "public", Name: "orders"}},
		standardCatalog(),
	)
	if !hasResolved(got, "mydb", "public", "orders") {
		t.Fatalf("case-insensitive fully-qualified ref should resolve, got %+v", got)
	}
}

// TestResolveRefsForDiagnostics_UnverifiableGuards verifies the "can't verify →
// trust" guards that keep shared/unlistable DBs from false-positiving: an empty
// database list, or a DB whose schemas/objects were never fetched, leaves a
// fully-qualified ref trusted rather than dropping it.
func TestResolveRefsForDiagnostics_UnverifiableGuards(t *testing.T) {
	t.Run("empty database list trusts ref", func(t *testing.T) {
		cat := diagCatalog{
			fetchedSchemas: map[string]bool{},
			fetchedObjects: map[string]bool{},
		}
		got := resolveRefsForDiagnostics(
			[]JoinTableRef{{DB: "ANYDB", Schema: "X", Name: "Y"}}, cat)
		if !hasResolved(got, "ANYDB", "X", "Y") {
			t.Fatalf("unverifiable ref (no db list) should be trusted, got %+v", got)
		}
	})

	t.Run("unfetched schema level trusts ref", func(t *testing.T) {
		// DB is known, but its schema list was never fetched (SHOW SCHEMAS failed).
		cat := diagCatalog{
			databases:      []string{"SHAREDDB"},
			fetchedSchemas: map[string]bool{},
			fetchedObjects: map[string]bool{},
		}
		got := resolveRefsForDiagnostics(
			[]JoinTableRef{{DB: "SHAREDDB", Schema: "X", Name: "Y"}}, cat)
		if !hasResolved(got, "SHAREDDB", "X", "Y") {
			t.Fatalf("ref in unfetched-schema DB should be trusted, got %+v", got)
		}
	})

	t.Run("unfetched object level trusts ref", func(t *testing.T) {
		// DB + schema are known, but the schema's objects were never fetched.
		cat := diagCatalog{
			databases:      []string{"MYDB"},
			schemas:        []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
			fetchedSchemas: map[string]bool{"MYDB": true},
			fetchedObjects: map[string]bool{},
		}
		got := resolveRefsForDiagnostics(
			[]JoinTableRef{{DB: "MYDB", Schema: "PUBLIC", Name: "ANYTABLE"}}, cat)
		if !hasResolved(got, "MYDB", "PUBLIC", "ANYTABLE") {
			t.Fatalf("ref in unfetched-object schema should be trusted, got %+v", got)
		}
	})
}

// TestDiagnose_NilProvider verifies that phase 1 always runs and returns pure
// markers even without a Snowflake connection.
func TestDiagnose_NilProvider(t *testing.T) {
	markers, err := Diagnose(context.Background(), nil, "SELECT (1")
	if err != nil {
		t.Fatalf("nil-provider Diagnose should not error, got %v", err)
	}
	if len(markers) == 0 {
		t.Fatal("expected a phase-1 syntax marker for the unmatched paren")
	}
}

// TestDiagnose_SchemaAwareTypoFlagged is the end-to-end proof of the divergence
// fix: a fully-qualified reference to a nonexistent database is dropped during
// resolution and surfaces as a "does not exist" marker — matching the editor,
// not silently trusted.
func TestDiagnose_SchemaAwareTypoFlagged(t *testing.T) {
	p := &fakeProvider{
		session:   SessionContext{Database: "MYDB", Schema: "PUBLIC"},
		databases: []string{"MYDB"},
		schemas:   map[string][]string{"MYDB": {"PUBLIC"}},
		objects: map[string][]StoreObject{
			schemaObjectKey("MYDB", "PUBLIC"): {
				{DB: "MYDB", Schema: "PUBLIC", Name: "ORDERS", Kind: "TABLE"},
			},
		},
		columns: map[string][]ColInfo{
			"MYDB\x00PUBLIC\x00ORDERS": {{Name: "ID", DataType: "NUMBER(38,0)"}},
		},
	}

	markers, err := Diagnose(context.Background(), p, "SELECT * FROM NODB.PUBLIC.ORDERS")
	if err != nil {
		t.Fatalf("Diagnose error: %v", err)
	}
	found := false
	for _, m := range markers {
		if strings.Contains(m.Message, "NODB") && strings.Contains(m.Message, "does not exist") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a 'database does not exist' marker for the typo DB, got %+v", markers)
	}
}

// TestDiagnose_SchemaAwareCleanValidRef verifies that a valid fully-qualified
// reference produces no existence markers.
func TestDiagnose_SchemaAwareCleanValidRef(t *testing.T) {
	p := &fakeProvider{
		session:   SessionContext{Database: "MYDB", Schema: "PUBLIC"},
		databases: []string{"MYDB"},
		schemas:   map[string][]string{"MYDB": {"PUBLIC"}},
		objects: map[string][]StoreObject{
			schemaObjectKey("MYDB", "PUBLIC"): {
				{DB: "MYDB", Schema: "PUBLIC", Name: "ORDERS", Kind: "TABLE"},
			},
		},
		columns: map[string][]ColInfo{
			"MYDB\x00PUBLIC\x00ORDERS": {{Name: "ID", DataType: "NUMBER(38,0)"}},
		},
	}

	markers, err := Diagnose(context.Background(), p, "SELECT ID FROM MYDB.PUBLIC.ORDERS")
	if err != nil {
		t.Fatalf("Diagnose error: %v", err)
	}
	for _, m := range markers {
		if strings.Contains(m.Message, "does not exist") {
			t.Fatalf("clean valid ref should produce no existence marker, got %q", m.Message)
		}
	}
}

// TestDiagnose_Phase2ErrorFallsBack verifies that a phase-2 failure (session
// context lookup errors) returns phase-1 markers plus the non-fatal error, so the
// caller can log it and still surface pure diagnostics.
func TestDiagnose_Phase2ErrorFallsBack(t *testing.T) {
	p := &fakeProvider{sessionErr: errors.New("no connection")}

	markers, err := Diagnose(context.Background(), p, "SELECT (1")
	if err == nil {
		t.Fatal("expected a non-fatal phase-2 error")
	}
	if len(markers) == 0 {
		t.Fatal("expected phase-1 markers despite the phase-2 error")
	}
}

// TestDiagnose_NoRefsShortCircuits verifies SQL with no table references skips
// metadata gathering (the ListDatabases error would surface otherwise).
func TestDiagnose_NoRefsShortCircuits(t *testing.T) {
	p := &fakeProvider{
		session: SessionContext{Database: "MYDB", Schema: "PUBLIC"},
		dbErr:   errors.New("ListDatabases must not be called"),
	}

	if _, err := Diagnose(context.Background(), p, "SELECT 1"); err != nil {
		t.Fatalf("SQL with no refs should short-circuit before ListDatabases, got %v", err)
	}
}
