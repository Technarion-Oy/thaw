// SPDX-License-Identifier: GPL-3.0-or-later

package streamlit

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"thaw/internal/snowflake"
	"thaw/internal/stage"
)

// DeployStreamlitParams holds the parameters for deploying a local Streamlit app
// directory to Snowflake via a temporary internal stage.
type DeployStreamlitParams struct {
	Database      string `json:"database"`
	Schema        string `json:"schema"`
	Name          string `json:"name"`          // STREAMLIT object name in Snowflake
	CaseSensitive bool   `json:"caseSensitive"` // when true, Name is double-quoted exactly; otherwise unquoted if valid
	LocalDir      string `json:"localDir"`      // absolute local path to the Streamlit app folder to upload
	MainFile      string `json:"mainFile"`      // entrypoint relative to LocalDir (e.g. "streamlit_app.py")
	OrReplace     bool   `json:"orReplace"`
	// Optional CREATE STREAMLIT clauses
	QueryWarehouse string `json:"queryWarehouse"` // QUERY_WAREHOUSE = <warehouse>
	Title          string `json:"title"`          // TITLE = '<display title>'
	Comment        string `json:"comment"`
}

// DeployStreamlit uploads a local Streamlit app directory to a temporary internal
// stage and creates a Snowflake STREAMLIT object from it, then drops the stage.
//
//  1. CREATE TEMPORARY STAGE in the target schema
//  2. Recursively upload the app files (stage.UploadDirToStage), preserving paths
//  3. CREATE [OR REPLACE] STREAMLIT … FROM @stage MAIN_FILE = '<relpath>'
//     (BuildCreateStreamlitSql — the same builder the Create modal uses)
//  4. DROP STAGE (deferred – also fires on error)
//
// A temporary stage is sufficient because CREATE STREAMLIT copies the files once
// at creation time (modern FROM <stage> MAIN_FILE grammar), so the source need
// not persist afterwards. (Mirrors snowflake.Client.DeployNotebook.)
func DeployStreamlit(ctx context.Context, client *snowflake.Client, params DeployStreamlitParams) error {
	if strings.TrimSpace(params.LocalDir) == "" {
		return fmt.Errorf("LocalDir must be provided")
	}
	mainFile := filepath.ToSlash(strings.TrimSpace(params.MainFile))
	if mainFile == "" {
		return fmt.Errorf("MainFile must be provided")
	}

	// Create a temporary stage in the target schema.
	stageName := fmt.Sprintf("THAW_STREAMLIT_%d", time.Now().UnixNano())
	stageRef := fmt.Sprintf(`%s.%s.%s`, snowflake.QuoteIdent(params.Database), snowflake.QuoteIdent(params.Schema), stageName)
	stageAt := "@" + stageRef

	if _, err := client.Execute(ctx, "CREATE TEMPORARY STAGE "+stageRef); err != nil {
		return fmt.Errorf("create streamlit stage: %w", err)
	}
	defer client.Execute(context.Background(), "DROP STAGE IF EXISTS "+stageRef) //nolint:errcheck

	// Recursively upload the app folder to the stage, preserving relative paths.
	if err := stage.UploadDirToStage(ctx, client, params.LocalDir, stageAt, true); err != nil {
		return fmt.Errorf("upload streamlit app to stage: %w", err)
	}

	// Build the CREATE STREAMLIT statement against the temp stage and run it.
	sql, err := BuildCreateStreamlitSql(params.Database, params.Schema, deployConfig(stageAt, mainFile, params))
	if err != nil {
		return fmt.Errorf("build create streamlit: %w", err)
	}
	if _, err := client.Execute(ctx, sql); err != nil {
		return fmt.Errorf("create streamlit: %w", err)
	}
	return nil
}

// deployConfig maps the deploy params and the (already-created) temp stage
// location onto the StreamlitConfig consumed by BuildCreateStreamlitSql, so the
// deploy path emits exactly the grammar the Create modal does.
func deployConfig(stageAt, mainFile string, p DeployStreamlitParams) StreamlitConfig {
	return StreamlitConfig{
		Name:           p.Name,
		CaseSensitive:  p.CaseSensitive,
		OrReplace:      p.OrReplace,
		StageLocation:  stageAt,
		MainFile:       mainFile,
		QueryWarehouse: p.QueryWarehouse,
		Title:          p.Title,
		Comment:        p.Comment,
	}
}
