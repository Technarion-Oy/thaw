// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"reflect"
	"testing"
)

func TestResultPropertyValueRows(t *testing.T) {
	// One row per property, with property/value columns in arbitrary order/case.
	res := &QueryResult{
		Columns: []string{"value", "property"},
		Rows: [][]interface{}{
			{"[PASSWORD, SAML]", "AUTHENTICATION_METHODS"},
			{"{}", "MFA_POLICY"},
		},
	}
	got := ResultPropertyValueRows(res)
	want := []PropertyPair{
		{Key: "AUTHENTICATION_METHODS", Value: "[PASSWORD, SAML]"},
		{Key: "MFA_POLICY", Value: "{}"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}

	// Empty / shapeless inputs yield a non-nil empty slice.
	for _, tc := range []*QueryResult{
		nil,
		{Columns: []string{"property", "value"}}, // no rows
		{Columns: []string{"a", "b"}, Rows: [][]any{{1, 2}}}, // missing the columns
	} {
		if got := ResultPropertyValueRows(tc); got == nil || len(got) != 0 {
			t.Errorf("ResultPropertyValueRows(%+v) = %+v, want empty non-nil", tc, got)
		}
	}
}
