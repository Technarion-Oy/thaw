// SPDX-License-Identifier: GPL-3.0-or-later

package storagelifecyclepolicy

import (
	"strings"
	"testing"
)

func TestBuildCreateStorageLifecyclePolicySql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      StorageLifecyclePolicyConfig
		contains []string
		absent   []string
	}{
		{
			name: "full config with archive options",
			cfg: StorageLifecyclePolicyConfig{
				Name:           "RETAIN_365",
				OrReplace:      true,
				Args:           []StorageLifecycleArg{{Name: "created_at", Type: "TIMESTAMP_NTZ"}},
				Body:           "created_at < DATEADD('day', -365, CURRENT_TIMESTAMP())",
				ArchiveTier:    "COLD",
				ArchiveForDays: 180,
				Comment:        "expire after a year",
			},
			contains: []string{
				"CREATE OR REPLACE STORAGE LIFECYCLE POLICY \"DB\".\"SC\".RETAIN_365 AS",
				"(created_at TIMESTAMP_NTZ) RETURNS BOOLEAN ->",
				"created_at < DATEADD('day', -365, CURRENT_TIMESTAMP())",
				"ARCHIVE_TIER = COLD",
				"ARCHIVE_FOR_DAYS = 180",
				"COMMENT = 'expire after a year'",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "multiple signature columns",
			cfg: StorageLifecyclePolicyConfig{
				Name: "MULTI",
				Args: []StorageLifecycleArg{
					{Name: "ts", Type: "TIMESTAMP_NTZ"},
					{Name: "region", Type: "VARCHAR"},
				},
				Body: "ts < CURRENT_TIMESTAMP() AND region = 'EU'",
			},
			contains: []string{"(ts TIMESTAMP_NTZ, region VARCHAR) RETURNS BOOLEAN ->"},
		},
		{
			name: "empty arg rows dropped",
			cfg: StorageLifecyclePolicyConfig{
				Name: "A",
				Args: []StorageLifecycleArg{{Name: "", Type: "VARCHAR"}, {Name: "good", Type: "DATE"}},
			},
			contains: []string{"(good DATE) RETURNS BOOLEAN ->"},
		},
		{
			name: "archive tier lowercased input is upper-cased",
			cfg: StorageLifecyclePolicyConfig{
				Name:           "A",
				ArchiveTier:    "cool",
				ArchiveForDays: 90,
			},
			contains: []string{"ARCHIVE_TIER = COOL", "ARCHIVE_FOR_DAYS = 90"},
		},
		{
			name: "tier without days emits neither (both-or-neither)",
			cfg: StorageLifecyclePolicyConfig{
				Name:        "A",
				ArchiveTier: "COLD",
			},
			absent: []string{"ARCHIVE_TIER", "ARCHIVE_FOR_DAYS"},
		},
		{
			name: "days without tier emits neither (both-or-neither)",
			cfg: StorageLifecyclePolicyConfig{
				Name:           "A",
				ArchiveForDays: 90,
			},
			absent: []string{"ARCHIVE_TIER", "ARCHIVE_FOR_DAYS"},
		},
		{
			name: "or replace suppresses if not exists",
			cfg: StorageLifecyclePolicyConfig{
				Name:        "A",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE STORAGE LIFECYCLE POLICY"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists when not or replace",
			cfg: StorageLifecyclePolicyConfig{
				Name:        "A",
				IfNotExists: true,
			},
			contains: []string{"CREATE STORAGE LIFECYCLE POLICY IF NOT EXISTS"},
		},
		{
			name: "single quotes escaped in comment",
			cfg: StorageLifecyclePolicyConfig{
				Name:    "A",
				Comment: "o'hare",
			},
			contains: []string{"COMMENT = 'o''hare'"},
		},
		{
			name: "comment omitted by default",
			cfg: StorageLifecyclePolicyConfig{
				Name: "A",
			},
			absent: []string{"COMMENT"},
		},
		{
			name: "case-sensitive name is quoted",
			cfg: StorageLifecyclePolicyConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
			},
			contains: []string{"\"MixedCase\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreateStorageLifecyclePolicySql("DB", "SC", tt.cfg)
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

// TestBuildCreateStorageLifecyclePolicySqlPlaceholders verifies that an empty
// config still yields a well-formed, completable template (placeholder name,
// signature, and body) rather than invalid SQL.
func TestBuildCreateStorageLifecyclePolicySqlPlaceholders(t *testing.T) {
	got, err := BuildCreateStorageLifecyclePolicySql("DB", "SC", StorageLifecyclePolicyConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"storage_lifecycle_policy_name", "(val TIMESTAMP_NTZ) RETURNS BOOLEAN ->", "TRUE"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder output to contain %q, got:\n%s", want, got)
		}
	}
}
