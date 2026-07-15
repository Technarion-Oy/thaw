// SPDX-License-Identifier: GPL-3.0-or-later

package sequence

import (
	"strings"
	"testing"
)

func TestBuildCreateSequenceSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      SequenceConfig
		contains []string
		absent   []string
	}{
		{
			name: "minimal",
			cfg: SequenceConfig{
				Name:      "MY_SEQ",
				Start:     1,
				Increment: 1,
			},
			contains: []string{
				"CREATE SEQUENCE \"DB\".\"SC\".MY_SEQ",
				"START WITH 1",
				"INCREMENT BY 1",
			},
			absent: []string{"OR REPLACE", "IF NOT EXISTS", "ORDER", "NOORDER", "COMMENT"},
		},
		{
			name: "or replace",
			cfg: SequenceConfig{
				Name:      "S",
				OrReplace: true,
				Start:     1,
				Increment: 1,
			},
			contains: []string{"CREATE OR REPLACE SEQUENCE"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists",
			cfg: SequenceConfig{
				Name:        "S",
				IfNotExists: true,
				Start:       1,
				Increment:   1,
			},
			contains: []string{"CREATE SEQUENCE IF NOT EXISTS"},
		},
		{
			name: "or replace suppresses if not exists",
			cfg: SequenceConfig{
				Name:        "S",
				OrReplace:   true,
				IfNotExists: true,
				Start:       1,
				Increment:   1,
			},
			contains: []string{"CREATE OR REPLACE SEQUENCE"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "custom start and increment",
			cfg: SequenceConfig{
				Name:      "S",
				Start:     100,
				Increment: 5,
			},
			contains: []string{"START WITH 100", "INCREMENT BY 5"},
		},
		{
			name: "order",
			cfg: SequenceConfig{
				Name:      "S",
				Start:     1,
				Increment: 1,
				Ordered:   "ORDER",
			},
			contains: []string{"\n  ORDER"},
			absent:   []string{"NOORDER"},
		},
		{
			name: "noorder",
			cfg: SequenceConfig{
				Name:      "S",
				Start:     1,
				Increment: 1,
				Ordered:   "NOORDER",
			},
			contains: []string{"\n  NOORDER"},
		},
		{
			name: "comment escaped",
			cfg: SequenceConfig{
				Name:      "S",
				Start:     1,
				Increment: 1,
				Comment:   "it's fine",
			},
			contains: []string{"COMMENT = 'it''s fine'"},
		},
		{
			name: "case sensitive",
			cfg: SequenceConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
				Start:         1,
				Increment:     1,
			},
			contains: []string{"\"DB\".\"SC\".\"MixedCase\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateSequenceSql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(got, ";") {
				t.Errorf("statement should end with ';', got:\n%s", got)
			}
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, got)
				}
			}
			for _, no := range tt.absent {
				if strings.Contains(got, no) {
					t.Errorf("expected output to NOT contain %q, got:\n%s", no, got)
				}
			}
		})
	}
}

func TestBuildCreateSequenceSqlPlaceholder(t *testing.T) {
	got, err := BuildCreateSequenceSql("DB", "SC", SequenceConfig{Start: 1, Increment: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "sequence_name") {
		t.Errorf("expected placeholder %q in:\n%s", "sequence_name", got)
	}
}
