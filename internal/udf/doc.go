// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package function builds SQL for Snowflake user-defined FUNCTION (UDF) objects —
// CREATE FUNCTION statements and the structured config behind them. Unlike an
// EXTERNAL FUNCTION (which proxies an HTTPS call to code running outside
// Snowflake), a regular UDF carries its own body: a SQL expression, or code in
// one of the handler languages (Python, Java, JavaScript, Scala) that Snowflake
// runs inside the warehouse.
//
// The builder emits the documented CREATE grammar:
//
//	CREATE [OR REPLACE] [SECURE] FUNCTION [IF NOT EXISTS] <fqn> ( [ <arg> <type> [, ...] ] )
//	  RETURNS { <result_type> | TABLE ( <col> <type> [, ...] ) }
//	  [ LANGUAGE <language> ]
//	  [ { CALLED ON NULL INPUT | RETURNS NULL ON NULL INPUT } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ RUNTIME_VERSION = '<version>' ]
//	  [ PACKAGES = ( '<package>' [, ...] ) ]
//	  [ IMPORTS = ( '<stage_path>' [, ...] ) ]
//	  [ HANDLER = '<handler>' ]
//	  [ COMMENT = '<string>' ]
//	  AS $$ <body> $$
//
// LANGUAGE is omitted for SQL functions (SQL is Snowflake's default); for SQL
// functions the body is the returned expression, for other languages it is the
// handler source code. Both are wrapped identically in $$ … $$.
//
// Functions share the regular FUNCTION management commands — they are altered and
// dropped through the plain FUNCTION grammar (ALTER FUNCTION <fqn>(<args>) … /
// DROP FUNCTION <fqn>(<args>)), so mutable properties are issued as free-form
// ALTER FUNCTION statements from internal/app/function.go (App.AlterFunction).
//
// thaw:domain: Object Browser & Administration
package udf
