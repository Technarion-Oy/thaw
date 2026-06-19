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
