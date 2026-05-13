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

// ── BEGIN ────────────────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Begin(t *testing.T) {
	// PASS cases are paired with COMMIT to avoid block-level "unclosed transaction" warnings.
	runPatternTests(t, []patternTestCase{
		// ── PASS ─────────────────────────────────────────────────────────────
		{
			name: "Bare BEGIN",
			sql:  "BEGIN;\nCOMMIT;",
		},
		{
			name: "BEGIN WORK",
			sql:  "BEGIN WORK;\nCOMMIT;",
		},
		{
			name: "BEGIN TRANSACTION",
			sql:  "BEGIN TRANSACTION;\nCOMMIT;",
		},
		{
			name: "BEGIN TRANSACTION NAME my_txn",
			sql:  "BEGIN TRANSACTION NAME my_txn;\nCOMMIT;",
		},
		{
			name: "BEGIN NAME my_txn",
			sql:  "BEGIN NAME my_txn;\nCOMMIT;",
		},
		{
			name: "Lowercase begin work",
			sql:  "begin work;\ncommit;",
		},
		{
			name: "Mixed case Begin Transaction",
			sql:  "Begin Transaction;\nCommit;",
		},
		{
			name: "Extra whitespace",
			sql:  "BEGIN   WORK;\nCOMMIT;",
		},
		{
			name: "BEGIN with comment",
			sql:  "BEGIN /* start txn */ WORK;\nCOMMIT;",
		},
		{
			name: "BEGIN TRANSACTION NAME quoted",
			sql:  "BEGIN TRANSACTION NAME \"my_txn\";\nCOMMIT;",
		},

		// ── FAIL ─────────────────────────────────────────────────────────────
		{
			name:          "BEGIN with invalid keyword",
			sql:           "BEGIN SOMETHING;\nCOMMIT;",
			expectWarning: true,
			expectedMatch: "unexpected token",
		},
		{
			name:          "BEGIN TRANSACTION with extra tokens",
			sql:           "BEGIN TRANSACTION EXTRA;\nCOMMIT;",
			expectWarning: true,
			expectedMatch: "unexpected token",
		},
		{
			name:          "BEGIN NAME without name",
			sql:           "BEGIN NAME;\nCOMMIT;",
			expectWarning: true,
			expectedMatch: "requires a transaction name",
		},
		{
			name:          "BEGIN TRANSACTION NAME without name",
			sql:           "BEGIN TRANSACTION NAME;\nCOMMIT;",
			expectWarning: true,
			expectedMatch: "requires a transaction name",
		},
	})
}

// ── COMMIT ───────────────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Commit(t *testing.T) {
	// PASS cases are paired with BEGIN to avoid block-level "no open transaction" warnings.
	runPatternTests(t, []patternTestCase{
		// ── PASS ─────────────────────────────────────────────────────────────
		{
			name: "Bare COMMIT",
			sql:  "BEGIN;\nCOMMIT",
		},
		{
			name: "COMMIT with semicolon",
			sql:  "BEGIN;\nCOMMIT;",
		},
		{
			name: "COMMIT WORK",
			sql:  "BEGIN;\nCOMMIT WORK",
		},
		{
			name: "Lowercase commit",
			sql:  "begin;\ncommit",
		},
		{
			name: "Mixed case Commit Work",
			sql:  "Begin;\nCommit Work",
		},
		{
			name: "Extra whitespace",
			sql:  "BEGIN;\nCOMMIT   WORK",
		},
		{
			name: "COMMIT with comment",
			sql:  "BEGIN;\nCOMMIT /* done */ WORK",
		},

		// ── FAIL ─────────────────────────────────────────────────────────────
		{
			name:          "COMMIT with extra tokens",
			sql:           "BEGIN;\nCOMMIT SOMETHING",
			expectWarning: true,
			expectedMatch: "unexpected token",
		},
		{
			name:          "COMMIT WORK with extra tokens",
			sql:           "BEGIN;\nCOMMIT WORK EXTRA",
			expectWarning: true,
			expectedMatch: "unexpected token",
		},
	})
}

