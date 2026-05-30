// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import (
	"context"
	"strings"
)

// SessionParam holds one row from SHOW PARAMETERS.
type SessionParam struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// SessionVar holds one row from SHOW VARIABLES.
type SessionVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// GetSessionParameters returns the current session parameters from
// SHOW PARAMETERS IN SESSION.
func (c *Client) GetSessionParameters(ctx context.Context) ([]SessionParam, error) {
	res, err := c.Execute(ctx, "SHOW PARAMETERS IN SESSION")
	if err != nil {
		return nil, err
	}

	// SHOW PARAMETERS columns: key, value, default, level, description, type
	keyIdx := ColIdx(res.Columns, "key", "name")
	valIdx := ColIdx(res.Columns, "value")
	typIdx := ColIdx(res.Columns, "type")
	descIdx := ColIdx(res.Columns, "description")

	var params []SessionParam
	for _, row := range res.Rows {
		key, val, typ, desc := "", "", "", ""
		if keyIdx >= 0 && keyIdx < len(row) {
			key = CellString(row[keyIdx])
		}
		if valIdx >= 0 && valIdx < len(row) {
			val = CellString(row[valIdx])
		}
		if typIdx >= 0 && typIdx < len(row) {
			typ = CellString(row[typIdx])
		}
		if descIdx >= 0 && descIdx < len(row) {
			desc = CellString(row[descIdx])
		}
		if key != "" {
			params = append(params, SessionParam{Key: key, Value: val, Type: typ, Description: desc})
		}
	}
	if params == nil {
		params = []SessionParam{}
	}
	return params, nil
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
		name, val, typ := "", "", ""
		if nameIdx >= 0 && nameIdx < len(row) {
			name = CellString(row[nameIdx])
		}
		if valIdx >= 0 && valIdx < len(row) {
			val = CellString(row[valIdx])
		}
		if typIdx >= 0 && typIdx < len(row) {
			typ = CellString(row[typIdx])
		}
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
		return "'" + strings.ReplaceAll(value, "'", "''") + "'"
	}
}
