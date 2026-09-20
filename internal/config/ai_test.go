// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAIConfigSchemaContextEnabled pins the tri-state behind the "Include schema
// context" switch (#924). Unset is not "off" or "on" but "never asked", and the
// answer depends on whether column names would leave the machine: an install
// that already had a hosted provider configured predates the feature and must
// not start shipping the catalog to OpenAI/Google on upgrade.
func TestAIConfigSchemaContextEnabled(t *testing.T) {
	on, off := true, false

	cases := []struct {
		name string
		cfg  AIConfig
		want bool
	}{
		{"unset + a hosted provider already configured stays off", AIConfig{Provider: "openai"}, false},
		{"unset + google stays off", AIConfig{Provider: "google"}, false},
		{"unset + ollama is on (local, nothing leaves the machine)", AIConfig{Provider: "ollama"}, true},
		{"unset + no provider yet is on (the modal writes an explicit value on save)", AIConfig{}, true},
		{"an explicit yes wins over the hosted-provider default", AIConfig{Provider: "openai", SchemaContext: &on}, true},
		{"an explicit no wins over the ollama default", AIConfig{Provider: "ollama", SchemaContext: &off}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.SchemaContextEnabled(); got != tc.want {
				t.Errorf("SchemaContextEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestAIConfigSchemaContextRoundTrip covers the persistence side of that
// tri-state: unset must survive a load as unset (not collapse to false), and an
// explicit choice must survive in both directions.
func TestAIConfigSchemaContextRoundTrip(t *testing.T) {
	t.Run("a config predating the field loads as unset", func(t *testing.T) {
		var got AIConfig
		if err := json.Unmarshal([]byte(`{"provider":"openai","model":"gpt-4o-mini","enabled":true}`), &got); err != nil {
			t.Fatal(err)
		}
		if got.SchemaContext != nil {
			t.Errorf("SchemaContext = %v, want nil: an absent key must stay distinguishable from an explicit false", *got.SchemaContext)
		}
	})

	t.Run("an explicit choice survives a save/load cycle", func(t *testing.T) {
		for _, want := range []bool{true, false} {
			raw, err := json.Marshal(AIConfig{Provider: "openai", SchemaContext: &want})
			if err != nil {
				t.Fatal(err)
			}
			var got AIConfig
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if got.SchemaContext == nil || *got.SchemaContext != want {
				t.Errorf("round-trip of %v gave %v (raw %s)", want, got.SchemaContext, raw)
			}
		}
	})

	t.Run("unset is not written to disk", func(t *testing.T) {
		raw, err := json.Marshal(AIConfig{Provider: "openai"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "schemaContext") {
			t.Fatalf("unset should stay out of config.json, got %s", raw)
		}
	})
}
