// SPDX-License-Identifier: GPL-3.0-or-later

// Package dataset builds SQL for Snowflake DATASET objects — CREATE DATASET
// statements and the structured config behind them. A dataset is a schema-level
// object from the Snowpark ML feature set: it holds versioned, immutable
// snapshots of data used for ML training and evaluation. Most datasets are
// produced via the Snowpark ML Python API; the SQL CREATE DATASET statement
// covered here creates an empty dataset (just a name, with the optional OR
// REPLACE / IF NOT EXISTS flags — there is no COMMENT or other property on
// CREATE).
//
// Datasets are versioned: data is added to a dataset one version at a time. The
// entire ALTER DATASET surface is version management — ADD VERSION (from a
// query, with optional PARTITION BY / COMMENT / METADATA) and DROP VERSION — both
// issued as free-form ALTER DATASET statements from internal/app/dataset.go
// (App.AlterDataset); the version list is read with SHOW VERSIONS IN DATASET
// (App.ListDatasetVersions). ALTER DATASET has no RENAME, SET COMMENT, or SET TAG
// clause, and GET_DDL does not support datasets, so there is no rename or
// DDL-export path for this type.
//
// thaw:domain: Object Browser & Administration
package dataset
