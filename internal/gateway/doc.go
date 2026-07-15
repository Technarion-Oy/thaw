// SPDX-License-Identifier: GPL-3.0-or-later

// Package gateway builds SQL for Snowflake GATEWAY objects — CREATE GATEWAY and
// ALTER GATEWAY statements plus the structured config behind them. A gateway is
// a schema-level Snowpark Container Services object that fronts service
// endpoints: it splits ingress HTTP traffic across up to five service endpoints
// according to a YAML specification (type: traffic_split / split_type: custom /
// targets with weights summing to 100), exposing an external ingress URL.
//
// Both CREATE and ALTER carry only the specification — there is no COMMENT, TAG,
// or any other clause:
//
//	CREATE [OR REPLACE] GATEWAY [IF NOT EXISTS] <fqn> FROM SPECIFICATION <spec>
//	ALTER  GATEWAY [IF EXISTS] <fqn>               FROM SPECIFICATION <spec>
//
// The specification is emitted inside a tagged $THAW$ … $THAW$ dollar-quote so
// the multi-line YAML needs no escaping. The whole ALTER GATEWAY surface is the
// FROM SPECIFICATION update (re-route traffic) — there is no RENAME, SET COMMENT,
// or SET TAG — so the only mutation is editing the spec, reachable from the
// properties panel (App.AlterGateway). GET_DDL does not support gateways, so
// there is no DDL-export path; the live specification is read with
// DESCRIBE GATEWAY (App.DescribeGateway), which also returns the ingress URL.
//
// thaw:domain: Object Browser & Administration
package gateway
