// SPDX-License-Identifier: GPL-3.0-or-later

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"thaw/internal/dbt"
)

// TestDbtProjectRoundTrip exercises the full dbt project lifecycle:
//
//  1. Create a source Snowflake database (THAW_DBT_SRC_<rand>) with two
//     tables (CUSTOMERS, ORDERS) and a view (VW_ACTIVE_ORDERS).
//  2. Scaffold a dbt project from it using [dbt.Generate].
//  3. Deploy the project against a fresh target database (THAW_DBT_TGT_<rand>)
//     via `dbt run`.
//  4. Assert that every staging view exists in the target and returns the
//     expected row count.
//
// # Prerequisites
//
//   - `dbt` must be available in PATH (install via: pip install dbt-snowflake).
//     The test is skipped gracefully when it is absent.
//   - Standard key-pair integration env vars must be set (SNOWFLAKE_ACCOUNT,
//     SNOWFLAKE_USER, SNOWFLAKE_PRIVATE_KEY, SNOWFLAKE_WAREHOUSE).
//   - The authenticated role must have CREATE DATABASE privilege.
//
// Both temporary databases are dropped via t.Cleanup even when the test fails.
func TestDbtProjectRoundTrip(t *testing.T) {
	if _, err := osexec.LookPath("dbt"); err != nil {
		t.Skip("dbt not found in PATH — install dbt-snowflake to run this test")
	}

	privKey := strings.TrimSpace(os.Getenv("SNOWFLAKE_PRIVATE_KEY"))
	if privKey == "" {
		t.Skip("SNOWFLAKE_PRIVATE_KEY not set")
	}

	client := keyPairConnFromEnv(t)

	srcDB := randomName("THAW_DBT_SRC_")
	tgtDB := randomName("THAW_DBT_TGT_")
	t.Logf("source=%s  target=%s", srcDB, tgtDB)

	// ── 1. Create source database ────────────────────────────────────────────

	mustExec(t, client, fmt.Sprintf("CREATE DATABASE %s", srcDB))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		client.Execute(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", srcDB)) //nolint:errcheck
	})

	const salesSchema = "SALES"
	mustExec(t, client, fmt.Sprintf("CREATE SCHEMA %s.%s", srcDB, salesSchema))

	mustExec(t, client, fmt.Sprintf(`
		CREATE TABLE %s.%s.CUSTOMERS (
			id         NUMBER        NOT NULL,
			name       VARCHAR(200)  NOT NULL,
			email      VARCHAR(200)  NOT NULL,
			created_at TIMESTAMP_NTZ DEFAULT CURRENT_TIMESTAMP
		)`, srcDB, salesSchema))

	mustExec(t, client, fmt.Sprintf(`
		CREATE TABLE %s.%s.ORDERS (
			id          NUMBER        NOT NULL,
			customer_id NUMBER        NOT NULL,
			total       NUMBER(10,2)  NOT NULL,
			status      VARCHAR(50)   NOT NULL DEFAULT 'pending',
			created_at  TIMESTAMP_NTZ DEFAULT CURRENT_TIMESTAMP
		)`, srcDB, salesSchema))

	mustExec(t, client, fmt.Sprintf(`
		CREATE VIEW %s.%s.VW_ACTIVE_ORDERS AS
			SELECT * FROM %s.%s.ORDERS WHERE status = 'active'`,
		srcDB, salesSchema, srcDB, salesSchema))

	// Seed rows so the staging views can be verified to pass data through.
	mustExec(t, client, fmt.Sprintf(`
		INSERT INTO %s.%s.CUSTOMERS (id, name, email) VALUES
			(1, 'Alice', 'alice@example.com'),
			(2, 'Bob',   'bob@example.com')`, srcDB, salesSchema))

	mustExec(t, client, fmt.Sprintf(`
		INSERT INTO %s.%s.ORDERS (id, customer_id, total, status) VALUES
			(101, 1,  99.99, 'active'),
			(102, 1,  49.00, 'closed'),
			(103, 2, 199.50, 'active')`, srcDB, salesSchema))

	// ── 2. Create target database ────────────────────────────────────────────

	mustExec(t, client, fmt.Sprintf("CREATE DATABASE %s", tgtDB))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		client.Execute(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", tgtDB)) //nolint:errcheck
	})

	// ── 3. Gather session metadata for profiles.yml ──────────────────────────

	sessionResult := mustQuery(t, client,
		`SELECT CURRENT_ACCOUNT(), CURRENT_USER(), CURRENT_ROLE(), CURRENT_WAREHOUSE()`)
	if len(sessionResult.Rows) == 0 {
		t.Fatal("session query returned no rows")
	}
	srow := sessionResult.Rows[0]
	session := dbt.SessionInfo{
		Account:   strings.ToLower(fmt.Sprint(srow[0])),
		User:      fmt.Sprint(srow[1]),
		Role:      fmt.Sprint(srow[2]),
		Warehouse: fmt.Sprint(srow[3]),
		Database:  tgtDB,
		Schema:    "PUBLIC",
	}

	// ── 4. Discover objects in the source schema ─────────────────────────────

	listCtx, listCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer listCancel()

	rawObjs, err := client.ListObjects(listCtx, srcDB, salesSchema)
	if err != nil {
		t.Fatalf("ListObjects(%s, %s): %v", srcDB, salesSchema, err)
	}

	var tables, views []string
	for _, o := range rawObjs {
		switch strings.ToUpper(o.Kind) {
		case "TABLE":
			tables = append(tables, o.Name)
		case "VIEW":
			views = append(views, o.Name)
		}
	}
	t.Logf("discovered tables=%v views=%v", tables, views)

	if len(tables) == 0 {
		t.Fatal("no tables discovered in source schema — setup may have failed")
	}
	if len(views) == 0 {
		t.Fatal("no views discovered in source schema — setup may have failed")
	}

	// ── 5. Scaffold the dbt project ──────────────────────────────────────────

	projectOut := t.TempDir()
	req := dbt.CreateRequest{
		ProjectName: "thaw_roundtrip",
		OutputDir:   projectOut,
		ProfileName: "thaw_roundtrip",
	}
	objects := []dbt.SchemaObjects{{
		DB:     srcDB,
		Schema: salesSchema,
		Tables: tables,
		Views:  views,
	}}

	result, err := dbt.Generate(req, session, objects)
	if err != nil {
		t.Fatalf("dbt.Generate: %v", err)
	}
	t.Logf("generated %d files under %s", len(result.FilesCreated), result.ProjectDir)
	for _, w := range result.Warnings {
		t.Logf("  warning: %s", w)
	}

	// ── 6. Write key-pair profiles.yml ───────────────────────────────────────
	// The generator writes a password-placeholder profile.  Overwrite it with
	// key-pair credentials so `dbt run` can authenticate without a password.
	// The RSA key file must persist for the lifetime of the dbt subprocess; it
	// lives inside the project dir which outlasts the test via TempDir cleanup.

	keyFile := filepath.Join(result.ProjectDir, "rsa_key.pem")
	if err := os.WriteFile(keyFile, []byte(privKey), 0600); err != nil {
		t.Fatalf("write rsa_key.pem: %v", err)
	}

	// Use SNOWFLAKE_ACCOUNT directly: it matches the format expected by the
	// dbt-snowflake adapter (e.g. "myorg-myaccount" or "xy12345.us-east-1"),
	// whereas CURRENT_ACCOUNT() may return a different representation.
	account := os.Getenv("SNOWFLAKE_ACCOUNT")

	roleYAML := ""
	if r := session.Role; r != "" && r != "<nil>" {
		roleYAML = fmt.Sprintf("      role: %s\n", r)
	}

	profiles := fmt.Sprintf(`%s:
  target: dev
  outputs:
    dev:
      type: snowflake
      account: %s
      user: %s
      private_key_path: %s
%s      warehouse: %s
      database: %s
      schema: PUBLIC
      threads: 4
      client_session_keep_alive: false
`, req.ProfileName, account, session.User, keyFile, roleYAML, session.Warehouse, tgtDB)

	profilesPath := filepath.Join(result.ProjectDir, "profiles.yml")
	if err := os.WriteFile(profilesPath, []byte(profiles), 0600); err != nil {
		t.Fatalf("write profiles.yml: %v", err)
	}

	// ── 7. Verify connection and run dbt ─────────────────────────────────────

	dbtCtx, dbtCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer dbtCancel()

	// `dbt debug` validates the profiles.yml connection before doing any work.
	// Failures here point to credential or network issues, not dbt model bugs.
	debugCmd := osexec.CommandContext(dbtCtx, "dbt", "debug",
		"--project-dir", result.ProjectDir,
		"--profiles-dir", result.ProjectDir,
	)
	debugOut, debugErr := debugCmd.CombinedOutput()
	t.Logf("dbt debug:\n%s", debugOut)
	if debugErr != nil {
		t.Fatalf("dbt debug failed — check profiles.yml credentials: %v", debugErr)
	}

	runCmd := osexec.CommandContext(dbtCtx, "dbt", "run",
		"--project-dir", result.ProjectDir,
		"--profiles-dir", result.ProjectDir,
	)
	runOut, runErr := runCmd.CombinedOutput()
	t.Logf("dbt run:\n%s", runOut)
	if runErr != nil {
		t.Fatalf("dbt run failed: %v", runErr)
	}

	// ── 8. Verify staging views in the target database ───────────────────────
	// Staging models are materialised as views (default from dbt_project.yml).
	// dbt lowercases model names; Snowflake upcases unquoted identifiers, so
	// the views appear as STG_CUSTOMERS, STG_ORDERS, STG_VW_ACTIVE_ORDERS.

	type viewCheck struct {
		name      string
		wantCount string
	}
	checks := []viewCheck{
		{"STG_CUSTOMERS", "2"},          // 2 inserted rows
		{"STG_ORDERS", "3"},             // 3 inserted rows
		{"STG_VW_ACTIVE_ORDERS", "2"},   // source view filters to 2 active rows
	}
	for _, vc := range checks {
		qr := mustQuery(t, client,
			fmt.Sprintf("SELECT COUNT(*) FROM %s.PUBLIC.%s", tgtDB, vc.name))
		if len(qr.Rows) == 0 || len(qr.Rows[0]) == 0 {
			t.Errorf("%s: COUNT(*) returned no rows", vc.name)
			continue
		}
		got := fmt.Sprint(qr.Rows[0][0])
		if got != vc.wantCount {
			t.Errorf("%s.PUBLIC.%s: COUNT(*) = %s, want %s",
				tgtDB, vc.name, got, vc.wantCount)
		} else {
			t.Logf("✓ %s.PUBLIC.%s — %s row(s)", tgtDB, vc.name, got)
		}
	}

	// Spot-check that the CUSTOMERS view exposes all expected source columns.
	colResult := mustQuery(t, client,
		fmt.Sprintf("SELECT * FROM %s.PUBLIC.STG_CUSTOMERS LIMIT 0", tgtDB))
	colSet := make(map[string]bool, len(colResult.Columns))
	for _, c := range colResult.Columns {
		colSet[strings.ToUpper(c)] = true
	}
	for _, want := range []string{"ID", "NAME", "EMAIL", "CREATED_AT"} {
		if !colSet[want] {
			t.Errorf("STG_CUSTOMERS missing column %s; got columns: %v", want, colResult.Columns)
		}
	}
}
