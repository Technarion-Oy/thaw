// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"context"
	"strings"
)

// SessionParam holds one row from SHOW PARAMETERS. Level records where the
// current value is set (e.g. "ACCOUNT", "DATABASE", "SCHEMA", "WAREHOUSE", or ""
// when the default applies) — the object-parameter editor uses it to highlight
// values overridden at the object vs inherited from a higher scope.
type SessionParam struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Level       string `json:"level"`
	Description string `json:"description"`
}

// SessionVar holds one row from SHOW VARIABLES.
type SessionVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// MFACachingStatus reports whether the account parameter
// ALLOW_CLIENT_MFA_CACHING is enabled. Known is false when the value could not
// be read (e.g. the query failed), so callers can distinguish a confirmed
// "disabled" from "unknown" and avoid nagging when they can't be sure.
type MFACachingStatus struct {
	Enabled bool
	Known   bool
}

// GetMFACachingEnabled best-effort reads ALLOW_CLIENT_MFA_CACHING at the account
// level. When enabled, the driver can cache a reusable MFA token so pooled
// connections don't each re-prompt/re-auth; when disabled, MFA logins fall back
// to the single-use passcode and every new physical connection fails. Errors are
// swallowed into Known=false rather than surfaced, since this only drives an
// optional in-app hint. See issue #804.
func (c *Client) GetMFACachingEnabled(ctx context.Context) MFACachingStatus {
	res, err := c.Execute(ctx, "SHOW PARAMETERS LIKE 'ALLOW_CLIENT_MFA_CACHING' IN ACCOUNT")
	if err != nil || len(res.Rows) == 0 {
		return MFACachingStatus{}
	}
	val := strings.TrimSpace(strings.ToLower(Cell(res.Rows[0], ColIdx(res.Columns, "value"))))
	return MFACachingStatus{Enabled: val == "true", Known: true}
}

// GetSessionParameters returns the current session parameters from
// SHOW PARAMETERS IN SESSION.
func (c *Client) GetSessionParameters(ctx context.Context) ([]SessionParam, error) {
	res, err := c.Execute(ctx, "SHOW PARAMETERS IN SESSION")
	if err != nil {
		return nil, err
	}
	return parseParameters(res), nil
}

// GetAccountParameters returns the account-level parameters from
// SHOW PARAMETERS IN ACCOUNT. Unprivileged roles may see limited or no rows;
// that is surfaced as an empty (non-nil) slice rather than an error, so the
// caller can render a graceful "no parameters" state. Altering these requires
// ALTER ACCOUNT SET and ACCOUNTADMIN, so this view is read-only.
func (c *Client) GetAccountParameters(ctx context.Context) ([]SessionParam, error) {
	res, err := c.Execute(ctx, "SHOW PARAMETERS IN ACCOUNT")
	if err != nil {
		return nil, err
	}
	return parseParameters(res), nil
}

// GetParametersIn runs SHOW PARAMETERS IN <target> and returns the parsed rows.
// target is the object clause the caller builds from a validated object type and
// quoted identifier parts — e.g. `DATABASE "d"`, `SCHEMA "d"."s"`,
// `WAREHOUSE "w"`. Unprivileged roles may see limited or no rows, surfaced as an
// empty (non-nil) slice rather than an error, mirroring GetAccountParameters.
func (c *Client) GetParametersIn(ctx context.Context, target string) ([]SessionParam, error) {
	res, err := c.Execute(ctx, "SHOW PARAMETERS IN "+target)
	if err != nil {
		return nil, err
	}
	return parseParameters(res), nil
}

// parseParameters extracts the key/value/type/level/description columns from a
// SHOW PARAMETERS result. Columns: key, value, default, level, description,
// type. Always returns a non-nil slice.
func parseParameters(res *QueryResult) []SessionParam {
	keyIdx := ColIdx(res.Columns, "key", "name")
	valIdx := ColIdx(res.Columns, "value")
	typIdx := ColIdx(res.Columns, "type")
	lvlIdx := ColIdx(res.Columns, "level")
	descIdx := ColIdx(res.Columns, "description")

	var params []SessionParam
	for _, row := range res.Rows {
		key := Cell(row, keyIdx)
		val := Cell(row, valIdx)
		typ := Cell(row, typIdx)
		lvl := Cell(row, lvlIdx)
		desc := Cell(row, descIdx)
		if key != "" {
			params = append(params, SessionParam{Key: key, Value: val, Type: typ, Level: lvl, Description: desc})
		}
	}
	if params == nil {
		params = []SessionParam{}
	}
	return params
}

// GetSessionVariables returns the current session variables from SHOW VARIABLES.
func (c *Client) GetSessionVariables(ctx context.Context) ([]SessionVar, error) {
	res, err := c.Execute(ctx, "SHOW VARIABLES")
	if err != nil {
		return nil, err
	}

	// SHOW VARIABLES columns: name, value, default, type, ...
	nameIdx := ColIdx(res.Columns, "name", "key")
	valIdx := ColIdx(res.Columns, "value")
	typIdx := ColIdx(res.Columns, "type")

	var vars []SessionVar
	for _, row := range res.Rows {
		name := Cell(row, nameIdx)
		val := Cell(row, valIdx)
		typ := Cell(row, typIdx)
		if name != "" {
			vars = append(vars, SessionVar{Key: name, Value: val, Type: typ})
		}
	}
	if vars == nil {
		vars = []SessionVar{}
	}
	return vars, nil
}

// QuoteSessionParamValue wraps value in single quotes (with escaping) when
// paramType indicates a string-like type; it returns value unchanged for
// boolean and numeric parameter types.
func QuoteSessionParamValue(value, paramType string) string {
	switch strings.ToUpper(paramType) {
	case "BOOLEAN", "NUMBER", "FIXED", "FLOAT":
		return value
	default:
		return QuoteStringLit(value)
	}
}
