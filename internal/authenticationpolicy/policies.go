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
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"thaw/internal/snowflake"
)

// This file models the four nested "property-bag" parameters of an
// authentication policy — MFA_POLICY, PAT_POLICY, WORKLOAD_IDENTITY_POLICY, and
// CLIENT_POLICY. Each is a parenthesized list of sub-properties with its own
// grammar. The frontend collects the values structurally (selects / numbers /
// toggles) but defers BOTH directions of the conversion to these builders /
// parsers (exposed over IPC via App.Build*Value / App.Parse*) so no SQL
// serialization or DESCRIBE parsing logic lives in TypeScript.
//
// Builders emit the value that follows `=` in an ALTER … SET <BAG> = <value>
// clause: a parenthesized, space-delimited property list (CLIENT_POLICY uses a
// comma-delimited entry list). Only sub-properties the caller set are emitted.
//
// Parsers read the value DESCRIBE AUTHENTICATION POLICY reports for a bag back
// into the struct so the editor can pre-fill. DESCRIBE renders these bags as
// JSON objects, so the parsers are JSON-driven and tolerant — an unrecognized /
// empty value yields a zero struct (the editor simply starts blank) rather than
// an error.

// ── MFA_POLICY ───────────────────────────────────────────────────────────────

// MFAPolicy models the MFA_POLICY bag: which MFA methods are allowed and whether
// MFA is enforced for externally-authenticated logins.
type MFAPolicy struct {
	// ALLOWED_METHODS — any of ALL, PASSKEY, TOTP, OTP, DUO (quoted literals).
	AllowedMethods []string `json:"allowedMethods"`
	// ENFORCE_MFA_ON_EXTERNAL_AUTHENTICATION — ALL or NONE (quoted literal).
	EnforceMFAOnExternalAuthentication string `json:"enforceMfaOnExternalAuthentication"`
}

// BuildMFAPolicyValue serializes p into the `( … )` value for SET MFA_POLICY.
func BuildMFAPolicyValue(p MFAPolicy) string {
	var props []string
	if hasToken(p.AllowedMethods) {
		props = append(props, "ALLOWED_METHODS = "+formatStringList(p.AllowedMethods))
	}
	if v := strings.TrimSpace(p.EnforceMFAOnExternalAuthentication); v != "" {
		props = append(props, "ENFORCE_MFA_ON_EXTERNAL_AUTHENTICATION = '"+snowflake.EscapeTextLit(strings.ToUpper(v))+"'")
	}
	return wrapProps(props)
}

// ParseMFAPolicy reads a DESCRIBE MFA_POLICY value back into the struct.
func ParseMFAPolicy(raw string) MFAPolicy {
	m := parseJSONObject(raw)
	return MFAPolicy{
		AllowedMethods:                     jsonStringList(m, "ALLOWED_METHODS"),
		EnforceMFAOnExternalAuthentication: jsonString(m, "ENFORCE_MFA_ON_EXTERNAL_AUTHENTICATION"),
	}
}

// ── PAT_POLICY ───────────────────────────────────────────────────────────────

// PATPolicy models the PAT_POLICY bag governing programmatic access tokens.
// The two day counts are *int and the boolean a *bool so the builder can tell
// "leave unset" (nil) apart from a deliberate value (including 0 / false).
type PATPolicy struct {
	DefaultExpiryInDays                   *int   `json:"defaultExpiryInDays"`                   // DEFAULT_EXPIRY_IN_DAYS
	MaxExpiryInDays                       *int   `json:"maxExpiryInDays"`                       // MAX_EXPIRY_IN_DAYS
	NetworkPolicyEvaluation               string `json:"networkPolicyEvaluation"`               // ENFORCED_REQUIRED | ENFORCED_NOT_REQUIRED | NOT_ENFORCED
	RequireRoleRestrictionForServiceUsers *bool  `json:"requireRoleRestrictionForServiceUsers"` // TRUE | FALSE
}

