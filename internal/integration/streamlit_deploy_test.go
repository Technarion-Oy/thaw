// SPDX-License-Identifier: GPL-3.0-or-later

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"thaw/internal/snowflake"
	"thaw/internal/streamlit"
)

// TestDeployStreamlit exercises the local-folder → STREAMLIT deploy path end to
// end against a live account: it writes a small multi-file app to a temp folder
// (including a subdirectory and junk that must be skipped), deploys it, verifies
// the object and its main file, then redeploys with OR REPLACE to cover the
// update-existing path. This is the smoke-test the unit tests can't replace — the
// recursive PUT, the temp-stage lifecycle, and CREATE STREAMLIT only fail at
// execution time.
//
// Requires the standard integration env vars (see export_test.go). Run with:
//
//	go test -v -tags integration -timeout 15m -run TestDeployStreamlit ./internal/integration/
func TestDeployStreamlit(t *testing.T) {
	client := connFromEnv(t)
	ctx := context.Background()

	dbName := randomName("THAW_TEST_")
	t.Logf("test database: %s", dbName)
	mustExec(t, client, fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := client.Execute(c, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)); err != nil {
			t.Logf("cleanup: drop %s: %v (manual cleanup may be required)", dbName, err)
		}
	})
	mustExec(t, client, fmt.Sprintf(`CREATE SCHEMA "%s"."PUBLIC"`, dbName))

	// A small app: a root entrypoint + a helper module, a pages/ subdirectory, an
	// environment.yml, and junk (.git/, __pycache__/, .DS_Store) that the upload
	// planner must skip.
	appDir := t.TempDir()
	writeAppFile(t, appDir, "streamlit_app.py", "import streamlit as st\nst.write('hello')\n")
	writeAppFile(t, appDir, "helpers.py", "def greet():\n    return 'hi'\n")
	writeAppFile(t, appDir, "environment.yml", "name: sf_env\ndependencies:\n  - streamlit\n")
	writeAppFile(t, appDir, "pages/page_1.py", "import streamlit as st\nst.write('page 1')\n")
	writeAppFile(t, appDir, ".DS_Store", "junk")
	writeAppFile(t, appDir, ".git/config", "[core]\n")
	writeAppFile(t, appDir, "__pycache__/helpers.cpython-310.pyc", "bytecode")

	// Main-file detection should pick the conventional entrypoint.
	detected, err := streamlit.DetectStreamlitMainFile(appDir)
	if err != nil {
		t.Fatalf("DetectStreamlitMainFile: %v", err)
	}
	if detected.MainFile != "streamlit_app.py" {
		t.Fatalf("detected main file = %q, want streamlit_app.py", detected.MainFile)
	}

	appName := randomName("APP_")
	params := snowflake.DeployStreamlitParams{
		Database: dbName,
		Schema:   "PUBLIC",
		Name:     appName,
		LocalDir: appDir,
		MainFile: detected.MainFile,
		Title:    "Thaw Smoke Test",
		Comment:  "deployed by TestDeployStreamlit",
	}

	deployCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := client.DeployStreamlit(deployCtx, params); err != nil {
		t.Fatalf("DeployStreamlit: %v", err)
	}

	fqn := fmt.Sprintf(`"%s"."PUBLIC"."%s"`, dbName, appName)

	// The object exists and DESCRIBE reports the main file we deployed.
	assertStreamlitMainFile(t, client, fqn, "streamlit_app.py")

	// Redeploy the same app with OR REPLACE (the update-existing path). Snapshot
	// semantics mean CREATE OR REPLACE is how a running app is refreshed.
	writeAppFile(t, appDir, "streamlit_app.py", "import streamlit as st\nst.write('hello v2')\n")
	params.OrReplace = true
	redeployCtx, cancel2 := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel2()
	if err := client.DeployStreamlit(redeployCtx, params); err != nil {
		t.Fatalf("DeployStreamlit (redeploy): %v", err)
	}
	assertStreamlitMainFile(t, client, fqn, "streamlit_app.py")
}

// writeAppFile writes content to appDir/rel, creating parent directories.
func writeAppFile(t *testing.T, appDir, rel, content string) {
	t.Helper()
	p := filepath.Join(appDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// assertStreamlitMainFile runs DESCRIBE STREAMLIT and checks the main_file value.
func assertStreamlitMainFile(t *testing.T, client *snowflake.Client, fqn, wantMain string) {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := client.Execute(c, fmt.Sprintf(`DESCRIBE STREAMLIT %s`, fqn))
	if err != nil {
		t.Fatalf("DESCRIBE STREAMLIT %s: %v", fqn, err)
	}
	idx := -1
	for i, col := range res.Columns {
		if strings.EqualFold(col, "main_file") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("DESCRIBE STREAMLIT has no main_file column (got %v)", res.Columns)
	}
	if len(res.Rows) == 0 {
		t.Fatalf("DESCRIBE STREAMLIT %s returned no rows", fqn)
	}
	got := strings.TrimSpace(snowflake.StrVal(res.Rows[0], idx))
	if got != wantMain {
		t.Errorf("main_file = %q, want %q", got, wantMain)
	}
}
