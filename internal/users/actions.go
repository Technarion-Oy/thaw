// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// This file holds the non-property `ALTER USER` action builders — the variants
// that are one-shot commands rather than a SET/UNSET of a single named property
// (which lives in users.go / BuildAlterUserPropertySQL). Every builder validates
// and quotes its inputs here, so the IPC delegators in internal/app/users.go and
// the UI never assemble ALTER USER text inline.
//
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-user

// TagPair is one `<tag_name> = '<tag_value>'` assignment in a SET TAG clause.
// Name may be a qualified tag reference (DB.SCHEMA.TAG); Value is a free-text
// string literal.
type TagPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// renderQualifiedName renders a free-hand, possibly-qualified identifier (a tag
// or policy name typed in the UI) for interpolation into SQL. It splits
// quote-aware via snowflake.SplitQualifiedName (so `"MY.TAG"` stays one part),
// then renders each part: explicitly-quoted parts keep their exact case
// (QuoteIdent), bare parts stay bare (QuoteOrBare) so Snowflake's identifier
// folding resolves them — the same rule DEFAULT_NAMESPACE and NETWORK_POLICY use.
func renderQualifiedName(what, v string, maxParts int) (string, error) {
	parts, err := snowflake.SplitQualifiedName(v, maxParts)
	if err != nil || len(parts) == 0 {
		return "", fmt.Errorf("invalid %s %q", what, v)
	}
	rendered := make([]string, len(parts))
	for i, p := range parts {
		if p.Quoted {
			rendered[i] = snowflake.QuoteIdent(p.Text)
		} else {
			rendered[i] = snowflake.QuoteOrBare(p.Text, false)
		}
	}
	return strings.Join(rendered, "."), nil
}

// BuildResetPasswordSQL builds `ALTER USER <name> RESET PASSWORD`, which
// generates a fresh single-use password reset URL for the user (it does not take
// a new password — use BuildAlterUserPropertySQL(name, "password", …) for that).
func BuildResetPasswordSQL(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("user name is required")
	}
	return fmt.Sprintf("ALTER USER %s RESET PASSWORD", snowflake.QuoteIdent(name)), nil
}

// BuildRenameUserSQL builds `ALTER USER <name> RENAME TO <new_name>`. newName is
// typed free-hand, so a bare name stays bare (folded by Snowflake) and a name
// needing quoting is quoted — mirroring the other free-hand identifier fields.
func BuildRenameUserSQL(name, newName string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("user name is required")
	}
	if strings.TrimSpace(newName) == "" {
		return "", fmt.Errorf("new user name is required")
	}
	target, err := renderQualifiedName("user name", newName, 1)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER USER %s RENAME TO %s", snowflake.QuoteIdent(name), target), nil
}

// BuildAbortAllQueriesSQL builds `ALTER USER <name> ABORT ALL QUERIES`, which
// cancels every running and queued query for the user across all sessions.
func BuildAbortAllQueriesSQL(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("user name is required")
	}
	return fmt.Sprintf("ALTER USER %s ABORT ALL QUERIES", snowflake.QuoteIdent(name)), nil
}

// BuildRemoveMfaMethodSQL builds `ALTER USER <name> REMOVE MFA METHOD <method>`,
// removing one enrolled MFA method so the user can re-enroll. method is one of
// PASSKEY, TOTP, DUO (the documented mfaActions method keywords).
func BuildRemoveMfaMethodSQL(name, method string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("user name is required")
	}
	m, err := snowflake.ValidateEnumValue("MFA method", method, "PASSKEY", "TOTP", "DUO")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER USER %s REMOVE MFA METHOD %s", snowflake.QuoteIdent(name), m), nil
}

// policyKeyword maps the policy kind selector to its SQL keyword and validates
// it. kind is one of AUTHENTICATION, PASSWORD, SESSION (case-insensitive).
func policyKeyword(kind string) (string, error) {
	return snowflake.ValidateEnumValue("policy kind", kind, "AUTHENTICATION", "PASSWORD", "SESSION")
}

