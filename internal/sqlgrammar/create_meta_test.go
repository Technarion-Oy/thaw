package sqlgrammar

import (
	"reflect"
	"strings"
	"testing"
)

// parseRulesByPrefix returns every Parse* grammar rule whose name starts with
// prefix as a (name, method-value) pair via reflection, so invariants can be
// asserted across whole command families without hand-maintaining a list.
func parseRulesByPrefix(prefix string) []struct {
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
		if !strings.HasPrefix(m.Name, prefix) {
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

// TestParseRules_RejectGarbage asserts that no implemented grammar rule fully
// accepts input that contains no significant tokens or that begins with a word
// that starts no command. A rule that does is either still a stub (return true)
// or far too lenient — both regressions worth failing on. These inputs are
// chosen to be invalid for EVERY command family (unlike e.g. "SELECT 1", which
// ParseSelect legitimately accepts).
func TestParseRules_RejectGarbage(t *testing.T) {
	garbage := []string{
		``,
		`   `,
		"\n\t ",
		`/* only a comment */`,
		`-- only a line comment`,
		`zzqqxx_not_a_command foo bar`,
		`42 is not a statement`,
	}
	rules := parseRulesByPrefix("Parse")
	if len(rules) < 700 {
		t.Fatalf("expected to discover the full Parse* rule set via reflection, found only %d", len(rules))
	}
	for _, r := range rules {
		for _, g := range garbage {
			if parseRule(g, r.rule) {
				t.Errorf("%s fully accepted non-command input %q (stub or over-lenient)", r.name, g)
			}
		}
	}
}
