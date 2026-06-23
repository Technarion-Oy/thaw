package sqlgrammar

import "testing"

// parseRule runs a single grammar rule (given as a method expression such as
// (*Validator).ParseCreateAlert) over sql and reports success only when the rule
// matched AND every significant token was consumed — the all-or-nothing check a
// top-level diagnostics consumer applies. Shared by all per-command test files.
func parseRule(sql string, rule func(*Validator) bool) bool {
	v := New(sql)
	return rule(v) && v.AtEnd()
}

// assertValid fails t for every statement the rule does not fully accept.
func assertValid(t *testing.T, rule func(*Validator) bool, cases ...string) {
	t.Helper()
	for _, sql := range cases {
		if !parseRule(sql, rule) {
			v := New(sql)
			rule(v)
			t.Errorf("expected VALID, got failure for %q: %s", sql, v.Failure().Message())
		}
	}
}

// assertInvalid fails t for every statement the rule fully accepts.
func assertInvalid(t *testing.T, rule func(*Validator) bool, cases ...string) {
	t.Helper()
	for _, sql := range cases {
		if parseRule(sql, rule) {
			t.Errorf("expected INVALID, but rule fully accepted %q", sql)
		}
	}
}