// BuildSetPolicySQL builds
// `ALTER USER <name> SET { AUTHENTICATION | PASSWORD | SESSION } POLICY <policy_name> [ FORCE ]`.
// policyName may be a qualified reference. FORCE detaches any policy of the same
// kind already attached to the user before attaching the new one.
func BuildSetPolicySQL(name, kind, policyName string, force bool) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("user name is required")
	}
	keyword, err := policyKeyword(kind)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(policyName) == "" {
		return "", fmt.Errorf("policy name is required")
	}
	policy, err := renderQualifiedName("policy name", policyName, 3)
	if err != nil {
		return "", err
	}
	sql := fmt.Sprintf("ALTER USER %s SET %s POLICY %s", snowflake.QuoteIdent(name), keyword, policy)
	if force {
		sql += " FORCE"
	}
	return sql, nil
}

// BuildUnsetPolicySQL builds
// `ALTER USER <name> UNSET { AUTHENTICATION | PASSWORD | SESSION } POLICY`.
func BuildUnsetPolicySQL(name, kind string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("user name is required")
	}
	keyword, err := policyKeyword(kind)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER USER %s UNSET %s POLICY", snowflake.QuoteIdent(name), keyword), nil
}

// BuildSetTagsSQL builds
// `ALTER USER <name> SET TAG <t1> = '<v1>' [ , <t2> = '<v2>' … ]`.
// Each tag name may be a qualified reference; values are quoted free-text
// literals. At least one tag is required.
func BuildSetTagsSQL(name string, tags []TagPair) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("user name is required")
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("at least one tag is required")
	}
	assignments := make([]string, 0, len(tags))
	for _, t := range tags {
		if strings.TrimSpace(t.Name) == "" {
			return "", fmt.Errorf("tag name is required")
		}
		tagName, err := renderQualifiedName("tag name", t.Name, 3)
		if err != nil {
			return "", err
		}
		assignments = append(assignments, fmt.Sprintf("%s = %s", tagName, snowflake.QuoteTextLit(t.Value)))
	}
	return fmt.Sprintf("ALTER USER %s SET TAG %s", snowflake.QuoteIdent(name), strings.Join(assignments, ", ")), nil
}

// BuildUnsetTagsSQL builds `ALTER USER <name> UNSET TAG <t1> [ , <t2> … ]`.
// Each tag name may be a qualified reference. At least one tag is required.
func BuildUnsetTagsSQL(name string, tagNames []string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("user name is required")
	}
	if len(tagNames) == 0 {
		return "", fmt.Errorf("at least one tag is required")
	}
	rendered := make([]string, 0, len(tagNames))
	for _, n := range tagNames {
		if strings.TrimSpace(n) == "" {
			return "", fmt.Errorf("tag name is required")
		}
		tagName, err := renderQualifiedName("tag name", n, 3)
		if err != nil {
			return "", err
		}
		rendered = append(rendered, tagName)
	}
	return fmt.Sprintf("ALTER USER %s UNSET TAG %s", snowflake.QuoteIdent(name), strings.Join(rendered, ", ")), nil
}

// BuildAddDelegatedAuthSQL builds
// `ALTER USER <name> ADD DELEGATED AUTHORIZATION OF ROLE <role> TO SECURITY INTEGRATION <integration>`.
// role and integration are typed free-hand (bare names fold).
func BuildAddDelegatedAuthSQL(name, role, integration string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("user name is required")
	}
	if strings.TrimSpace(role) == "" {
		return "", fmt.Errorf("role name is required")
	}
	if strings.TrimSpace(integration) == "" {
		return "", fmt.Errorf("security integration name is required")
	}
	roleRef, err := renderQualifiedName("role name", role, 1)
	if err != nil {
		return "", err
	}
	intRef, err := renderQualifiedName("security integration name", integration, 1)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"ALTER USER %s ADD DELEGATED AUTHORIZATION OF ROLE %s TO SECURITY INTEGRATION %s",
		snowflake.QuoteIdent(name), roleRef, intRef,
	), nil
}

