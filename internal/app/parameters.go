// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"
	"regexp"
	"strings"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// objectParamTypes is the allowlist of object kinds the generic parameters
// viewer/editor supports. The object type is interpolated straight into the SQL
// (`SHOW PARAMETERS IN <TYPE> …`, `ALTER <TYPE> … SET …`), so it MUST be
// validated against this set — never passed through verbatim.
var objectParamTypes = map[string]bool{
	"DATABASE":  true,
	"SCHEMA":    true,
	"WAREHOUSE": true,
	"TABLE":     true,
	"TASK":      true,
	"USER":      true,
}

// paramNamePattern matches a bare Snowflake parameter name. Parameter names come
// from SHOW PARAMETERS output (trusted), but the name is interpolated unquoted
// into ALTER … SET/UNSET, so it is validated defensively against injection.
var paramNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

// objectTarget builds the "<TYPE> <qualified-name>" clause shared by the
// SHOW PARAMETERS and ALTER statements, validating the object type against the
// allowlist and double-quoting each identifier part. Blank parts are skipped so
// callers can pass a fixed-arity [db, schema, name] slice for objects with fewer
// levels (a warehouse is [name], a schema [db, schema]).
func objectTarget(objectType string, nameParts []string) (string, error) {
	ot := strings.ToUpper(strings.TrimSpace(objectType))
	if !objectParamTypes[ot] {
		return "", fmt.Errorf("unsupported object type %q", objectType)
	}
	var quoted []string
	for _, p := range nameParts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		quoted = append(quoted, snowflake.QuoteIdent(p))
	}
	if len(quoted) == 0 {
		return "", fmt.Errorf("object name is required")
	}
	return ot + " " + strings.Join(quoted, "."), nil
}

// GetObjectParameters returns all parameters for a single object via
// SHOW PARAMETERS IN <objectType> <name>. objectType must be one of DATABASE,
// SCHEMA, TABLE, WAREHOUSE, TASK, USER; nameParts holds the identifier parts of
// the object's qualified name (e.g. ["DB"], ["DB","SCHEMA"], ["WH"]). Unprivileged
// roles may see limited or no rows, which the frontend renders gracefully.
func (a *App) GetObjectParameters(objectType string, nameParts []string) ([]snowflake.SessionParam, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	target, err := objectTarget(objectType, nameParts)
	if err != nil {
		return nil, err
	}
	return client.GetParametersIn(a.fctx(FeatureObjectBrowser), target)
}

// SetObjectParameter applies ALTER <objectType> <name> SET <param> = <value> for
// a single object parameter, quoting the value per its type. Requires ownership
// or the relevant privilege on the object; Snowflake returns a privilege error
// otherwise, which the frontend surfaces inline.
func (a *App) SetObjectParameter(objectType string, nameParts []string, name, value, paramType string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	target, err := objectTarget(objectType, nameParts)
	if err != nil {
		return err
	}
	if !paramNamePattern.MatchString(name) {
		return fmt.Errorf("invalid parameter name %q", name)
	}
	valExpr := snowflake.QuoteSessionParamValue(value, paramType)
	_, err = client.Execute(a.fctx(FeatureObjectBrowser),
		fmt.Sprintf("ALTER %s SET %s = %s", target, name, valExpr))
	return err
}

// UnsetObjectParameter applies ALTER <objectType> <name> UNSET <param>, reverting
// the parameter to the value inherited from a higher scope (account/database/…).
func (a *App) UnsetObjectParameter(objectType string, nameParts []string, name string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	target, err := objectTarget(objectType, nameParts)
	if err != nil {
		return err
	}
	if !paramNamePattern.MatchString(name) {
		return fmt.Errorf("invalid parameter name %q", name)
	}
	_, err = client.Execute(a.fctx(FeatureObjectBrowser),
		fmt.Sprintf("ALTER %s UNSET %s", target, name))
	return err
}
