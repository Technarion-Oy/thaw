// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqleditor

import "testing"

// ── PUT ───────────────────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Put(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name: "Basic PUT with file:// and @stage",
			sql:  "PUT file:///tmp/data.csv @mystage",
		},
		{
			name: "PUT with nested stage path",
			sql:  "PUT file:///tmp/data.csv @mystage/path/",
		},
		{
			name: "PUT with schema-qualified stage",
			sql:  "PUT file:///tmp/data.csv @mydb.myschema.mystage",
		},
		{
			name: "PUT with user stage",
			sql:  "PUT file:///tmp/data.csv @~/staged/",
		},
		{
			name: "PUT with OVERWRITE = TRUE",
			sql:  "PUT file:///tmp/data.csv @mystage OVERWRITE = TRUE",
		},
		{
			name: "PUT with OVERWRITE = FALSE",
			sql:  "PUT file:///tmp/data.csv @mystage OVERWRITE = FALSE",
		},
		{
			name: "PUT with AUTO_COMPRESS = TRUE",
			sql:  "PUT file:///tmp/data.csv @mystage AUTO_COMPRESS = TRUE",
		},
		{
			name: "PUT with AUTO_COMPRESS = FALSE",
			sql:  "PUT file:///tmp/data.csv @mystage AUTO_COMPRESS = FALSE",
		},
		{
			name: "PUT with valid PARALLEL",
			sql:  "PUT file:///tmp/data.csv @mystage PARALLEL = 4",
		},
		{
			name: "PUT with PARALLEL = 1 (boundary)",
			sql:  "PUT file:///tmp/data.csv @mystage PARALLEL = 1",
		},
		{
			name: "PUT with PARALLEL = 99 (boundary)",
			sql:  "PUT file:///tmp/data.csv @mystage PARALLEL = 99",
		},
		{
			name: "PUT with SOURCE_COMPRESSION = GZIP",
			sql:  "PUT file:///tmp/data.csv @mystage SOURCE_COMPRESSION = GZIP",
		},
		{
			name: "PUT with SOURCE_COMPRESSION = AUTO_DETECT",
			sql:  "PUT file:///tmp/data.csv @mystage SOURCE_COMPRESSION = AUTO_DETECT",
		},
		{
			name: "PUT with SOURCE_COMPRESSION = NONE",
			sql:  "PUT file:///tmp/data.csv @mystage SOURCE_COMPRESSION = NONE",
		},
		{
			name: "PUT with SOURCE_COMPRESSION = BROTLI",
			sql:  "PUT file:///tmp/data.csv @mystage SOURCE_COMPRESSION = BROTLI",
		},
		{
			name: "PUT with SOURCE_COMPRESSION = ZSTD",
			sql:  "PUT file:///tmp/data.csv @mystage SOURCE_COMPRESSION = ZSTD",
		},
		{
			name: "PUT with SOURCE_COMPRESSION = DEFLATE",
			sql:  "PUT file:///tmp/data.csv @mystage SOURCE_COMPRESSION = DEFLATE",
		},
		{
			name: "PUT with SOURCE_COMPRESSION = RAW_DEFLATE",
			sql:  "PUT file:///tmp/data.csv @mystage SOURCE_COMPRESSION = RAW_DEFLATE",
		},
		{
			name: "PUT with SOURCE_COMPRESSION = BZ2",
			sql:  "PUT file:///tmp/data.csv @mystage SOURCE_COMPRESSION = BZ2",
		},
		{
			name: "PUT with all valid options",
			sql:  "PUT file:///tmp/data.csv @mystage PARALLEL = 8 AUTO_COMPRESS = TRUE SOURCE_COMPRESSION = GZIP OVERWRITE = FALSE",
		},
		{
			name: "PUT lowercase — case insensitive",
			sql:  "put file:///tmp/data.csv @mystage",
		},
		{
			name: "PUT with semicolon terminator",
			sql:  "PUT file:///tmp/data.csv @mystage;",
		},
		{
			name: "PUT with Windows-style file path",
			sql:  "PUT file://C:\\Users\\data.csv @mystage",
		},

		// ── Invalid Cases ────────────────────────────────────────────────────
		{
			name:          "Bare PUT — no arguments",
			sql:           "PUT",
			expectWarning: true,
			expectedMatch: "file://",
		},
		{
			name:          "PUT with semicolon only — no arguments",
			sql:           "PUT;",
			expectWarning: true,
			expectedMatch: "file://",
		},
		{
			name:          "PUT missing file:// prefix — bare path",
			sql:           "PUT /tmp/data.csv @mystage",
			expectWarning: true,
			expectedMatch: "file://",
		},
		{
			name:          "PUT missing @stage destination",
			sql:           "PUT file:///tmp/data.csv",
			expectWarning: true,
			expectedMatch: "stage destination",
		},
		{
			name:          "PUT reversed argument order (@stage before file://)",
			sql:           "PUT @mystage file:///tmp/data.csv",
			expectWarning: true,
			expectedMatch: "wrong order",
		},
		{
			name:          "PUT PARALLEL = 0 (below minimum)",
			sql:           "PUT file:///tmp/data.csv @mystage PARALLEL = 0",
			expectWarning: true,
			expectedMatch: "PARALLEL must be a positive integer between 1 and 99",
		},
		{
			name:          "PUT PARALLEL = 100 (above maximum)",
			sql:           "PUT file:///tmp/data.csv @mystage PARALLEL = 100",
			expectWarning: true,
			expectedMatch: "PARALLEL must be a positive integer between 1 and 99",
		},
		{
			name:          "PUT PARALLEL = -1 (negative value)",
			sql:           "PUT file:///tmp/data.csv @mystage PARALLEL = -1",
			expectWarning: true,
			expectedMatch: "PARALLEL must be a positive integer between 1 and 99",
		},
		{
			name:          "PUT invalid SOURCE_COMPRESSION",
			sql:           "PUT file:///tmp/data.csv @mystage SOURCE_COMPRESSION = ZIP",
			expectWarning: true,
			expectedMatch: "Invalid SOURCE_COMPRESSION",
		},
		{
			name:          "PUT invalid SOURCE_COMPRESSION — LZ4",
			sql:           "PUT file:///tmp/data.csv @mystage SOURCE_COMPRESSION = LZ4",
			expectWarning: true,
			expectedMatch: "Invalid SOURCE_COMPRESSION",
		},
		{
			name:          "PUT OVERWRITE = YES (invalid value)",
			sql:           "PUT file:///tmp/data.csv @mystage OVERWRITE = YES",
			expectWarning: true,
			expectedMatch: "OVERWRITE must be TRUE or FALSE",
		},
		{
			name:          "PUT AUTO_COMPRESS = 1 (invalid value)",
			sql:           "PUT file:///tmp/data.csv @mystage AUTO_COMPRESS = 1",
			expectWarning: true,
			expectedMatch: "AUTO_COMPRESS must be TRUE or FALSE",
		},
	})
}

