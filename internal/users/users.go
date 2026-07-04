// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package users

import (
	"context"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// BuildAlterUserPropertySQL builds an ALTER USER ... SET / UNSET statement for a
// single property. property must be one of: loginName, displayName, firstName,
// middleName, lastName, email, comment, password, defaultWarehouse, defaultRole,
// defaultNamespace, networkPolicy, defaultSecondaryRoles, type, daysToExpiry,
// minsToUnlock, minsToBypassMfa, disabled, mustChangePassword.
//
// An empty value UNSETs the property (resetting it to its default) for every
// property except the booleans (which require TRUE/FALSE) and password (which
// cannot be empty). Enum and integer values are validated before being
// interpolated into the SQL string; strings are escaped, identifiers quoted.
func BuildAlterUserPropertySQL(name, property, value string) (string, error) {
	u := snowflake.QuoteIdent(name)
	trimmed := strings.TrimSpace(value)

	what := fmt.Sprintf("user property %q", property)
	checkEnum := func(v string, allowed ...string) (string, error) {
		return snowflake.ValidateEnumValue(what, v, allowed...)
	}
	validateInt := func(v string) (string, error) {
		return snowflake.ValidateNonNegativeInt(what, v)
	}

	// setOrUnset emits `SET <KEY> = <rendered>` or, when the value is empty,
	// `UNSET <KEY>`. render is applied to the non-empty trimmed value.
	setOrUnset := func(key string, render func(string) (string, error)) (string, error) {
		if trimmed == "" {
			return fmt.Sprintf("ALTER USER %s UNSET %s", u, key), nil
		}
		rendered, err := render(trimmed)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("ALTER USER %s SET %s = %s", u, key, rendered), nil
	}
	asString := func(v string) (string, error) { return snowflake.QuoteStringLit(v), nil }
	asIdent := func(v string) (string, error) { return snowflake.QuoteIdent(v), nil }
	asInt := func(v string) (string, error) { return validateInt(v) }

	stringProps := map[string]string{
		"loginName":   "LOGIN_NAME",
		"displayName": "DISPLAY_NAME",
		"firstName":   "FIRST_NAME",
		"middleName":  "MIDDLE_NAME",
		"lastName":    "LAST_NAME",
		"email":       "EMAIL",
		"comment":     "COMMENT",
	}
	identProps := map[string]string{
		"defaultWarehouse": "DEFAULT_WAREHOUSE",
		"defaultRole":      "DEFAULT_ROLE",
		"networkPolicy":    "NETWORK_POLICY",
	}
	intProps := map[string]string{
		"daysToExpiry":    "DAYS_TO_EXPIRY",
		"minsToUnlock":    "MINS_TO_UNLOCK",
		"minsToBypassMfa": "MINS_TO_BYPASS_MFA",
	}
	// Note: MFA is deliberately not managed here — DISABLE_MFA is a legacy
	// Duo-era property with contested support; the current admin mechanism is
	// MINS_TO_BYPASS_MFA (above) or ALTER USER … REMOVE MFA METHOD.
	boolProps := map[string]string{
		"disabled":           "DISABLED",
		"mustChangePassword": "MUST_CHANGE_PASSWORD",
	}

	if key, ok := stringProps[property]; ok {
		return setOrUnset(key, asString)
	}
	if key, ok := identProps[property]; ok {
		return setOrUnset(key, asIdent)
	}
	if key, ok := intProps[property]; ok {
		return setOrUnset(key, asInt)
	}
	if key, ok := boolProps[property]; ok {
		v, err := checkEnum(value, "TRUE", "FALSE")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("ALTER USER %s SET %s = %s", u, key, v), nil
	}

	switch property {
	case "password":
		if trimmed == "" {
			return "", fmt.Errorf("password cannot be empty")
		}
		// Deliberately not trimmed — leading/trailing spaces are legal in passwords.
		return fmt.Sprintf("ALTER USER %s SET PASSWORD = %s", u, snowflake.QuoteStringLit(value)), nil
	case "defaultNamespace":
		// DATABASE or DATABASE.SCHEMA — quote each dotted part separately.
		// The split is quote-aware so a quoted identifier containing a literal
		// dot (`"MY.DB".PUB`) stays one part; empty segments ("DB.", ".SCHEMA")
		// and unbalanced quotes are rejected up front, since QuoteIdent("")
		// would render `""` and Snowflake would throw a raw syntax error.
		return setOrUnset("DEFAULT_NAMESPACE", func(v string) (string, error) {
			parts, err := splitNamespace(v)
			if err != nil || len(parts) > 2 {
				return "", fmt.Errorf("invalid namespace %q: expected DATABASE or DATABASE.SCHEMA", v)
			}
			return snowflake.Qualify(parts...), nil
		})
	case "defaultSecondaryRoles":
		// Only ('ALL') and () are valid values; empty UNSETs. The clause is
		// rendered by FormatSecondaryRoles — the shared writer for the
		// DEFAULT_SECONDARY_ROLES grammar.
		if trimmed == "" {
			return fmt.Sprintf("ALTER USER %s UNSET DEFAULT_SECONDARY_ROLES", u), nil
		}
		v, err := checkEnum(value, "ALL", "NONE")
		if err != nil {
			return "", err
		}
		var roles []string
		if v == "ALL" {
			roles = []string{"ALL"}
		}
		return fmt.Sprintf("ALTER USER %s SET DEFAULT_SECONDARY_ROLES = %s", u, snowflake.FormatSecondaryRoles(roles)), nil
	case "type":
		return setOrUnset("TYPE", func(v string) (string, error) {
			return checkEnum(v, "PERSON", "SERVICE", "LEGACY_SERVICE")
		})
	default:
		return "", fmt.Errorf("unknown user property: %s", property)
	}
}

// splitNamespace splits a DATABASE / DATABASE.SCHEMA reference on dots that sit
// outside double-quoted identifiers, so `"MY.DB".PUB` yields ["MY.DB", "PUB"].
// Each returned part is unquoted (outer quotes stripped, doubled quotes
// unescaped) and non-empty — the caller re-quotes via Qualify. Unbalanced
// quotes or empty segments return an error.
func splitNamespace(v string) ([]string, error) {
	var parts []string
	var cur strings.Builder
	inQuote := false
	rs := []rune(v)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case c == '"' && inQuote && i+1 < len(rs) && rs[i+1] == '"':
			cur.WriteRune('"') // escaped quote inside a quoted identifier
			i++
		case c == '"':
			inQuote = !inQuote
		case c == '.' && !inQuote:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(c)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unbalanced quotes in %q", v)
	}
	parts = append(parts, cur.String())
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
		if parts[i] == "" {
			return nil, fmt.Errorf("empty segment in %q", v)
		}
	}
	return parts, nil
}

// AlterProperty applies a single SET/UNSET property change to a user.
func AlterProperty(ctx context.Context, client *snowflake.Client, name, property, value string) error {
	query, err := BuildAlterUserPropertySQL(name, property, value)
	if err != nil {
		return err
	}
	_, err = client.Execute(ctx, query)
	return err
}
