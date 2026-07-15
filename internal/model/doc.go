// SPDX-License-Identifier: GPL-3.0-or-later

// Package model builds SQL for Snowflake MODEL objects — CREATE MODEL statements
// and the structured config behind them. A model is a schema-level object from
// the Snowpark ML Model Registry: it holds one or more versioned ML artifacts and
// can be invoked as a function for inference. Most models are registered via the
// Snowpark ML Python API; the SQL CREATE MODEL statement covered here builds a
// model either by copying an existing model (FROM MODEL … [VERSION …]) or by
// loading serialized artifacts from an internal stage (FROM @stage).
//
// Models support versioning: each model has a set of versions, a default version
// used for direct method calls, and optional per-version aliases. The mutable
// properties (COMMENT, DEFAULT_VERSION, per-version ALIAS, tags) and RENAME are
// issued as free-form ALTER MODEL statements from internal/app/model.go
// (App.AlterModel); the version list is read with SHOW VERSIONS IN MODEL
// (App.ListModelVersions). GET_DDL does not support models, so there is no
// DDL-export path for this type.
//
// thaw:domain: Object Browser & Administration
package model
