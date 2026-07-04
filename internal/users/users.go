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
	"thaw/internal/sqltok"
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
	// QuoteTextLit, not QuoteStringLit: these are human-entered values, and
	// Snowflake treats backslash as an escape inside single-quoted literals —
	// a trailing `\` would swallow the closing quote and break the statement.
	asString := func(v string) (string, error) { return snowflake.QuoteTextLit(v), nil }
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
	// defaultWarehouse / defaultRole arrive from Selects populated with
	// canonical-case names out of SHOW, so unconditional quoting is exact.
	// networkPolicy is typed free-hand and handled below with QuoteOrBare.
	identProps := map[string]string{
		"defaultWarehouse": "DEFAULT_WAREHOUSE",
		"defaultRole":      "DEFAULT_ROLE",
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
	case "networkPolicy":
		// Typed free-hand in the UI: QuoteOrBare leaves a valid bare name
		// unquoted so Snowflake's identifier folding resolves `corp_policy`
		// to the uppercase-stored CORP_POLICY, matching the other builders.
		return setOrUnset("NETWORK_POLICY", func(v string) (string, error) {
			return snowflake.QuoteOrBare(v, false), nil
		})
	case "password":
		if trimmed == "" {
			return "", fmt.Errorf("password cannot be empty")
		}
		// Deliberately not trimmed — leading/trailing spaces are legal in
		// passwords, and QuoteTextLit keeps embedded backslashes literal.
		return fmt.Sprintf("ALTER USER %s SET PASSWORD = %s", u, snowflake.QuoteTextLit(value)), nil
	case "defaultNamespace":
		// DATABASE or DATABASE.SCHEMA — the split is quote-aware so a quoted
		// identifier containing a literal dot (`"MY.DB".PUB`) stays one part;
		// empty segments ("DB.", ".SCHEMA") and unbalanced quotes are rejected
		// up front, since QuoteIdent("") would render `""` and Snowflake would
		// throw a raw syntax error. Explicitly-quoted parts keep their exact
		// case (QuoteIdent); bare parts are typed free-hand and stay bare
		// (QuoteOrBare) so Snowflake's identifier folding resolves them.
		return setOrUnset("DEFAULT_NAMESPACE", func(v string) (string, error) {
			parts, err := splitNamespace(v)
			if err != nil || len(parts) > 2 {
				return "", fmt.Errorf("invalid namespace %q: expected DATABASE or DATABASE.SCHEMA", v)
			}
			rendered := make([]string, len(parts))
			for i, p := range parts {
				if p.quoted {
					rendered[i] = snowflake.QuoteIdent(p.text)
				} else {
					rendered[i] = snowflake.QuoteOrBare(p.text, false)
				}
			}
			return strings.Join(rendered, "."), nil
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

// nsPart is one dotted segment of a namespace reference: its unquoted text and
// whether the input wrapped it in double quotes (case-sensitive intent).
type nsPart struct {
	text   string
	quoted bool
}

// splitNamespace splits a DATABASE / DATABASE.SCHEMA reference into its dotted
// parts using the shared sqltok tokenizer (quote-aware, so `"MY.DB".PUB`
// yields ["MY.DB", "PUB"]). Each returned part is unquoted via sqltok.Unquote
// and non-empty, with a flag recording whether it was quoted — the caller
// re-quotes accordingly. Trailing tokens, empty segments, and unterminated
// quotes return an error.
func splitNamespace(v string) ([]nsPart, error) {
	trimmed := strings.TrimSpace(v)
	tokens := sqltok.Tokenize(trimmed)
	raw, next := sqltok.ReadIdentParts(tokens, trimmed, 0, 2)
	// The whole input must be exactly one identifier path — reject leftovers
	// ("DB.", third parts beyond maxParts, stray tokens).
	if raw == nil || next >= len(tokens) || tokens[next].Kind != sqltok.EOF {
		return nil, fmt.Errorf("invalid namespace %q", v)
	}
	parts := make([]nsPart, len(raw))
	for i, p := range raw {
		if strings.HasPrefix(p, `"`) {
			// Quoted identifier: must be a terminated pair.
			if len(p) < 2 || !strings.HasSuffix(p, `"`) {
				return nil, fmt.Errorf("unbalanced quotes in %q", v)
			}
			parts[i] = nsPart{text: sqltok.Unquote(p), quoted: true}
		} else {
			parts[i] = nsPart{text: p, quoted: false}
		}
		if parts[i].text == "" {
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
