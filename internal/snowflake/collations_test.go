// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import (
	"strings"
	"testing"
)

func TestCollationsContainsUtf8First(t *testing.T) {
	opts := Collations()
	if len(opts) == 0 {
		t.Fatal("Collations returned no options")
	}
	if opts[0].Value != "utf8" {
		t.Errorf("expected first option to be utf8, got %q", opts[0].Value)
	}
}

func TestCollationsCoversEveryLocaleVariant(t *testing.T) {
	opts := Collations()
	got := make(map[string]bool, len(opts))
	for _, o := range opts {
		if o.Value == "" {
			t.Error("collation option has empty value")
		}
		if o.Label == "" {
			t.Errorf("collation option %q has empty label", o.Value)
		}
		got[o.Value] = true
	}

	// utf8 + (locales × variants)
	wantCount := 1 + len(collationLocales)*len(commonCollationVariants)
	if len(opts) != wantCount {
		t.Errorf("expected %d collation options, got %d", wantCount, len(opts))
	}

	for _, want := range []string{"en", "en-ci", "en-ci-ai", "en-cs", "ja", "zh-ci", "fr_CA-ci-ai"} {
		if !got[want] {
			t.Errorf("expected collation %q to be present", want)
		}
	}
}

func TestCollationsLabelsAnnotateVariants(t *testing.T) {
	opts := Collations()
	for _, o := range opts {
		if strings.HasSuffix(o.Value, "-ci") && !strings.Contains(o.Label, "case-insensitive") {
			t.Errorf("expected -ci label to mention case-insensitive, got %q", o.Label)
		}
	}
}

func TestCollationLocalesReturnsCopy(t *testing.T) {
	a := CollationLocales()
	if len(a) != len(collationLocales) {
		t.Fatalf("expected %d locales, got %d", len(collationLocales), len(a))
	}
	a[0].Code = "MUTATED"
	if collationLocales[0].Code == "MUTATED" {
		t.Error("CollationLocales must return a copy, not the backing slice")
	}
}

func TestCollationSpecifiersReturnsCopy(t *testing.T) {
	a := CollationSpecifiers()
	if len(a) != len(collationSpecifiers) {
		t.Fatalf("expected %d specifiers, got %d", len(collationSpecifiers), len(a))
	}
	a[0].Code = "MUTATED"
	if collationSpecifiers[0].Code == "MUTATED" {
		t.Error("CollationSpecifiers must return a copy, not the backing slice")
	}
}

func TestCollationSpecifiersCoverSpec(t *testing.T) {
	specs := CollationSpecifiers()
	got := make(map[string]bool, len(specs))
	for _, s := range specs {
		got[s.Code] = true
	}
	for _, want := range []string{"ci", "cs", "ai", "as", "pi", "ps", "fl", "fu", "lower", "upper", "trim", "ltrim", "rtrim"} {
		if !got[want] {
			t.Errorf("expected specifier %q to be present", want)
		}
	}
}

func TestBuildCollation(t *testing.T) {
	cases := []struct {
		name       string
		locale     string
		specifiers []string
		want       string
	}{
		{"locale only", "en", nil, "en"},
		{"locale and one specifier", "en", []string{"ci"}, "en-ci"},
		{"locale and many specifiers", "en", []string{"ci", "ai"}, "en-ci-ai"},
		{"specifiers only", "", []string{"ci", "trim"}, "ci-trim"},
		{"empty segments skipped", "  en ", []string{"", " ai "}, "en-ai"},
		{"all empty", "", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildCollation(tc.locale, tc.specifiers...); got != tc.want {
				t.Errorf("BuildCollation(%q, %v) = %q, want %q", tc.locale, tc.specifiers, got, tc.want)
			}
		})
	}
}
