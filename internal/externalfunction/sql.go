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
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ExternalFunctionArg is a single function parameter: a name and a data type
// (e.g. {Name: "x", Type: "NUMBER"}). Arguments with a blank name are skipped by
// the builder.
type ExternalFunctionArg struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// HeaderPair is a single HTTP header forwarded to the remote service via the
// HEADERS clause: a name and a value, both rendered as single-quoted string
// literals. Pairs with a blank name are skipped.
type HeaderPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ExternalFunctionConfig holds the parameters for creating a Snowflake EXTERNAL
// FUNCTION. ApiIntegration and Url are required; every other field is optional —
// an empty/zero value means the corresponding clause is omitted and Snowflake's
// default applies.
type ExternalFunctionConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	Secure        bool   `json:"secure"`

	Args    []ExternalFunctionArg `json:"args"`    // ( <name> <type>, ... )
	Returns string                `json:"returns"` // RETURNS <result_data_type>
	NotNull bool                  `json:"notNull"` // RETURNS <type> NOT NULL

	// NullHandling is "" (default), "CALLED ON NULL INPUT", "RETURNS NULL ON NULL
	// INPUT", or "STRICT".
	NullHandling string `json:"nullHandling"`
	// Volatility is "" (default), "VOLATILE", or "IMMUTABLE".
	Volatility string `json:"volatility"`

	Comment string `json:"comment"`

	ApiIntegration     string       `json:"apiIntegration"`     // API_INTEGRATION = <integration> (required)
	Headers            []HeaderPair `json:"headers"`            // HEADERS = ( '<h>' = '<v>', ... )
	ContextHeaders     []string     `json:"contextHeaders"`     // CONTEXT_HEADERS = ( <context_fn>, ... )
	MaxBatchRows       string       `json:"maxBatchRows"`       // MAX_BATCH_ROWS = <int> (or "")
	Compression        string       `json:"compression"`        // COMPRESSION = { NONE | AUTO | GZIP | DEFLATE } (or "")
	RequestTranslator  string       `json:"requestTranslator"`  // REQUEST_TRANSLATOR = <udf> (or "")
	ResponseTranslator string       `json:"responseTranslator"` // RESPONSE_TRANSLATOR = <udf> (or "")
	Url                string       `json:"url"`                // AS '<url_of_proxy_and_resource>' (required)
}

// BuildCreateExternalFunctionSql constructs a CREATE EXTERNAL FUNCTION statement
// from the given config. The argument list, return type, API integration, and URL
// always appear (placeholders fill in for empty required fields so the live
// preview stays valid-looking); the remaining clauses are emitted only when set,
// in the order Snowflake documents them. OR REPLACE and SECURE are independent
// modifiers on the CREATE keyword.
func BuildCreateExternalFunctionSql(db, schema string, cfg ExternalFunctionConfig) (string, error) {
	var sb strings.Builder

	body := "EXTERNAL FUNCTION"
	if cfg.Secure {
		body = "SECURE " + body
	}
	createClause := snowflake.CreateClause(body, cfg.OrReplace, false)

	name := cfg.Name
	if name == "" {
		name = "external_function_name"
	}

	fmt.Fprintf(&sb, "%s %s(%s)", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive), buildArgList(cfg.Args))

	returns := strings.TrimSpace(cfg.Returns)
	if returns == "" {
		returns = "VARIANT"
	}
	fmt.Fprintf(&sb, "\n  RETURNS %s", returns)
	if cfg.NotNull {
		sb.WriteString(" NOT NULL")
	}

	if nh := strings.TrimSpace(cfg.NullHandling); nh != "" {
		fmt.Fprintf(&sb, "\n  %s", strings.ToUpper(nh))
	}
	if vol := strings.TrimSpace(cfg.Volatility); vol != "" {
		fmt.Fprintf(&sb, "\n  %s", strings.ToUpper(vol))
	}
	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	api := strings.TrimSpace(cfg.ApiIntegration)
	if api == "" {
		api = "<api_integration>"
	}
	fmt.Fprintf(&sb, "\n  API_INTEGRATION = %s", api)

	if hc := buildHeaders(cfg.Headers); hc != "" {
		fmt.Fprintf(&sb, "\n  HEADERS = (%s)", hc)
	}
	if ch := buildContextHeaders(cfg.ContextHeaders); ch != "" {
		fmt.Fprintf(&sb, "\n  CONTEXT_HEADERS = (%s)", ch)
	}
	if mbr := strings.TrimSpace(cfg.MaxBatchRows); mbr != "" {
		fmt.Fprintf(&sb, "\n  MAX_BATCH_ROWS = %s", mbr)
	}
	if comp := strings.TrimSpace(cfg.Compression); comp != "" {
		fmt.Fprintf(&sb, "\n  COMPRESSION = %s", strings.ToUpper(comp))
	}
	if rt := strings.TrimSpace(cfg.RequestTranslator); rt != "" {
		fmt.Fprintf(&sb, "\n  REQUEST_TRANSLATOR = %s", rt)
	}
	if rt := strings.TrimSpace(cfg.ResponseTranslator); rt != "" {
		fmt.Fprintf(&sb, "\n  RESPONSE_TRANSLATOR = %s", rt)
	}

	url := strings.TrimSpace(cfg.Url)
	if url == "" {
		url = "<url_of_proxy_and_resource>"
	}
	fmt.Fprintf(&sb, "\n  AS '%s'", snowflake.EscapeStringLit(url))

	return sb.String() + ";", nil
}

// buildArgList renders the comma-separated "<name> <type>" parameter list. Args
// with a blank name are skipped; a blank type falls back to a placeholder so the
// preview stays readable. Returns "" for a zero-argument function.
func buildArgList(args []ExternalFunctionArg) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			continue
		}
		typ := strings.TrimSpace(a.Type)
		if typ == "" {
			typ = "VARIANT"
		}
		parts = append(parts, fmt.Sprintf("%s %s", snowflake.QuoteOrBare(name, false), typ))
	}
	return strings.Join(parts, ", ")
}

// buildHeaders renders the inner "'<name>' = '<value>'" list for the HEADERS
// clause, skipping pairs with a blank name. Returns "" when none are set.
func buildHeaders(headers []HeaderPair) string {
	parts := make([]string, 0, len(headers))
	for _, h := range headers {
		name := strings.TrimSpace(h.Name)
		if name == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("'%s' = '%s'",
			snowflake.EscapeStringLit(name), snowflake.EscapeStringLit(h.Value)))
	}
	return strings.Join(parts, ", ")
}

// buildContextHeaders renders the inner bare-identifier list for the
// CONTEXT_HEADERS clause (e.g. current_timestamp), skipping blanks. Returns ""
// when none are set.
func buildContextHeaders(fns []string) string {
	parts := make([]string, 0, len(fns))
	for _, f := range fns {
		if v := strings.TrimSpace(f); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ", ")
}
