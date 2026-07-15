// SPDX-License-Identifier: GPL-3.0-or-later

// Package icebergtable builds SQL for Snowflake ICEBERG TABLE objects — CREATE
// ICEBERG TABLE statements and the structured config behind them. An Iceberg
// table stores its data in the open Apache Iceberg table format on an external
// volume (cloud object storage), enabling interoperability with engines such as
// Spark and Trino.
//
// CREATE ICEBERG TABLE has several variants whose required attributes differ;
// the builder models each via IcebergTableConfig.TableType:
//
//   - Snowflake-managed (CATALOG = 'SNOWFLAKE'): Snowflake is the Iceberg
//     catalog. Requires a column list and a BASE_LOCATION.
//   - External Iceberg catalog — Iceberg REST or AWS Glue (identical table DDL;
//     the difference is the catalog integration type). Requires
//     CATALOG_TABLE_NAME (CATALOG_NAMESPACE / AUTO_REFRESH optional).
//   - Delta Lake files in object storage. Requires BASE_LOCATION; the catalog
//     integration must use CATALOG_SOURCE = OBJECT_STORE, TABLE_FORMAT = DELTA.
//   - Iceberg files in object storage (unmanaged). Requires METADATA_FILE_PATH;
//     this variant has no AUTO_REFRESH.
//
// EXTERNAL_VOLUME and CATALOG are optional for every variant (a schema /
// database / account default is used when omitted). Columns are inferred for
// every non-Snowflake-managed variant.
//
// The mutable properties — COMMENT plus the manual REFRESH that re-syncs an
// externally-managed table's metadata — and RENAME TO are issued as free-form
// ALTER ICEBERG TABLE statements from internal/app/icebergtable.go
// (App.AlterIcebergTable). GET_DDL has no ICEBERG_TABLE object type; Iceberg
// tables are retrieved via the 'TABLE' type, so DDL export is normalized in
// internal/snowflake (buildGetDDLQuery), not here.
//
// thaw:domain: Object Browser & Administration
package icebergtable
