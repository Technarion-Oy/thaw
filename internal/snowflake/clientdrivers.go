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
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ClientDriver describes a Snowflake client/driver by its canonical token — the
// identifier Snowflake uses for it across CLIENT_TYPES, CLIENT_POLICY, and the
// client-version reporting views.
type ClientDriver struct {
	// Token is the canonical identifier, e.g. "JDBC_DRIVER".
	Token string
	// VersionGoverned reports whether this client supports per-driver minimum
	// version enforcement — the granular programmatic drivers, connectors, and
	// SDKs. Command-line / interactive clients (SnowSQL, the Snowflake CLI) are
	// governed at the coarser CLIENT_TYPES level instead and cannot have a minimum
	// version pinned, so they are excluded by callers that need the enforceable
	// set (e.g. authenticationpolicy.ClientPolicyDrivers).
	VersionGoverned bool
	// VersionInfoAliases are the SYSTEM$CLIENT_VERSION_INFO() clientId / clientAppId
	// values that identify this driver, so its supported/recommended versions can
	// be matched back to the token (the function names clients differently than the
	// policy grammar). Matching is case/format-insensitive, and multiple aliases
	// are tolerated; an empty list (or no match) simply yields no version hint.
	VersionInfoAliases []string
}

// ClientDrivers returns the canonical catalog of Snowflake client/driver tokens.
// It is the single general source for these identifiers; feature-specific call
// sites filter it for their use case rather than hard-coding their own copy.
func ClientDrivers() []ClientDriver {
	return []ClientDriver{
		{Token: "JDBC_DRIVER", VersionGoverned: true, VersionInfoAliases: []string{"JDBC"}},
		{Token: "ODBC_DRIVER", VersionGoverned: true, VersionInfoAliases: []string{"ODBC"}},
		{Token: "PYTHON_DRIVER", VersionGoverned: true, VersionInfoAliases: []string{"PythonConnector", "Python Connector", "Python"}},
		{Token: "JAVASCRIPT_DRIVER", VersionGoverned: true, VersionInfoAliases: []string{"JavaScript", "JSDriver", "NodejsDriver", "Nodejs", "Node.js"}},
		{Token: "C_DRIVER", VersionGoverned: true, VersionInfoAliases: []string{"C API", "CAPI"}},
		{Token: "GO_DRIVER", VersionGoverned: true, VersionInfoAliases: []string{"Go", "GoDriver", "Golang"}},
		{Token: "PHP_DRIVER", VersionGoverned: true, VersionInfoAliases: []string{"PDO", "PHP_PDO", "PHP", "PHPPDODriver"}},
		{Token: "DOTNET_DRIVER", VersionGoverned: true, VersionInfoAliases: []string{".NET", "DOTNETDriver", "DotNet"}},
		{Token: "SQL_API", VersionGoverned: true, VersionInfoAliases: []string{"SQLAPI", "SQL API"}},
		{Token: "SNOWPIPE_STREAMING_CLIENT_SDK", VersionGoverned: true},
		{Token: "PY_CORE", VersionGoverned: true, VersionInfoAliases: []string{"PyCore"}},
		{Token: "SPROC_PYTHON", VersionGoverned: true},
		{Token: "PYTHON_SNOWPARK", VersionGoverned: true, VersionInfoAliases: []string{"PythonSnowpark"}},
		{Token: "SQL_ALCHEMY", VersionGoverned: true, VersionInfoAliases: []string{"SnowflakeSQLAlchemy", "SQLAlchemy"}},
		{Token: "SNOWPARK", VersionGoverned: true, VersionInfoAliases: []string{"Snowpark"}},
		{Token: "SNOWFLAKE_CLIENT", VersionGoverned: true},
		// Interactive / CLI clients — real Snowflake clients, but governed via
		// CLIENT_TYPES rather than a CLIENT_POLICY minimum version.
		{Token: "SNOWSQL", VersionGoverned: false, VersionInfoAliases: []string{"SnowSQL"}},
		{Token: "SNOWFLAKE_CLI", VersionGoverned: false, VersionInfoAliases: []string{"Snowflake CLI", "SnowflakeCLI", "snowcli"}},
	}
}

// ClientVersionInfo is one entry from SYSTEM$CLIENT_VERSION_INFO(), describing the
// supported and recommended versions of a single Snowflake client/driver.
type ClientVersionInfo struct {
	ClientID                          string   `json:"clientId"`
	ClientAppID                       string   `json:"clientAppId"`
	MinimumSupportedVersion           string   `json:"minimumSupportedVersion"`
	MinimumNearingEndOfSupportVersion string   `json:"minimumNearingEndOfSupportVersion"`
	RecommendedVersion                string   `json:"recommendedVersion"`
	DeprecatedVersions                []string `json:"deprecatedVersions"`
}

// GetClientVersionInfo runs SELECT SYSTEM$CLIENT_VERSION_INFO() and parses its
// JSON-array result into structured per-client version info. It is general —
// reusable wherever Snowflake's supported/recommended client versions are needed
// (e.g. the authentication-policy CLIENT_POLICY editor's version suggestions).
// A NULL / empty result yields a nil slice and no error.
func (c *Client) GetClientVersionInfo(ctx context.Context) ([]ClientVersionInfo, error) {
	var raw string
	if err := c.queryRowCtx(ctx, "SELECT SYSTEM$CLIENT_VERSION_INFO()").Scan(&raw); err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var info []ClientVersionInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, fmt.Errorf("parse SYSTEM$CLIENT_VERSION_INFO output: %w", err)
	}
	return info, nil
}

// MatchClientVersions maps catalog driver tokens to their SYSTEM$CLIENT_VERSION_INFO
// entry, matching each token's aliases against the reported clientId / clientAppId
// (case- and format-insensitive). Tokens the function doesn't report are omitted.
// It is the general join between the driver catalog and live version info.
func MatchClientVersions(info []ClientVersionInfo) map[string]ClientVersionInfo {
	byKey := make(map[string]ClientVersionInfo, len(info)*2)
	for _, e := range info {
		for _, id := range []string{e.ClientID, e.ClientAppID} {
			if k := normalizeClientKey(id); k != "" {
				if _, exists := byKey[k]; !exists {
					byKey[k] = e
				}
			}
		}
	}
	out := make(map[string]ClientVersionInfo)
	for _, d := range ClientDrivers() {
		for _, alias := range d.VersionInfoAliases {
			if e, ok := byKey[normalizeClientKey(alias)]; ok {
				out[d.Token] = e
				break
			}
		}
	}
	return out
}

// normalizeClientKey upper-cases s and drops every non-alphanumeric rune, so
// ".NET", "Python Connector", and "GoDriver" compare regardless of separators or
// casing (".NET" → "NET", "Python Connector" → "PYTHONCONNECTOR").
func normalizeClientKey(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r
		case r >= 'a' && r <= 'z':
			return r - ('a' - 'A')
		case r >= '0' && r <= '9':
			return r
		default:
			return -1
		}
	}, s)
}
