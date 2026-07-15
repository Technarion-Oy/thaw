// SPDX-License-Identifier: GPL-3.0-or-later

// Package datametricfunction builds SQL for Snowflake DATA METRIC FUNCTION (DMF)
// objects — CREATE DATA METRIC FUNCTION statements and the structured config
// behind them. A data metric function defines a data-quality rule that returns a
// single NUMBER metric (e.g. a count of NULLs, of rows failing a regular
// expression, or of duplicate keys). DMFs are scheduled against tables and views
// via ALTER TABLE … ADD DATA METRIC FUNCTION and their results are surfaced for
// monitoring and alerting; this package only covers the lifecycle of the DMF
// definition itself.
//
// A DMF is distinct from a regular UDF: its argument is one (or more) TABLE
// argument(s) — a named set of typed columns — rather than scalar parameters, it
// always RETURNS NUMBER, and its body is a deterministic scalar SQL expression
// that aggregates over the table argument.
//
// The builder emits the documented CREATE grammar:
//
//	CREATE [OR REPLACE] [SECURE] DATA METRIC FUNCTION [IF NOT EXISTS] <fqn>
//	  ( <arg_name> TABLE ( <col> <type> [, ...] ) )
//	  RETURNS NUMBER [NOT NULL]
//	  [ COMMENT = '<string>' ]
//	  AS '<expression>'
//
// OR REPLACE and IF NOT EXISTS are mutually exclusive (the builder drops IF NOT
// EXISTS when OR REPLACE is set). The return type is always NUMBER. The body is
// emitted with $$ dollar-quoting so multi-line SQL expressions containing single
// quotes (e.g. REGEXP literals) need no escaping.
//
// Data metric functions share the regular FUNCTION management commands rather
// than having DMF-specific variants: they are altered and dropped through the
// plain FUNCTION grammar (ALTER FUNCTION <fqn>(TABLE(<types>)) … / DROP FUNCTION
// <fqn>(TABLE(<types>))), so the mutable properties are issued as free-form ALTER
// FUNCTION statements from internal/app/datametricfunction.go
// (App.AlterDataMetricFunction), and DESCRIBE FUNCTION supplies the body that
// SHOW DATA METRIC FUNCTIONS omits (App.DescribeDataMetricFunction). GET_DDL has
// no DATA_METRIC_FUNCTION object type — DMFs are retrieved via the 'FUNCTION'
// type with the argument signature appended, normalized in internal/snowflake
// (buildGetDDLQuery), not here.
//
// thaw:domain: Object Browser & Administration
package datametricfunction