// BuildPATPolicyValue serializes p into the `( … )` value for SET PAT_POLICY.
func BuildPATPolicyValue(p PATPolicy) string {
	var props []string
	if p.DefaultExpiryInDays != nil {
		props = append(props, fmt.Sprintf("DEFAULT_EXPIRY_IN_DAYS = %d", *p.DefaultExpiryInDays))
	}
	if p.MaxExpiryInDays != nil {
		props = append(props, fmt.Sprintf("MAX_EXPIRY_IN_DAYS = %d", *p.MaxExpiryInDays))
	}
	if v := strings.TrimSpace(p.NetworkPolicyEvaluation); v != "" {
		props = append(props, "NETWORK_POLICY_EVALUATION = "+strings.ToUpper(v))
	}
	if p.RequireRoleRestrictionForServiceUsers != nil {
		b := "FALSE"
		if *p.RequireRoleRestrictionForServiceUsers {
			b = "TRUE"
		}
		props = append(props, "REQUIRE_ROLE_RESTRICTION_FOR_SERVICE_USERS = "+b)
	}
	return wrapProps(props)
}

// ParsePATPolicy reads a DESCRIBE PAT_POLICY value back into the struct.
func ParsePATPolicy(raw string) PATPolicy {
	m := parseJSONObject(raw)
	return PATPolicy{
		DefaultExpiryInDays:                   jsonIntPtr(m, "DEFAULT_EXPIRY_IN_DAYS"),
		MaxExpiryInDays:                       jsonIntPtr(m, "MAX_EXPIRY_IN_DAYS"),
		NetworkPolicyEvaluation:               jsonString(m, "NETWORK_POLICY_EVALUATION"),
		RequireRoleRestrictionForServiceUsers: jsonBoolPtr(m, "REQUIRE_ROLE_RESTRICTION_FOR_SERVICE_USERS"),
	}
}

// ── WORKLOAD_IDENTITY_POLICY ─────────────────────────────────────────────────

// WorkloadIdentityPolicy models the WORKLOAD_IDENTITY_POLICY bag. The providers
// are bare keywords (ALL/AWS/AZURE/GCP/OIDC); the account/issuer lists are
// quoted string literals.
type WorkloadIdentityPolicy struct {
	AllowedProviders    []string `json:"allowedProviders"`    // ALL | AWS | AZURE | GCP | OIDC (bare)
	AllowedAWSAccounts  []string `json:"allowedAwsAccounts"`  // 12-digit account IDs (quoted)
	AllowedAzureIssuers []string `json:"allowedAzureIssuers"` // authority URLs (quoted)
	AllowedOIDCIssuers  []string `json:"allowedOidcIssuers"`  // HTTPS URLs (quoted)
}

// BuildWorkloadIdentityPolicyValue serializes p into the `( … )` value for
// SET WORKLOAD_IDENTITY_POLICY.
func BuildWorkloadIdentityPolicyValue(p WorkloadIdentityPolicy) string {
	var props []string
	if hasToken(p.AllowedProviders) {
		props = append(props, "ALLOWED_PROVIDERS = "+formatBareList(p.AllowedProviders))
	}
	if hasToken(p.AllowedAWSAccounts) {
		props = append(props, "ALLOWED_AWS_ACCOUNTS = "+formatStringList(p.AllowedAWSAccounts))
	}
	if hasToken(p.AllowedAzureIssuers) {
		props = append(props, "ALLOWED_AZURE_ISSUERS = "+formatStringList(p.AllowedAzureIssuers))
	}
	if hasToken(p.AllowedOIDCIssuers) {
		props = append(props, "ALLOWED_OIDC_ISSUERS = "+formatStringList(p.AllowedOIDCIssuers))
	}
	return wrapProps(props)
}

// ParseWorkloadIdentityPolicy reads a DESCRIBE WORKLOAD_IDENTITY_POLICY value
// back into the struct.
func ParseWorkloadIdentityPolicy(raw string) WorkloadIdentityPolicy {
	m := parseJSONObject(raw)
	return WorkloadIdentityPolicy{
		AllowedProviders:    jsonStringList(m, "ALLOWED_PROVIDERS"),
		AllowedAWSAccounts:  jsonStringList(m, "ALLOWED_AWS_ACCOUNTS"),
		AllowedAzureIssuers: jsonStringList(m, "ALLOWED_AZURE_ISSUERS"),
		AllowedOIDCIssuers:  jsonStringList(m, "ALLOWED_OIDC_ISSUERS"),
	}
}

