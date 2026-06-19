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
	"reflect"
	"testing"
)

func intp(v int) *int    { return &v }
func boolp(b bool) *bool { return &b }

func TestBuildMFAPolicyValue(t *testing.T) {
	got := BuildMFAPolicyValue(MFAPolicy{
		AllowedMethods:                     []string{"TOTP", "DUO"},
		EnforceMFAOnExternalAuthentication: "none",
	})
	want := "( ALLOWED_METHODS = ('TOTP', 'DUO') ENFORCE_MFA_ON_EXTERNAL_AUTHENTICATION = 'NONE' )"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if v := BuildMFAPolicyValue(MFAPolicy{}); v != "()" {
		t.Errorf("empty MFA policy = %q", v)
	}
}

func TestBuildPATPolicyValue(t *testing.T) {
	got := BuildPATPolicyValue(PATPolicy{
		DefaultExpiryInDays:                   intp(15),
		MaxExpiryInDays:                       intp(90),
		NetworkPolicyEvaluation:               "not_enforced",
		RequireRoleRestrictionForServiceUsers: boolp(false),
	})
	want := "( DEFAULT_EXPIRY_IN_DAYS = 15 MAX_EXPIRY_IN_DAYS = 90 NETWORK_POLICY_EVALUATION = NOT_ENFORCED REQUIRE_ROLE_RESTRICTION_FOR_SERVICE_USERS = FALSE )"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	// false / 0 must be distinguishable from unset.
	if v := BuildPATPolicyValue(PATPolicy{RequireRoleRestrictionForServiceUsers: boolp(true)}); v != "( REQUIRE_ROLE_RESTRICTION_FOR_SERVICE_USERS = TRUE )" {
		t.Errorf("bool-only = %q", v)
	}
	// Out-of-range expiry days (defense-in-depth) are dropped: 0, negative, and
	// >365 are all rejected; the documented bounds (1 and 365) pass.
	for _, days := range []int{0, -5, 366, 1000} {
		if v := BuildPATPolicyValue(PATPolicy{DefaultExpiryInDays: intp(days), MaxExpiryInDays: intp(days)}); v != "()" {
			t.Errorf("out-of-range %d expiry should be dropped, got %q", days, v)
		}
	}
	if v := BuildPATPolicyValue(PATPolicy{DefaultExpiryInDays: intp(1), MaxExpiryInDays: intp(365)}); v != "( DEFAULT_EXPIRY_IN_DAYS = 1 MAX_EXPIRY_IN_DAYS = 365 )" {
		t.Errorf("boundary days = %q", v)
	}
}