// BuildRemoveDelegatedAuthSQL builds one of the two REMOVE DELEGATED variants:
//   - role set → `REMOVE DELEGATED AUTHORIZATION OF ROLE <role> FROM SECURITY INTEGRATION <integration>`
//   - role empty → `REMOVE DELEGATED AUTHORIZATIONS FROM SECURITY INTEGRATION <integration>`
//     (removes every delegated authorization for the integration)
func BuildRemoveDelegatedAuthSQL(name, role, integration string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("user name is required")
	}
	if strings.TrimSpace(integration) == "" {
		return "", fmt.Errorf("security integration name is required")
	}
	intRef, err := renderQualifiedName("security integration name", integration, 1)
	if err != nil {
		return "", err
	}
	var clause string
	if strings.TrimSpace(role) == "" {
		clause = "REMOVE DELEGATED AUTHORIZATIONS"
	} else {
		roleRef, err := renderQualifiedName("role name", role, 1)
		if err != nil {
			return "", err
		}
		clause = fmt.Sprintf("REMOVE DELEGATED AUTHORIZATION OF ROLE %s", roleRef)
	}
	return fmt.Sprintf("ALTER USER %s %s FROM SECURITY INTEGRATION %s", snowflake.QuoteIdent(name), clause, intRef), nil
}

// exec runs a builder's output against the client, sharing the build-then-execute
// shape of AlterProperty for the action builders.
func exec(ctx context.Context, client *snowflake.Client, sql string, buildErr error) error {
	if buildErr != nil {
		return buildErr
	}
	_, err := client.Execute(ctx, sql)
	return err
}

// ResetPassword runs ALTER USER … RESET PASSWORD.
func ResetPassword(ctx context.Context, client *snowflake.Client, name string) error {
	sql, err := BuildResetPasswordSQL(name)
	return exec(ctx, client, sql, err)
}

// Rename runs ALTER USER … RENAME TO.
func Rename(ctx context.Context, client *snowflake.Client, name, newName string) error {
	sql, err := BuildRenameUserSQL(name, newName)
	return exec(ctx, client, sql, err)
}

// AbortAllQueries runs ALTER USER … ABORT ALL QUERIES.
func AbortAllQueries(ctx context.Context, client *snowflake.Client, name string) error {
	sql, err := BuildAbortAllQueriesSQL(name)
	return exec(ctx, client, sql, err)
}

// RemoveMfaMethod runs ALTER USER … REMOVE MFA METHOD <method>.
func RemoveMfaMethod(ctx context.Context, client *snowflake.Client, name, method string) error {
	sql, err := BuildRemoveMfaMethodSQL(name, method)
	return exec(ctx, client, sql, err)
}

// SetPolicy runs ALTER USER … SET { AUTHENTICATION | PASSWORD | SESSION } POLICY.
func SetPolicy(ctx context.Context, client *snowflake.Client, name, kind, policyName string, force bool) error {
	sql, err := BuildSetPolicySQL(name, kind, policyName, force)
	return exec(ctx, client, sql, err)
}

// UnsetPolicy runs ALTER USER … UNSET { AUTHENTICATION | PASSWORD | SESSION } POLICY.
func UnsetPolicy(ctx context.Context, client *snowflake.Client, name, kind string) error {
	sql, err := BuildUnsetPolicySQL(name, kind)
	return exec(ctx, client, sql, err)
}

// SetTags runs ALTER USER … SET TAG.
func SetTags(ctx context.Context, client *snowflake.Client, name string, tags []TagPair) error {
	sql, err := BuildSetTagsSQL(name, tags)
	return exec(ctx, client, sql, err)
}

// UnsetTags runs ALTER USER … UNSET TAG.
func UnsetTags(ctx context.Context, client *snowflake.Client, name string, tagNames []string) error {
	sql, err := BuildUnsetTagsSQL(name, tagNames)
	return exec(ctx, client, sql, err)
}

// AddDelegatedAuth runs ALTER USER … ADD DELEGATED AUTHORIZATION OF ROLE … TO SECURITY INTEGRATION ….
func AddDelegatedAuth(ctx context.Context, client *snowflake.Client, name, role, integration string) error {
	sql, err := BuildAddDelegatedAuthSQL(name, role, integration)
	return exec(ctx, client, sql, err)
}

// RemoveDelegatedAuth runs ALTER USER … REMOVE DELEGATED … FROM SECURITY INTEGRATION ….
func RemoveDelegatedAuth(ctx context.Context, client *snowflake.Client, name, role, integration string) error {
	sql, err := BuildRemoveDelegatedAuthSQL(name, role, integration)
	return exec(ctx, client, sql, err)
}