// ── ROLLBACK ─────────────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Rollback(t *testing.T) {
	// PASS cases are paired with BEGIN to avoid block-level "no open transaction" warnings.
	runPatternTests(t, []patternTestCase{
		// ── PASS ─────────────────────────────────────────────────────────────
		{
			name: "Bare ROLLBACK",
			sql:  "BEGIN;\nROLLBACK",
		},
		{
			name: "ROLLBACK with semicolon",
			sql:  "BEGIN;\nROLLBACK;",
		},
		{
			name: "ROLLBACK WORK",
			sql:  "BEGIN;\nROLLBACK WORK",
		},
		{
			name: "ROLLBACK TO SAVEPOINT sp1",
			sql:  "BEGIN;\nROLLBACK TO SAVEPOINT sp1",
		},
		{
			name: "ROLLBACK WORK TO SAVEPOINT sp1",
			sql:  "BEGIN;\nROLLBACK WORK TO SAVEPOINT sp1",
		},
		{
			name: "Lowercase rollback to savepoint",
			sql:  "begin;\nrollback to savepoint sp1",
		},
		{
			name: "Mixed case Rollback Work",
			sql:  "Begin;\nRollback Work",
		},
		{
			name: "Extra whitespace",
			sql:  "BEGIN;\nROLLBACK   WORK",
		},
		{
			name: "ROLLBACK with comment",
			sql:  "BEGIN;\nROLLBACK /* undo */ WORK",
		},
		{
			name: "ROLLBACK TO SAVEPOINT quoted name",
			sql:  "BEGIN;\nROLLBACK TO SAVEPOINT \"My_Save\"",
		},

		// ── FAIL ─────────────────────────────────────────────────────────────
		{
			name:          "ROLLBACK with extra tokens",
			sql:           "BEGIN;\nROLLBACK SOMETHING",
			expectWarning: true,
			expectedMatch: "unexpected token",
		},
		{
			name:          "ROLLBACK TO without SAVEPOINT",
			sql:           "BEGIN;\nROLLBACK TO sp1",
			expectWarning: true,
			expectedMatch: "ROLLBACK TO requires SAVEPOINT",
		},
		{
			name:          "ROLLBACK TO SAVEPOINT without name",
			sql:           "BEGIN;\nROLLBACK TO SAVEPOINT",
			expectWarning: true,
			expectedMatch: "requires a savepoint name",
		},
		{
			name:          "ROLLBACK WORK with extra tokens",
			sql:           "BEGIN;\nROLLBACK WORK EXTRA",
			expectWarning: true,
			expectedMatch: "unexpected token",
		},
	})
}

// ── SAVEPOINT ────────────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Savepoint(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── PASS ─────────────────────────────────────────────────────────────
		{
			name: "SAVEPOINT with name",
			sql:  "SAVEPOINT sp1",
		},
		{
			name: "SAVEPOINT with quoted name",
			sql:  `SAVEPOINT "my_save"`,
		},
		{
			name: "SAVEPOINT with semicolon",
			sql:  "SAVEPOINT sp1;",
		},
		{
			name: "Lowercase savepoint",
			sql:  "savepoint sp1",
		},
		{
			name: "Extra whitespace",
			sql:  "SAVEPOINT   sp1",
		},
		{
			name: "SAVEPOINT with comment before name",
			sql:  "SAVEPOINT /* mark */ sp1",
		},

		// ── FAIL ─────────────────────────────────────────────────────────────
		{
			name:          "Bare SAVEPOINT — no name",
			sql:           "SAVEPOINT",
			expectWarning: true,
			expectedMatch: "requires a savepoint name",
		},
		{
			name:          "SAVEPOINT with only semicolon",
			sql:           "SAVEPOINT;",
			expectWarning: true,
			expectedMatch: "requires a savepoint name",
		},
		{
			name:          "SAVEPOINT with space then semicolon",
			sql:           "SAVEPOINT ;",
			expectWarning: true,
			expectedMatch: "requires a savepoint name",
		},
	})
}

