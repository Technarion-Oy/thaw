// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package procedure

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// Argument represents a procedure or function argument with its metadata and value.
type Argument struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
	Value    string `json:"value"`
}

func formatValue(arg Argument) string {
	val := strings.TrimSpace(arg.Value)
	if val == "" {
		return "NULL"
	}
	if snowflake.IsBoolean(arg.DataType) || snowflake.IsNumeric(arg.DataType) {
		return val
	}
	return fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''"))
}

func escapeIdent(s string) string {
	return strings.ReplaceAll(s, "\"", "\"\"")
}

// BuildCallStatement constructs a CALL SQL statement for a stored procedure.
func BuildCallStatement(db, schema, name string, args []Argument) string {
	var formattedArgs []string
	for _, arg := range args {
		formattedArgs = append(formattedArgs, formatValue(arg))
	}
	
	joinedArgs := strings.Join(formattedArgs, ", ")
	return fmt.Sprintf("CALL \"%s\".\"%s\".\"%s\"(%s);", 
		escapeIdent(db), escapeIdent(schema), escapeIdent(name), joinedArgs)
}

// BuildFunctionSelectStatement constructs a SELECT SQL statement for a user-defined function.
func BuildFunctionSelectStatement(db, schema, name string, args []Argument, isTableFunction bool) string {
	var formattedArgs []string
	for _, arg := range args {
		formattedArgs = append(formattedArgs, formatValue(arg))
	}
	
	joinedArgs := strings.Join(formattedArgs, ", ")
	fqn := fmt.Sprintf("\"%s\".\"%s\".\"%s\"", escapeIdent(db), escapeIdent(schema), escapeIdent(name))
	
	if isTableFunction {
		return fmt.Sprintf("SELECT * FROM TABLE(%s(%s)) LIMIT 1000;", fqn, joinedArgs)
	}
	return fmt.Sprintf("SELECT %s(%s) AS result LIMIT 1000;", fqn, joinedArgs)
}
