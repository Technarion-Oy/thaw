package sqlgrammar

import (
	"reflect"
	"strings"
	"testing"
)

// createRules returns every ParseCreate* grammar rule as a (name, method-value)
// pair via reflection, so invariants can be asserted across all of them without
// hand-maintaining a list.
func createRules() []struct {
	name string
	rule func(*Validator) bool
} {
	var out []struct {
		name string
		rule func(*Validator) bool
	}
	vt := reflect.TypeFor[*Validator]()
	for i := 0; i < vt.NumMethod(); i++ {
		m := vt.Method(i)
		if !strings.HasPrefix(m.Name, "ParseCreate") {
			continue
		}
		if m.Type.NumIn() != 1 || m.Type.NumOut() != 1 || m.Type.Out(0).Kind() != reflect.Bool {
			continue
		}
		name := m.Name
		out = append(out, struct {
			name string
			rule func(*Validator) bool
		}{name, func(v *Validator) bool {
			return reflect.ValueOf(v).MethodByName(name).Call(nil)[0].Bool()
		}})
	}
	return out
}

// TestCreateRules_RejectGarbage asserts that no implemented CREATE rule fully
// accepts empty input or a clearly non-CREATE statement. A rule that does is
// either still a stub (return true) or far too lenient.
func TestCreateRules_RejectGarbage(t *testing.T) {
	garbage := []string{
		``,
		`   `,
		`SELECT 1`,
		`DROP TABLE foo`,
		`this is not sql at all`,
	}
	rules := createRules()
	if len(rules) < 100 {
		t.Fatalf("expected to discover the full ParseCreate* rule set via reflection, found only %d", len(rules))
	}
	for _, r := range rules {
		for _, g := range garbage {
			if parseRule(g, r.rule) {
				t.Errorf("%s fully accepted non-CREATE input %q (stub or over-lenient)", r.name, g)
			}
		}
	}
}