// ── GET ───────────────────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Get(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name: "Basic GET with @stage and file://",
			sql:  "GET @mystage file:///tmp/",
		},
		{
			name: "GET with nested stage path",
			sql:  "GET @mystage/path/ file:///tmp/data/",
		},
		{
			name: "GET with schema-qualified stage",
			sql:  "GET @mydb.myschema.mystage file:///tmp/",
		},
		{
			name: "GET with user stage",
			sql:  "GET @~/staged/ file:///tmp/",
		},
		{
			name: "GET with PARALLEL",
			sql:  "GET @mystage file:///tmp/ PARALLEL = 4",
		},
		{
			name: "GET with PARALLEL = 1 (boundary)",
			sql:  "GET @mystage file:///tmp/ PARALLEL = 1",
		},
		{
			name: "GET with PARALLEL = 99 (boundary)",
			sql:  "GET @mystage file:///tmp/ PARALLEL = 99",
		},
		{
			name: "GET with PATTERN",
			sql:  "GET @mystage file:///tmp/ PATTERN = '.*\\.csv'",
		},
		{
			name: "GET lowercase — case insensitive",
			sql:  "get @mystage file:///tmp/",
		},
		{
			name: "GET with semicolon terminator",
			sql:  "GET @mystage file:///tmp/;",
		},

		// ── Invalid Cases ────────────────────────────────────────────────────
		{
			name:          "Bare GET — no arguments",
			sql:           "GET",
			expectWarning: true,
			expectedMatch: "stage source",
		},
		{
			name:          "GET with semicolon only",
			sql:           "GET;",
			expectWarning: true,
			expectedMatch: "stage source",
		},
		{
			name:          "GET missing @stage source",
			sql:           "GET file:///tmp/",
			expectWarning: true,
			expectedMatch: "stage source",
		},
		{
			name:          "GET missing file:// destination",
			sql:           "GET @mystage",
			expectWarning: true,
			expectedMatch: "file://",
		},
		{
			name:          "GET missing file:// — bare path",
			sql:           "GET @mystage /tmp/",
			expectWarning: true,
			expectedMatch: "file://",
		},
		{
			name:          "GET PARALLEL = 0 (below minimum)",
			sql:           "GET @mystage file:///tmp/ PARALLEL = 0",
			expectWarning: true,
			expectedMatch: "PARALLEL must be a positive integer between 1 and 99",
		},
		{
			name:          "GET PARALLEL = 100 (above maximum)",
			sql:           "GET @mystage file:///tmp/ PARALLEL = 100",
			expectWarning: true,
			expectedMatch: "PARALLEL must be a positive integer between 1 and 99",
		},
		{
			name:          "GET PARALLEL = -1 (negative value)",
			sql:           "GET @mystage file:///tmp/ PARALLEL = -1",
			expectWarning: true,
			expectedMatch: "PARALLEL must be a positive integer between 1 and 99",
		},
		{
			// GET with reversed args still produces an actionable error because
			// the @stage check fails before the file:// check.
			name:          "GET reversed argument order (file:// before @stage)",
			sql:           "GET file:///tmp/ @mystage",
			expectWarning: true,
			expectedMatch: "stage source",
		},
	})
}

