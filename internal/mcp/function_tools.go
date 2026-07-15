// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/fnmeta"
	"thaw/internal/procedure"
	"thaw/internal/snowflake"
)

// Tool input types for function/procedure metadata tools.

type searchFunctionsInput struct {
	Prefix string `json:"prefix" jsonschema:"the function name prefix to search for"`
}

type functionLookupInput struct {
	Name string `json:"name" jsonschema:"the exact function name to look up"`
}

type procedureParamsInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema" jsonschema:"the schema name"`
	Name     string `json:"name" jsonschema:"the procedure name"`
	ArgTypes string `json:"arg_types,omitempty" jsonschema:"argument type signature for overloaded procedures, e.g. VARCHAR, NUMBER"`
}

type functionInfoInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema" jsonschema:"the schema name"`
	Name     string `json:"name" jsonschema:"the function name"`
	ArgTypes string `json:"arg_types,omitempty" jsonschema:"argument type signature for overloaded functions, e.g. VARCHAR, NUMBER"`
}

type buildCallStatementInput struct {
	Database string               `json:"database" jsonschema:"the database name"`
	Schema   string               `json:"schema" jsonschema:"the schema name"`
	Name     string               `json:"name" jsonschema:"the procedure name"`
	Args     []procedure.Argument `json:"args" jsonschema:"the procedure arguments with name, dataType, and value"`
}

type buildFunctionSelectInput struct {
	Database        string               `json:"database" jsonschema:"the database name"`
	Schema          string               `json:"schema" jsonschema:"the schema name"`
	Name            string               `json:"name" jsonschema:"the function name"`
	Args            []procedure.Argument `json:"args" jsonschema:"the function arguments with name, dataType, and value"`
	IsTableFunction bool                 `json:"is_table_function" jsonschema:"true for table functions (SELECT * FROM TABLE(...)), false for scalar functions"`
}

// registerFunctionTools wires function/procedure metadata and invocation
// builder tools onto srv. All tools are read-only or pure builders, so they
// are registered in every execution mode.
func registerFunctionTools(srv *mcpsdk.Server, client *snowflake.Client, fnStore *fnmeta.Store) {

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "search_functions",
		Description: "Search the local function metadata cache by name prefix. Returns matching function names, signatures, and descriptions.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in searchFunctionsInput) (*mcpsdk.CallToolResult, any, error) {
		if fnStore == nil {
			return nil, nil, fmt.Errorf("function metadata store is not available")
		}
		if strings.TrimSpace(in.Prefix) == "" {
			return nil, nil, fmt.Errorf("prefix is required")
		}
		results, err := fnStore.Search(strings.ToUpper(in.Prefix))
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(results), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_function_tooltip",
		Description: "Look up function metadata (signature, description, type) by exact name from the local cache.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in functionLookupInput) (*mcpsdk.CallToolResult, any, error) {
		if fnStore == nil {
			return nil, nil, fmt.Errorf("function metadata store is not available")
		}
		if strings.TrimSpace(in.Name) == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		results, err := fnStore.Lookup(strings.ToUpper(in.Name))
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(results), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_procedure_params",
		Description: "Retrieve parameter metadata for a stored procedure from its Snowflake DDL. Returns parameter names and data types.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in procedureParamsInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		params, err := client.GetProcedureParams(ctx, in.Database, in.Schema, in.Name, in.ArgTypes)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(params), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_function_info",
		Description: "Retrieve parameter and return-type metadata for a user-defined function from its Snowflake DDL. Indicates whether the function is a table function.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in functionInfoInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		info, err := client.GetFunctionInfo(ctx, in.Database, in.Schema, in.Name, in.ArgTypes)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(info), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_call_statement",
		Description: "Generate a syntactically correct CALL statement for a stored procedure. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in buildCallStatementInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		sql := procedure.BuildCallStatement(in.Database, in.Schema, in.Name, in.Args)
		return textResult(sql), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_function_select",
		Description: "Generate a SELECT statement to invoke a user-defined function. Handles both scalar (SELECT fn(...)) and table (SELECT * FROM TABLE(fn(...))) forms. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in buildFunctionSelectInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		sql := procedure.BuildFunctionSelectStatement(in.Database, in.Schema, in.Name, in.Args, in.IsTableFunction)
		return textResult(sql), nil, nil
	})
}
