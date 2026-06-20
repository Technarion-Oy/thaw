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

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// AuthenticationPolicyConfig holds the parameters for creating a Snowflake
// AUTHENTICATION POLICY object. Every parameter is optional in the CREATE
// grammar; an unspecified parameter inherits Snowflake's documented default
// (the list parameters default to ALL, MFA_ENROLLMENT defaults to OPTIONAL), so
// the builder emits only the fields the caller has explicitly set. The
// list-valued parameters are []string slices of bare tokens (e.g. "PASSWORD",
// "SAML", or a security-integration name) which the builder renders as
// single-quoted string literals; MFA_ENROLLMENT is a single enumerated keyword.
type AuthenticationPolicyConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`

	// AuthenticationMethods sets AUTHENTICATION_METHODS — the login methods the
	// policy permits. Allowed tokens: ALL, SAML, PASSWORD, OAUTH, KEYPAIR,
	// PROGRAMMATIC_ACCESS_TOKEN, WORKLOAD_IDENTITY. Default: ('ALL').
	AuthenticationMethods []string `json:"authenticationMethods"`
	// ClientTypes sets CLIENT_TYPES — the client kinds that may connect. Allowed
	// tokens: ALL, SNOWFLAKE_UI, DRIVERS, SNOWFLAKE_CLI, SNOWSQL. Default: ('ALL').
	ClientTypes []string `json:"clientTypes"`
	// SecurityIntegrations sets SECURITY_INTEGRATIONS — the security integrations
	// (or the special token ALL) usable for SAML/OAuth login. Default: ('ALL').
	SecurityIntegrations []string `json:"securityIntegrations"`
	// MFAEnrollment sets MFA_ENROLLMENT — one of REQUIRED,
	// REQUIRED_PASSWORD_ONLY, OPTIONAL. Empty leaves it at the default (OPTIONAL).
	MFAEnrollment string `json:"mfaEnrollment"`

	// The four nested property bags. Each is serialized through its own
	// Build<Bag>Value serializer; an empty bag builds to "()" and is omitted, so
	// the CREATE inherits the Snowflake default just like the list parameters.
	MFAPolicy              MFAPolicy              `json:"mfaPolicy"`
	PATPolicy              PATPolicy              `json:"patPolicy"`
	WorkloadIdentityPolicy WorkloadIdentityPolicy `json:"workloadIdentityPolicy"`
	ClientPolicy           ClientPolicy           `json:"clientPolicy"`

	Comment string `json:"comment"`
}

// FormatStringList renders a token slice into the SQL list grammar used by the
// authentication-policy list parameters — each token becomes a single-quoted
// string literal, e.g. []string{"PASSWORD","SAML"} → "('PASSWORD', 'SAML')".
// Blank tokens are skipped. It delegates to the shared snowflake.FormatStringLitList
// so the CREATE builder, the nested property-bag serializers, and the properties
// modal (which reaches this over IPC via App.FormatAuthPolicyList) serialize
// lists through one implementation. Pure string handling — no connection required.
func FormatStringList(tokens []string) string { return snowflake.FormatStringLitList(tokens) }

// BuildCreateAuthenticationPolicySql constructs a CREATE AUTHENTICATION POLICY
// statement from the given config. Only parameters the caller explicitly set
// (non-empty lists / MFA enrollment / comment) are emitted; the rest inherit
// Snowflake's documented defaults. When the name is blank the builder
// substitutes a placeholder so the live preview reads as a completable template
// rather than invalid SQL.
//
//	CREATE [OR REPLACE] AUTHENTICATION POLICY [IF NOT EXISTS] <fqn>
//	  [AUTHENTICATION_METHODS = (…)]
//	  [CLIENT_TYPES = (…)]
//	  [SECURITY_INTEGRATIONS = (…)]
//	  [MFA_ENROLLMENT = {REQUIRED | REQUIRED_PASSWORD_ONLY | OPTIONAL}]
//	  [MFA_POLICY = (…)] [PAT_POLICY = (…)]
//	  [WORKLOAD_IDENTITY_POLICY = (…)] [CLIENT_POLICY = (…)]
//	  [COMMENT = '…'];
func BuildCreateAuthenticationPolicySql(db, schema string, cfg AuthenticationPolicyConfig) (string, error) {
	var sb strings.Builder

	createClause := "CREATE"
	if cfg.OrReplace {
		createClause += " OR REPLACE"
	}
	createClause += " AUTHENTICATION POLICY"
	// OR REPLACE and IF NOT EXISTS are mutually exclusive; OR REPLACE wins.
	if cfg.IfNotExists && !cfg.OrReplace {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "authentication_policy_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", createClause,
		snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	if snowflake.HasNonBlankToken(cfg.AuthenticationMethods) {
		fmt.Fprintf(&sb, "\n  AUTHENTICATION_METHODS = %s", snowflake.FormatStringLitList(cfg.AuthenticationMethods))
	}
	if snowflake.HasNonBlankToken(cfg.ClientTypes) {
		fmt.Fprintf(&sb, "\n  CLIENT_TYPES = %s", snowflake.FormatStringLitList(cfg.ClientTypes))
	}
	if snowflake.HasNonBlankToken(cfg.SecurityIntegrations) {
		fmt.Fprintf(&sb, "\n  SECURITY_INTEGRATIONS = %s", snowflake.FormatStringLitList(cfg.SecurityIntegrations))
	}
	// MFA_ENROLLMENT is interpolated bare (an enum keyword), so it gets the same
	// isBareToken guard as the other bare tokens in this package — a value with
	// ')'/';'/whitespace from an IPC caller is dropped rather than emitted.
	if v := strings.ToUpper(strings.TrimSpace(cfg.MFAEnrollment)); v != "" && isBareToken(v) {
		fmt.Fprintf(&sb, "\n  MFA_ENROLLMENT = %s", v)
	}

	// Nested property bags — emit each only when it serializes to a non-empty
	// value (an unset bag builds to "()", which Snowflake rejects, so it's omitted
	// and the policy inherits the default). Serialization reuses the same
	// Build<Bag>Value functions the ALTER path uses.
	for _, bag := range []struct{ keyword, value string }{
		{"MFA_POLICY", BuildMFAPolicyValue(cfg.MFAPolicy)},
		{"PAT_POLICY", BuildPATPolicyValue(cfg.PATPolicy)},
		{"WORKLOAD_IDENTITY_POLICY", BuildWorkloadIdentityPolicyValue(cfg.WorkloadIdentityPolicy)},
		{"CLIENT_POLICY", BuildClientPolicyValue(cfg.ClientPolicy)},
	} {
		if bag.value != "()" {
			fmt.Fprintf(&sb, "\n  %s = %s", bag.keyword, bag.value)
		}
	}

	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", snowflake.EscapeTextLit(cfg.Comment))
	}

	return sb.String() + ";", nil
}
