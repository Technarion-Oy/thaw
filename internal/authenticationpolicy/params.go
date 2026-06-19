// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package authenticationpolicy

import "thaw/internal/snowflake"

// ListParamMeta describes one of the policy's top-level list parameters for the
// properties editor: its ALTER keyword, the field label, the fixed enumeration of
// values offered (nil for a free-form list), and whether the editor accepts
// arbitrary entries on top of those options. The allowed-value sets are part of
// the CREATE AUTHENTICATION POLICY grammar, so they live here next to the builders
// rather than being duplicated in the frontend.
type ListParamMeta struct {
	Keyword  string   `json:"keyword"`
	Label    string   `json:"label"`
	Options  []string `json:"options"`
	Freeform bool     `json:"freeform"`
}

// ListParams returns the metadata for the three list parameters the properties
// modal renders. AUTHENTICATION_METHODS and CLIENT_TYPES are fixed enumerations;
// SECURITY_INTEGRATIONS is free-form (integration names) plus the ALL token.
func ListParams() []ListParamMeta {
	return []ListParamMeta{
		{
			Keyword: "AUTHENTICATION_METHODS",
			Label:   "Authentication methods",
			Options: []string{"ALL", "SAML", "PASSWORD", "OAUTH", "KEYPAIR", "PROGRAMMATIC_ACCESS_TOKEN", "WORKLOAD_IDENTITY"},
		},
		{
			Keyword: "CLIENT_TYPES",
			Label:   "Client types",
			Options: []string{"ALL", "SNOWFLAKE_UI", "DRIVERS", "SNOWFLAKE_CLI", "SNOWSQL"},
		},
		{
			Keyword:  "SECURITY_INTEGRATIONS",
			Label:    "Security integrations",
			Options:  []string{"ALL"},
			Freeform: true,
		},
	}
}

// MFAEnrollmentOptions returns the allowed values for the MFA_ENROLLMENT scalar
// parameter. The Snowflake default is OPTIONAL.
func MFAEnrollmentOptions() []string {
	return []string{"REQUIRED", "REQUIRED_PASSWORD_ONLY", "OPTIONAL"}
}

// ClientPolicyDrivers returns the driver/client tokens selectable in a
// CLIENT_POLICY bag — the version-governed subset of the general
// snowflake.ClientDrivers catalog. CLI/interactive clients (governed via
// CLIENT_TYPES, not a minimum version) are filtered out as inapplicable here.
func ClientPolicyDrivers() []string {
	var out []string
	for _, d := range snowflake.ClientDrivers() {
		if d.VersionGoverned {
			out = append(out, d.Token)
		}
	}
	return out
}
