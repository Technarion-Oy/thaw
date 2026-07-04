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
	"slices"
	"strconv"
	"strings"

	"thaw/internal/snowflake"
)

// BuildAlterUserPropertySQL builds an ALTER USER ... SET / UNSET statement for a
// single property. property must be one of: loginName, displayName, firstName,
// middleName, lastName, email, comment, password, defaultWarehouse, defaultRole,
// defaultNamespace, networkPolicy, defaultSecondaryRoles, type, daysToExpiry,
// minsToUnlock, minsToBypassMfa, disabled, mustChangePassword, disableMfa.
//
// An empty value UNSETs the property (resetting it to its default) for every
// property except the booleans (which require TRUE/FALSE) and password (which
// cannot be empty). Enum and integer values are validated before being
// interpolated into the SQL string; strings are escaped, identifiers quoted.
func BuildAlterUserPropertySQL(name, property, value string) (string, error) {
	u := snowflake.QuoteIdent(name)
	trimmed := strings.TrimSpace(value)

	checkEnum := func(v string, allowed ...string) (string, error) {
		up := strings.ToUpper(strings.TrimSpace(v))
		if slices.Contains(allowed, up) {
			return up, nil
		}
		return "", fmt.Errorf("invalid value %q for user property %q", v, property)
	}
	validateInt := func(v string) (string, error) {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 0 {
			return "", fmt.Errorf("invalid integer value %q for user property %q", v, property)
		}
		return strconv.Itoa(n), nil
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
	boolProps := map[string]string{
		"disabled":           "DISABLED",
		"mustChangePassword": "MUST_CHANGE_PASSWORD",
		"disableMfa":         "DISABLE_MFA",
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
		return setOrUnset("DEFAULT_NAMESPACE", func(v string) (string, error) {
			return snowflake.Qualify(strings.Split(v, ".")...), nil
		})
	case "defaultSecondaryRoles":
		// Only ('ALL') and () are valid values; empty UNSETs.
		if trimmed == "" {
			return fmt.Sprintf("ALTER USER %s UNSET DEFAULT_SECONDARY_ROLES", u), nil
		}
		v, err := checkEnum(value, "ALL", "NONE")
		if err != nil {
			return "", err
		}
		if v == "ALL" {
			return fmt.Sprintf("ALTER USER %s SET DEFAULT_SECONDARY_ROLES = ('ALL')", u), nil
		}
		return fmt.Sprintf("ALTER USER %s SET DEFAULT_SECONDARY_ROLES = ()", u), nil
	case "type":
		return setOrUnset("TYPE", func(v string) (string, error) {
			return checkEnum(v, "PERSON", "SERVICE", "LEGACY_SERVICE")
		})
	default:
		return "", fmt.Errorf("unknown user property: %s", property)
	}
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
