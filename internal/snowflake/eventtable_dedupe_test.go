// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "testing"

func TestDedupeEventTables(t *testing.T) {
	basic := []SnowflakeObject{
		{Name: "ORDERS", Kind: "TABLE", Schema: "PUBLIC"},
		{Name: "EVENTS", Kind: "TABLE", Schema: "PUBLIC"},    // surfaced as an event table too
		{Name: "events", Kind: "TABLE", Schema: "analytics"}, // different schema, case-insensitive match
		{Name: "CUSTOMERS", Kind: "VIEW", Schema: "PUBLIC"},
	}
	extended := []SnowflakeObject{
		{Name: "EVENTS", Kind: "EVENT TABLE", Schema: "PUBLIC"},
		{Name: "EVENTS", Kind: "EVENT TABLE", Schema: "ANALYTICS"},
		{Name: "MY_PIPE", Kind: "PIPE", Schema: "PUBLIC"},
	}

	got := dedupeEventTables(basic, extended)

	var names []string
	for _, o := range got {
		names = append(names, o.Schema+"."+o.Name+":"+o.Kind)
	}

	// Both EVENTS basic rows (PUBLIC + analytics) must be dropped; ORDERS and
	// CUSTOMERS must remain.
	want := map[string]bool{
		"PUBLIC.ORDERS:TABLE":   true,
		"PUBLIC.CUSTOMERS:VIEW": true,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d objects, got %d: %v", len(want), len(got), names)
	}
	for _, o := range got {
		key := o.Schema + "." + o.Name + ":" + o.Kind
		if !want[key] {
			t.Errorf("unexpected object survived dedup: %s", key)
		}
	}
}

func TestDedupeEventTablesNoEventTables(t *testing.T) {
	basic := []SnowflakeObject{
		{Name: "ORDERS", Kind: "TABLE", Schema: "PUBLIC"},
	}
	extended := []SnowflakeObject{
		{Name: "MY_PIPE", Kind: "PIPE", Schema: "PUBLIC"},
	}
	got := dedupeEventTables(basic, extended)
	// No event tables → basic returned unchanged.
	if len(got) != 1 || got[0].Name != "ORDERS" {
		t.Fatalf("expected basic returned unchanged, got %v", got)
	}
}
