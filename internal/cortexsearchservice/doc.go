// SPDX-License-Identifier: GPL-3.0-or-later

// Package cortexsearchservice builds SQL for Snowflake CORTEX SEARCH SERVICE
// objects — CREATE CORTEX SEARCH SERVICE statements and the structured config
// behind them. A Cortex Search Service is a schema-level object that indexes a
// text column over the rows of a source query to provide low-latency hybrid
// (keyword + semantic) search and retrieval for RAG applications.
//
// The CREATE builder covers both documented shapes, selected by IndexMode:
// the single-index ("ON <column>", with an optional EMBEDDING_MODEL) form and the
// multi-index (TEXT INDEXES / VECTOR INDEXES) form. Either way it can emit
// PRIMARY KEY, filterable ATTRIBUTES, the refresh WAREHOUSE and TARGET_LAG, the
// REFRESH_MODE / INITIALIZE / FULL_INDEX_BUILD_INTERVAL_DAYS / REQUEST_LOGGING /
// AUTO_SUSPEND tuning options, a COMMENT, and the defining query appended verbatim
// after AS. The multi-index form omits EMBEDDING_MODEL and IF NOT EXISTS, which
// Snowflake does not allow there. Scoring profiles are added post-create via
// ALTER (see below) rather than in CREATE.
//
// Every ALTER CORTEX SEARCH SERVICE option is reachable from the properties modal
// via free-form ALTER statements issued through internal/app/cortexsearchservice.go
// (App.AlterCortexSearchService): the lifecycle actions
// SUSPEND / RESUME [ INDEXING | SERVING ] and REFRESH; the SET/UNSET properties
// TARGET_LAG, WAREHOUSE, ATTRIBUTES, PRIMARY KEY, AUTO_SUSPEND,
// FULL_INDEX_BUILD_INTERVAL_DAYS, REQUEST_LOGGING and COMMENT; SET/UNSET TAG; and
// ADD/DROP SCORING PROFILE. EMBEDDING_MODEL is fixed at creation and so is not
// alterable. The rich properties SHOW omits (search column, attributes, embedding
// model, definition, serving/indexing state, plus the mutable primary key /
// auto-suspend / request-logging / full-index-build-interval values) are read with
// DESCRIBE CORTEX SEARCH SERVICE and merged into GetObjectProperties; currently
// applied tags are read via App.GetCortexSearchServiceTags
// (INFORMATION_SCHEMA.TAG_REFERENCES). ALTER does not support RENAME and GET_DDL
// does not support cortex search services, so there is no rename or DDL-export path
// for this type.
//
// thaw:domain: Object Browser & Administration
package cortexsearchservice