// ── CLIENT_POLICY ────────────────────────────────────────────────────────────

// ClientPolicyEntry pins a single driver/client to a minimum version.
type ClientPolicyEntry struct {
	Driver         string `json:"driver"`         // e.g. JDBC_DRIVER
	MinimumVersion string `json:"minimumVersion"` // e.g. 3.13.0
}

// ClientPolicy models the CLIENT_POLICY bag: a comma-delimited list of
// `<driver> = ( MINIMUM_VERSION = '<version>' )` entries.
type ClientPolicy struct {
	Entries []ClientPolicyEntry `json:"entries"`
}

// BuildClientPolicyValue serializes p into the `( … )` value for
// SET CLIENT_POLICY. Entries missing a driver or version are skipped.
func BuildClientPolicyValue(p ClientPolicy) string {
	var entries []string
	for _, e := range p.Entries {
		d := strings.ToUpper(strings.TrimSpace(e.Driver))
		v := strings.TrimSpace(e.MinimumVersion)
		if d == "" || v == "" {
			continue
		}
		entries = append(entries, fmt.Sprintf("%s = ( MINIMUM_VERSION = '%s' )", d, snowflake.EscapeTextLit(v)))
	}
	if len(entries) == 0 {
		return "()"
	}
	return "( " + strings.Join(entries, ", ") + " )"
}

// ParseClientPolicy reads a DESCRIBE CLIENT_POLICY value (a JSON object keyed by
// driver name, each holding a MINIMUM_VERSION) back into the struct. Entries are
// sorted by driver so the editor renders deterministically.
func ParseClientPolicy(raw string) ClientPolicy {
	m := parseJSONObject(raw)
	var cp ClientPolicy
	for driver, v := range m {
		obj, ok := v.(map[string]any)
		if !ok {
			continue
		}
		var ver string
		for ik, iv := range obj {
			if strings.EqualFold(ik, "MINIMUM_VERSION") {
				if s, ok := iv.(string); ok {
					ver = s
				}
			}
		}
		if ver != "" {
			cp.Entries = append(cp.Entries, ClientPolicyEntry{Driver: driver, MinimumVersion: ver})
		}
	}
	sort.Slice(cp.Entries, func(i, j int) bool { return cp.Entries[i].Driver < cp.Entries[j].Driver })
	return cp
}

// ── shared helpers ───────────────────────────────────────────────────────────

// wrapProps wraps a space-delimited property list in parentheses (or returns the
// empty list `()` when nothing was set).
func wrapProps(props []string) string {
	if len(props) == 0 {
		return "()"
	}
	return "( " + strings.Join(props, " ") + " )"
}

// formatBareList renders a token slice into a parenthesized bare-keyword list,
// e.g. []string{"AWS","AZURE"} → "(AWS, AZURE)". Blank tokens are skipped.
func formatBareList(tokens []string) string {
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		parts = append(parts, strings.ToUpper(t))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// parseJSONObject parses raw as a JSON object, normalizing top-level keys to
// upper-case. Returns nil on empty / non-object input (reads from a nil map
// return zero values, so callers need no special-casing).
func parseJSONObject(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[strings.ToUpper(k)] = v
	}
	return out
}

func jsonString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func jsonStringList(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		// Tolerate a scalar rendering (some editions emit a single value bare).
		if strings.TrimSpace(t) != "" {
			return []string{t}
		}
	}
	return nil
}

func jsonIntPtr(m map[string]any, key string) *int {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch n := v.(type) {
	case float64:
		i := int(n)
		return &i
	case string:
		if iv, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return &iv
		}
	}
	return nil
}

func jsonBoolPtr(m map[string]any, key string) *bool {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch b := v.(type) {
	case bool:
		return &b
	case string:
		switch strings.ToUpper(strings.TrimSpace(b)) {
		case "TRUE":
			t := true
			return &t
		case "FALSE":
			f := false
			return &f
		}
	}
	return nil
}
