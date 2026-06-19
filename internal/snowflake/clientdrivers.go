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
}

// ClientDrivers returns the canonical catalog of Snowflake client/driver tokens.
// It is the single general source for these identifiers; feature-specific call
// sites filter it for their use case rather than hard-coding their own copy.
func ClientDrivers() []ClientDriver {
	return []ClientDriver{
		{Token: "JDBC_DRIVER", VersionGoverned: true},
		{Token: "ODBC_DRIVER", VersionGoverned: true},
		{Token: "PYTHON_DRIVER", VersionGoverned: true},
		{Token: "JAVASCRIPT_DRIVER", VersionGoverned: true},
		{Token: "C_DRIVER", VersionGoverned: true},
		{Token: "GO_DRIVER", VersionGoverned: true},
		{Token: "PHP_DRIVER", VersionGoverned: true},
		{Token: "DOTNET_DRIVER", VersionGoverned: true},
		{Token: "SQL_API", VersionGoverned: true},
		{Token: "SNOWPIPE_STREAMING_CLIENT_SDK", VersionGoverned: true},
		{Token: "PY_CORE", VersionGoverned: true},
		{Token: "SPROC_PYTHON", VersionGoverned: true},
		{Token: "PYTHON_SNOWPARK", VersionGoverned: true},
		{Token: "SQL_ALCHEMY", VersionGoverned: true},
		{Token: "SNOWPARK", VersionGoverned: true},
		{Token: "SNOWFLAKE_CLIENT", VersionGoverned: true},
		// Interactive / CLI clients — real Snowflake clients, but governed via
		// CLIENT_TYPES rather than a CLIENT_POLICY minimum version.
		{Token: "SNOWSQL", VersionGoverned: false},
		{Token: "SNOWFLAKE_CLI", VersionGoverned: false},
	}
}