func TestBuildWorkloadIdentityPolicyValue(t *testing.T) {
	got := BuildWorkloadIdentityPolicyValue(WorkloadIdentityPolicy{
		AllowedProviders:   []string{"aws", "azure"},
		AllowedAWSAccounts: []string{"123456789012"},
		AllowedOIDCIssuers: []string{"https://issuer.example.com"},
	})
	want := "( ALLOWED_PROVIDERS = (AWS, AZURE) ALLOWED_AWS_ACCOUNTS = ('123456789012') ALLOWED_OIDC_ISSUERS = ('https://issuer.example.com') )"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestBuildClientPolicyValue(t *testing.T) {
	got := BuildClientPolicyValue(ClientPolicy{Entries: []ClientPolicyEntry{
		{Driver: "jdbc_driver", MinimumVersion: "3.13.0"},
		{Driver: "PYTHON_DRIVER", MinimumVersion: "3.0.0"},
		{Driver: "", MinimumVersion: "x"}, // skipped
	}})
	want := "( JDBC_DRIVER = ( MINIMUM_VERSION = '3.13.0' ), PYTHON_DRIVER = ( MINIMUM_VERSION = '3.0.0' ) )"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if v := BuildClientPolicyValue(ClientPolicy{}); v != "()" {
		t.Errorf("empty client policy = %q", v)
	}
	// A repeated driver (case-insensitive) is deduped first-wins so the bag never
	// has a duplicate key.
	dup := BuildClientPolicyValue(ClientPolicy{Entries: []ClientPolicyEntry{
		{Driver: "GO_DRIVER", MinimumVersion: "1.14.1"},
		{Driver: "go_driver", MinimumVersion: "9.9.9"},
	}})
	if dup != "( GO_DRIVER = ( MINIMUM_VERSION = '1.14.1' ) )" {
		t.Errorf("duplicate driver not deduped: %q", dup)
	}
}

func TestBuildBagsRejectBareTokenInjection(t *testing.T) {
	// Driver / provider / enum values are interpolated bare; anything that isn't
	// a plain identifier (e.g. contains ')' or ';') must be dropped, not emitted,
	// so an IPC caller can't break out of the bag's parentheses.
	if v := BuildClientPolicyValue(ClientPolicy{Entries: []ClientPolicyEntry{
		{Driver: "JDBC_DRIVER) ; DROP POLICY X --", MinimumVersion: "1.0"},
		{Driver: "GO_DRIVER", MinimumVersion: "1.14.1"},
	}}); v != "( GO_DRIVER = ( MINIMUM_VERSION = '1.14.1' ) )" {
		t.Errorf("client policy did not drop injected driver: %q", v)
	}
	if v := BuildWorkloadIdentityPolicyValue(WorkloadIdentityPolicy{
		AllowedProviders: []string{"AWS", "AZURE) UNSET COMMENT"},
	}); v != "( ALLOWED_PROVIDERS = (AWS) )" {
		t.Errorf("workload identity policy did not drop injected provider: %q", v)
	}
	if v := BuildPATPolicyValue(PATPolicy{NetworkPolicyEvaluation: "NOT_ENFORCED)"}); v != "()" {
		t.Errorf("PAT policy did not drop injected network-policy-evaluation: %q", v)
	}
}

func TestParseMFAPolicy(t *testing.T) {
	got := ParseMFAPolicy(`{"ALLOWED_METHODS":["TOTP","DUO"],"ENFORCE_MFA_ON_EXTERNAL_AUTHENTICATION":"NONE"}`)
	want := MFAPolicy{AllowedMethods: []string{"TOTP", "DUO"}, EnforceMFAOnExternalAuthentication: "NONE"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}
	// Garbage / empty → zero struct, no panic.
	if g := ParseMFAPolicy("not json"); g.AllowedMethods != nil || g.EnforceMFAOnExternalAuthentication != "" {
		t.Errorf("garbage parse = %+v", g)
	}
}

func TestParsePATPolicy(t *testing.T) {
	got := ParsePATPolicy(`{"default_expiry_in_days":15,"network_policy_evaluation":"ENFORCED_REQUIRED","require_role_restriction_for_service_users":true}`)
	if got.DefaultExpiryInDays == nil || *got.DefaultExpiryInDays != 15 {
		t.Errorf("default expiry = %v", got.DefaultExpiryInDays)
	}
	if got.NetworkPolicyEvaluation != "ENFORCED_REQUIRED" {
		t.Errorf("eval = %q", got.NetworkPolicyEvaluation)
	}
	if got.RequireRoleRestrictionForServiceUsers == nil || !*got.RequireRoleRestrictionForServiceUsers {
		t.Errorf("require role restriction = %v", got.RequireRoleRestrictionForServiceUsers)
	}
}

func TestParseClientPolicy(t *testing.T) {
	got := ParseClientPolicy(`{"PYTHON_DRIVER":{"MINIMUM_VERSION":"3.0.0"},"JDBC_DRIVER":{"MINIMUM_VERSION":"3.13.0"}}`)
	want := ClientPolicy{Entries: []ClientPolicyEntry{
		{Driver: "JDBC_DRIVER", MinimumVersion: "3.13.0"},
		{Driver: "PYTHON_DRIVER", MinimumVersion: "3.0.0"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}
}

func TestParseWorkloadIdentityPolicy(t *testing.T) {
	got := ParseWorkloadIdentityPolicy(`{"ALLOWED_PROVIDERS":["AWS"],"ALLOWED_AWS_ACCOUNTS":["123456789012"]}`)
	if !reflect.DeepEqual(got.AllowedProviders, []string{"AWS"}) {
		t.Errorf("providers = %v", got.AllowedProviders)
	}
	if !reflect.DeepEqual(got.AllowedAWSAccounts, []string{"123456789012"}) {
		t.Errorf("aws accounts = %v", got.AllowedAWSAccounts)
	}
}

// A strict-JSON rendering could emit a bare numeric AWS account; it must be
// coerced to its digits, not dropped or rendered in exponent form.
func TestParseWorkloadIdentityPolicyNumericAccount(t *testing.T) {
	got := ParseWorkloadIdentityPolicy(`{"ALLOWED_AWS_ACCOUNTS":[123456789012, "210987654321"]}`)
	if !reflect.DeepEqual(got.AllowedAWSAccounts, []string{"123456789012", "210987654321"}) {
		t.Errorf("aws accounts = %v", got.AllowedAWSAccounts)
	}
}

// DESCRIBE AUTHENTICATION POLICY renders the bags in Snowflake's structured-
// object notation (`{KEY=VALUE, KEY={NESTED=VALUE}}`), NOT JSON — e.g. the docs'
// `{GO_DRIVER={MINIMUM_VERSION=3.14.1}}`. These cover the parsers against that
// real rendering, the path JSON-only tests above never exercised.

func TestParseMFAPolicyStructured(t *testing.T) {
	got := ParseMFAPolicy(`{ALLOWED_METHODS=[TOTP, DUO], ENFORCE_MFA_ON_EXTERNAL_AUTHENTICATION=NONE}`)
	want := MFAPolicy{AllowedMethods: []string{"TOTP", "DUO"}, EnforceMFAOnExternalAuthentication: "NONE"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}
}

func TestParsePATPolicyStructured(t *testing.T) {
	got := ParsePATPolicy(`{DEFAULT_EXPIRY_IN_DAYS=15, NETWORK_POLICY_EVALUATION=ENFORCED_REQUIRED, REQUIRE_ROLE_RESTRICTION_FOR_SERVICE_USERS=FALSE}`)
	if got.DefaultExpiryInDays == nil || *got.DefaultExpiryInDays != 15 {
		t.Errorf("default expiry = %v", got.DefaultExpiryInDays)
	}
	if got.NetworkPolicyEvaluation != "ENFORCED_REQUIRED" {
		t.Errorf("eval = %q", got.NetworkPolicyEvaluation)
	}
	if got.RequireRoleRestrictionForServiceUsers == nil || *got.RequireRoleRestrictionForServiceUsers {
		t.Errorf("require role restriction = %v", got.RequireRoleRestrictionForServiceUsers)
	}
}

func TestParseClientPolicyStructured(t *testing.T) {
	// Exactly the rendering shown in the DESCRIBE reference, plus a second entry.
	got := ParseClientPolicy(`{GO_DRIVER={MINIMUM_VERSION=3.14.1}, JDBC_DRIVER={MINIMUM_VERSION=3.25.0}}`)
	want := ClientPolicy{Entries: []ClientPolicyEntry{
		{Driver: "GO_DRIVER", MinimumVersion: "3.14.1"},
		{Driver: "JDBC_DRIVER", MinimumVersion: "3.25.0"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}
}

func TestParseWorkloadIdentityPolicyStructured(t *testing.T) {
	got := ParseWorkloadIdentityPolicy(`{ALLOWED_PROVIDERS=[AWS, AZURE], ALLOWED_AWS_ACCOUNTS=[123456789012]}`)
	if !reflect.DeepEqual(got.AllowedProviders, []string{"AWS", "AZURE"}) {
		t.Errorf("providers = %v", got.AllowedProviders)
	}
	if !reflect.DeepEqual(got.AllowedAWSAccounts, []string{"123456789012"}) {
		t.Errorf("aws accounts = %v", got.AllowedAWSAccounts)
	}
}

// A quoted version string in the structured rendering must survive intact.
func TestParseClientPolicyStructuredQuoted(t *testing.T) {
	got := ParseClientPolicy(`{PYTHON_DRIVER={MINIMUM_VERSION='3.0.0'}}`)
	want := ClientPolicy{Entries: []ClientPolicyEntry{{Driver: "PYTHON_DRIVER", MinimumVersion: "3.0.0"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}
}

// If DESCRIBE ever returns the parenthesized SQL-grammar form instead of the
// brace form, the parsers must still populate the bags (otherwise a real,
// populated bag would render blank and a Set from it would wipe the policy).
// `( KEY = VALUE )` parses as an object; `('A', 'B')` as a value list.

func TestParseMFAPolicyParenForm(t *testing.T) {
	got := ParseMFAPolicy(`( ALLOWED_METHODS = ('TOTP', 'DUO') ENFORCE_MFA_ON_EXTERNAL_AUTHENTICATION = 'NONE' )`)
	want := MFAPolicy{AllowedMethods: []string{"TOTP", "DUO"}, EnforceMFAOnExternalAuthentication: "NONE"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}
}

func TestParsePATPolicyParenForm(t *testing.T) {
	got := ParsePATPolicy(`( DEFAULT_EXPIRY_IN_DAYS = 15 MAX_EXPIRY_IN_DAYS = 90 NETWORK_POLICY_EVALUATION = ENFORCED_REQUIRED REQUIRE_ROLE_RESTRICTION_FOR_SERVICE_USERS = TRUE )`)
	if got.DefaultExpiryInDays == nil || *got.DefaultExpiryInDays != 15 {
		t.Errorf("default expiry = %v", got.DefaultExpiryInDays)
	}
	if got.MaxExpiryInDays == nil || *got.MaxExpiryInDays != 90 {
		t.Errorf("max expiry = %v", got.MaxExpiryInDays)
	}
	if got.NetworkPolicyEvaluation != "ENFORCED_REQUIRED" {
		t.Errorf("eval = %q", got.NetworkPolicyEvaluation)
	}
	if got.RequireRoleRestrictionForServiceUsers == nil || !*got.RequireRoleRestrictionForServiceUsers {
		t.Errorf("require role restriction = %v", got.RequireRoleRestrictionForServiceUsers)
	}
}

func TestParseClientPolicyParenForm(t *testing.T) {
	got := ParseClientPolicy(`( GO_DRIVER = ( MINIMUM_VERSION = '3.14.1' ), JDBC_DRIVER = ( MINIMUM_VERSION = '3.25.0' ) )`)
	want := ClientPolicy{Entries: []ClientPolicyEntry{
		{Driver: "GO_DRIVER", MinimumVersion: "3.14.1"},
		{Driver: "JDBC_DRIVER", MinimumVersion: "3.25.0"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}
}

func TestParseWorkloadIdentityPolicyParenForm(t *testing.T) {
	got := ParseWorkloadIdentityPolicy(`( ALLOWED_PROVIDERS = (AWS, AZURE) ALLOWED_AWS_ACCOUNTS = ('123456789012') )`)
	if !reflect.DeepEqual(got.AllowedProviders, []string{"AWS", "AZURE"}) {
		t.Errorf("providers = %v", got.AllowedProviders)
	}
	if !reflect.DeepEqual(got.AllowedAWSAccounts, []string{"123456789012"}) {
		t.Errorf("aws accounts = %v", got.AllowedAWSAccounts)
	}
}