// ── LIST / LS ─────────────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_List(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name: "Basic LIST with @stage",
			sql:  "LIST @mystage",
		},
		{
			name: "LIST with nested stage path",
			sql:  "LIST @mystage/path/",
		},
		{
			name: "LIST with schema-qualified stage",
			sql:  "LIST @mydb.myschema.mystage",
		},
		{
			name: "LIST with user stage",
			sql:  "LIST @~/",
		},
		{
			name: "LIST with PATTERN",
			sql:  "LIST @mystage PATTERN = '.*\\.csv'",
		},
		{
			name: "LS alias with @stage",
			sql:  "LS @mystage",
		},
		{
			name: "LS alias with PATTERN",
			sql:  "LS @mystage PATTERN = '.*\\.csv'",
		},
		{
			name: "LIST lowercase — case insensitive",
			sql:  "list @mystage",
		},
		{
			name: "LIST with semicolon terminator",
			sql:  "LIST @mystage;",
		},

		// ── Invalid Cases ────────────────────────────────────────────────────
		{
			name:          "Bare LIST — no stage argument",
			sql:           "LIST",
			expectWarning: true,
			expectedMatch: "LIST (LS) requires a stage argument",
		},
		{
			name:          "LIST with semicolon only",
			sql:           "LIST;",
			expectWarning: true,
			expectedMatch: "LIST (LS) requires a stage argument",
		},
		{
			name:          "LIST with bare identifier (no @)",
			sql:           "LIST mystage",
			expectWarning: true,
			expectedMatch: "LIST (LS) requires a stage argument",
		},
		{
			name:          "Bare LS — no stage argument",
			sql:           "LS",
			expectWarning: true,
			expectedMatch: "LIST (LS) requires a stage argument",
		},
		{
			name:          "LS with bare identifier (no @)",
			sql:           "LS mystage",
			expectWarning: true,
			expectedMatch: "LIST (LS) requires a stage argument",
		},
	})
}

// ── REMOVE / RM ───────────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Remove(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name: "Basic REMOVE with @stage",
			sql:  "REMOVE @mystage",
		},
		{
			name: "REMOVE with nested stage path",
			sql:  "REMOVE @mystage/path/to/file.csv",
		},
		{
			name: "REMOVE with schema-qualified stage",
			sql:  "REMOVE @mydb.myschema.mystage",
		},
		{
			name: "REMOVE with user stage",
			sql:  "REMOVE @~/staged/file.csv",
		},
		{
			name: "REMOVE with PATTERN",
			sql:  "REMOVE @mystage PATTERN = '.*\\.csv'",
		},
		{
			name: "RM alias with @stage",
			sql:  "RM @mystage",
		},
		{
			name: "RM alias with PATTERN",
			sql:  "RM @mystage PATTERN = '.*\\.csv'",
		},
		{
			name: "REMOVE lowercase — case insensitive",
			sql:  "remove @mystage",
		},
		{
			name: "REMOVE with semicolon terminator",
			sql:  "REMOVE @mystage;",
		},

		// ── Invalid Cases ────────────────────────────────────────────────────
		{
			name:          "Bare REMOVE — no stage argument",
			sql:           "REMOVE",
			expectWarning: true,
			expectedMatch: "REMOVE (RM) requires a stage argument",
		},
		{
			name:          "REMOVE with semicolon only",
			sql:           "REMOVE;",
			expectWarning: true,
			expectedMatch: "REMOVE (RM) requires a stage argument",
		},
		{
			name:          "REMOVE with bare identifier (no @)",
			sql:           "REMOVE mystage",
			expectWarning: true,
			expectedMatch: "REMOVE (RM) requires a stage argument",
		},
		{
			name:          "Bare RM — no stage argument",
			sql:           "RM",
			expectWarning: true,
			expectedMatch: "REMOVE (RM) requires a stage argument",
		},
		{
			name:          "RM with bare identifier (no @)",
			sql:           "RM mystage",
			expectWarning: true,
			expectedMatch: "REMOVE (RM) requires a stage argument",
		},
	})
}
