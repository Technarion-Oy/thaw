// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package externalfunction

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildCreateExternalFunctionSql_Minimal(t *testing.T) {
	sql, err := BuildCreateExternalFunctionSql("DB", "SC", ExternalFunctionConfig{
		Name:           "MY_FN",
		Returns:        "VARIANT",
		ApiIntegration: "MY_API",
		Url:            "https://example.com/echo",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `CREATE EXTERNAL FUNCTION "DB"."SC".MY_FN()` + "\n" +
		`  RETURNS VARIANT` + "\n" +
		`  API_INTEGRATION = MY_API` + "\n" +
		`  AS 'https://example.com/echo';`
	if sql != want {
		t.Errorf("minimal mismatch:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildCreateExternalFunctionSql_Full(t *testing.T) {
	sql, err := BuildCreateExternalFunctionSql("DB", "SC", ExternalFunctionConfig{
		Name:      "ADD_ONE",
		OrReplace: true,
		Secure:    true,
		Args: []ExternalFunctionArg{
			{Name: "x", Type: "NUMBER"},
			{Name: "y", Type: "VARCHAR"},
		},
		Returns:            "NUMBER",
		NotNull:            true,
		NullHandling:       "STRICT",
		Volatility:         "IMMUTABLE",
		Comment:            "calls lambda",
		ApiIntegration:     "MY_API",
		Headers:            []HeaderPair{{Name: "x-env", Value: "prod"}},
		ContextHeaders:     []string{"current_timestamp", "current_user"},
		MaxBatchRows:       "100",
		Compression:        "gzip",
		RequestTranslator:  "DB.SC.REQ",
		ResponseTranslator: "DB.SC.RES",
		Url:                "https://example.com/add",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := []string{
		`CREATE OR REPLACE SECURE EXTERNAL FUNCTION "DB"."SC".ADD_ONE(x NUMBER, y VARCHAR)`,
		"\n  RETURNS NUMBER NOT NULL",
		"\n  STRICT",
		"\n  IMMUTABLE",
		"\n  COMMENT = 'calls lambda'",
		"\n  API_INTEGRATION = MY_API",
		"\n  HEADERS = ('x-env' = 'prod')",
		"\n  CONTEXT_HEADERS = (current_timestamp, current_user)",
		"\n  MAX_BATCH_ROWS = 100",
		"\n  COMPRESSION = GZIP",
		"\n  REQUEST_TRANSLATOR = DB.SC.REQ",
		"\n  RESPONSE_TRANSLATOR = DB.SC.RES",
		"\n  AS 'https://example.com/add';",
	}
	for _, c := range checks {
		if !strings.Contains(sql, c) {
			t.Errorf("expected SQL to contain %q\nfull SQL:\n%s", c, sql)
		}
	}
}

func TestBuildCreateExternalFunctionSql_Placeholders(t *testing.T) {
	// Empty required fields fall back to placeholders so the preview stays
	// readable rather than producing broken SQL.
	sql, err := BuildCreateExternalFunctionSql("DB", "SC", ExternalFunctionConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range []string{
		"external_function_name()",
		"RETURNS VARIANT",
		"API_INTEGRATION = <api_integration>",
		"AS '<url_of_proxy_and_resource>'",
	} {
		if !strings.Contains(sql, c) {
			t.Errorf("expected placeholder %q in:\n%s", c, sql)
		}
	}
}

func TestBuildCreateExternalFunctionSql_CaseSensitiveName(t *testing.T) {
	sql, _ := BuildCreateExternalFunctionSql("DB", "SC", ExternalFunctionConfig{
		Name:           "myFn",
		CaseSensitive:  true,
		Returns:        "VARIANT",
		ApiIntegration: "API",
		Url:            "https://x",
	})
	if !strings.Contains(sql, `"myFn"`) {
		t.Errorf("expected quoted case-sensitive name in:\n%s", sql)
	}
}

func TestBuildCreateExternalFunctionSql_EscapesLiterals(t *testing.T) {
	sql, _ := BuildCreateExternalFunctionSql("DB", "SC", ExternalFunctionConfig{
		Name:           "FN",
		Returns:        "VARIANT",
		Comment:        "it's external",
		ApiIntegration: "API",
		Url:            "https://x?q='a'",
		Headers:        []HeaderPair{{Name: "k", Value: "v'v"}},
	})
	if !strings.Contains(sql, "COMMENT = 'it''s external'") {
		t.Errorf("comment not escaped:\n%s", sql)
	}
	if !strings.Contains(sql, "AS 'https://x?q=''a'''") {
		t.Errorf("url not escaped:\n%s", sql)
	}
	if !strings.Contains(sql, "'k' = 'v''v'") {
		t.Errorf("header value not escaped:\n%s", sql)
	}
	// Sanity: the escaping helpers used here are the shared snowflake ones.
	if snowflake.EscapeStringLit("a'b") != "a''b" {
		t.Fatalf("unexpected EscapeStringLit behavior")
	}
}

func TestBuildArgList_SkipsBlankNames(t *testing.T) {
	got := buildArgList([]ExternalFunctionArg{
		{Name: "a", Type: "INT"},
		{Name: "  ", Type: "INT"},
		{Name: "b", Type: ""},
	})
	if got != "a INT, b VARIANT" {
		t.Errorf("buildArgList = %q", got)
	}
}