// ── RELEASE SAVEPOINT ────────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_ReleaseSavepoint(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── PASS ─────────────────────────────────────────────────────────────
		{
			name: "RELEASE SAVEPOINT with name",
			sql:  "RELEASE SAVEPOINT sp1",
		},
		{
			name: "RELEASE SAVEPOINT quoted name",
			sql:  `RELEASE SAVEPOINT "my_save"`,
		},
		{
			name: "RELEASE SAVEPOINT with semicolon",
			sql:  "RELEASE SAVEPOINT sp1;",
		},
		{
			name: "Lowercase release savepoint",
			sql:  "release savepoint sp1",
		},
		{
			name: "Extra whitespace",
			sql:  "RELEASE   SAVEPOINT   sp1",
		},
		{
			name: "RELEASE SAVEPOINT with comment",
			sql:  "RELEASE SAVEPOINT /* done */ sp1",
		},

		// ── FAIL ─────────────────────────────────────────────────────────────
		{
			name:          "Bare RELEASE SAVEPOINT — no name",
			sql:           "RELEASE SAVEPOINT",
			expectWarning: true,
			expectedMatch: "requires a savepoint name",
		},
		{
			name:          "RELEASE SAVEPOINT with only semicolon",
			sql:           "RELEASE SAVEPOINT;",
			expectWarning: true,
			expectedMatch: "requires a savepoint name",
		},
		{
			name:          "RELEASE SAVEPOINT with space then semicolon",
			sql:           "RELEASE SAVEPOINT ;",
			expectWarning: true,
			expectedMatch: "requires a savepoint name",
		},
	})
}

// ── Block-level transaction tracking ─────────────────────────────────────────

func TestValidateSnowflakePatterns_TransactionBlocks(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── PASS — matched pairs ─────────────────────────────────────────────
		{
			name: "BEGIN + COMMIT pair",
			sql:  "BEGIN;\nSELECT 1;\nCOMMIT;",
		},
		{
			name: "BEGIN + ROLLBACK pair",
			sql:  "BEGIN;\nSELECT 1;\nROLLBACK;",
		},
		{
			name: "BEGIN TRANSACTION + COMMIT pair",
			sql:  "BEGIN TRANSACTION;\nSELECT 1;\nCOMMIT;",
		},
		{
			name: "BEGIN WORK + COMMIT WORK pair",
			sql:  "BEGIN WORK;\nSELECT 1;\nCOMMIT WORK;",
		},

		// ── FAIL — unmatched pairs ───────────────────────────────────────────
		{
			name:          "Unclosed BEGIN — no COMMIT or ROLLBACK",
			sql:           "BEGIN;\nSELECT 1;",
			expectWarning: true,
			expectedMatch: "not committed or rolled back",
		},
		{
			name:          "COMMIT without BEGIN",
			sql:           "SELECT 1;\nCOMMIT;",
			expectWarning: true,
			expectedMatch: "no open transaction",
		},
		{
			name:          "ROLLBACK without BEGIN",
			sql:           "SELECT 1;\nROLLBACK;",
			expectWarning: true,
			expectedMatch: "no open transaction",
		},
		{
			name:          "Nested BEGIN — second BEGIN inside open transaction",
			sql:           "BEGIN;\nBEGIN;\nCOMMIT;\nCOMMIT;",
			expectWarning: true,
			expectedMatch: "nested BEGIN",
		},

		// ── PASS — scripting inside $$ should not count ──────────────────────
		{
			name: "BEGIN inside dollar-quoted procedure body — not a transaction",
			sql:  "CREATE PROCEDURE foo() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;",
		},
	})
}
