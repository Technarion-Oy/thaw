// SPDX-License-Identifier: GPL-3.0-or-later

// Package externalfunction builds SQL for Snowflake EXTERNAL FUNCTION objects —
// CREATE EXTERNAL FUNCTION statements and the structured config behind them. An
// external function is a user-defined function that, instead of running code
// stored in Snowflake, calls code executed outside Snowflake (e.g. an AWS Lambda,
// an Azure Function, or a GCP Cloud Function) through an API integration that
// proxies the HTTPS request. This makes external functions distinct from regular
// UDFs: they have no body, they route every call through an API_INTEGRATION and a
// remote URL, and they carry transport options (headers, batching, compression,
// request/response translators) that regular UDFs do not.
//
// The builder emits the documented CREATE grammar:
//
//	CREATE [OR REPLACE] [SECURE] EXTERNAL FUNCTION <fqn> ( [ <arg> <type> [, ...] ] )
//	  RETURNS <result_type> [NOT NULL]
//	  [ { CALLED ON NULL INPUT | RETURNS NULL ON NULL INPUT | STRICT } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ COMMENT = '<string>' ]
//	  API_INTEGRATION = <integration>
//	  [ HEADERS = ( '<h>' = '<v>' [, ...] ) ]
//	  [ CONTEXT_HEADERS = ( <context_fn> [, ...] ) ]
//	  [ MAX_BATCH_ROWS = <int> ]
//	  [ COMPRESSION = <type> ]
//	  [ REQUEST_TRANSLATOR = <udf> ]
//	  [ RESPONSE_TRANSLATOR = <udf> ]
//	  AS '<url_of_proxy_and_resource>'
//
// API_INTEGRATION and the AS '<url>' are mandatory; everything else is optional
// and emitted only when set.
//
// External functions share the regular FUNCTION management commands rather than
// having EXTERNAL-specific variants: they are altered and dropped through the
// plain FUNCTION grammar (ALTER FUNCTION <fqn>(<args>) … / DROP FUNCTION
// <fqn>(<args>)), so the mutable properties are issued as free-form ALTER FUNCTION
// statements from internal/app/externalfunction.go (App.AlterExternalFunction),
// and DESCRIBE FUNCTION supplies the rich detail (API integration, URL, headers,
// translators, compression) that SHOW EXTERNAL FUNCTIONS omits
// (App.DescribeExternalFunction). GET_DDL has no EXTERNAL_FUNCTION object type —
// external functions are retrieved via the 'FUNCTION' type with the argument
// signature appended, normalized in internal/snowflake (buildGetDDLQuery), not
// here.
//
// thaw:domain: Object Browser & Administration
package externalfunction
