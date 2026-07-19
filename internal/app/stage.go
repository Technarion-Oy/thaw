// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"
	"strings"
	"thaw/internal/apperrors"
	"thaw/internal/fileformat"
	"thaw/internal/snowflake"
	"thaw/internal/stage"
)

// ListStageEntries returns directory-aware entries within an internal named stage.
func (a *App) ListStageEntries(database, schema, stageName, dirPath string) ([]snowflake.GitRepoEntry, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListStageEntries(a.fctx(FeatureStages), database, schema, stageName, dirPath)
}

// ListStages returns the stages in a schema with their INTERNAL/EXTERNAL type,
// so callers can filter (e.g. external tables may only reference an EXTERNAL stage).
func (a *App) ListStages(database, schema string) ([]snowflake.StageSummary, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListStages(a.fctx(FeatureStages), database, schema)
}

// ListWorkspaces returns all workspaces visible to the current user.
func (a *App) ListWorkspaces() ([]snowflake.WorkspaceInfo, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListWorkspaces(a.fctx(FeatureStages))
}

// ListWorkspaceEntries returns directory-aware entries within a workspace.
func (a *App) ListWorkspaceEntries(database, schema, workspaceName, dirPath string) ([]snowflake.GitRepoEntry, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListWorkspaceEntries(a.fctx(FeatureStages), database, schema, workspaceName, dirPath)
}

// ListStageFiles returns the list of files on a Snowflake stage.
func (a *App) ListStageFiles(stageName string, pattern string) ([]stage.StageFile, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return stage.ListStageFiles(a.fctx(FeatureStages), client, stageName, pattern)
}

// UploadFileToStage executes a PUT command to upload a local file to an internal stage.
func (a *App) UploadFileToStage(localPath string, stageName string, parallel int, autoCompress bool, sourceCompression string, overwrite bool) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}

	return stage.UploadFileToStage(a.fctx(FeatureStages), client, localPath, stageName, parallel, autoCompress, sourceCompression, overwrite)
}

// DownloadFileFromStage executes a GET command to download files from an internal stage to a local directory.
func (a *App) DownloadFileFromStage(stageName string, localDirPath string, parallel int, pattern string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}

	return stage.DownloadFileFromStage(a.fctx(FeatureStages), client, stageName, localDirPath, parallel, pattern)
}

// RemoveStageFiles deletes files from a stage using the REMOVE command.
func (a *App) RemoveStageFiles(stageName string, pattern string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}

	return stage.RemoveStageFiles(a.fctx(FeatureStages), client, stageName, pattern)
}

// GetLocalFilePreview reads a local file and returns up to 50 rows.
// It uses pure Go to mimic Snowflake's native file format parsing for CSV and JSON.
func (a *App) GetLocalFilePreview(path string, cfg fileformat.FileFormatConfig) fileformat.PreviewResult {
	return fileformat.PreviewLocalFile(path, cfg)
}

// GetStageFilePreview queries a Snowflake stage file with an inline FILE_FORMAT
// derived from cfg and returns up to 50 rows. The stagePath must be a fully
// qualified stage reference, e.g. "@DB.SCHEMA.STAGE/path/to/file.csv".
func (a *App) GetStageFilePreview(stagePath string, cfg fileformat.FileFormatConfig) (fileformat.PreviewResult, error) {
	client := a.currentClient()
	if client == nil {
		return fileformat.PreviewResult{}, apperrors.ErrNotConnected
	}
	return fileformat.PreviewStageFile(a.fctx(FeatureStages), client, stagePath, cfg)
}

// ExecuteStageFile executes a SQL file from an internal named stage.
// Only .sql files are accepted; the frontend gates this too, but we validate server-side for defense-in-depth.
func (a *App) ExecuteStageFile(database, schema, stageName, filePath string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	if !strings.HasSuffix(strings.ToLower(filePath), ".sql") {
		return fmt.Errorf("only .sql files can be executed, got %q", filePath)
	}
	return client.ExecuteGitFile(a.fctx(FeatureStages), database, schema, stageName, filePath) // SQL pattern is identical: EXECUTE IMMEDIATE FROM @db.schema.name/path
}

// AlterStage runs an ALTER STAGE IF EXISTS statement on the given stage.
// clause is everything that follows the stage name in the ALTER statement.
func (a *App) AlterStage(database, schema, name, clause string) error {
	return a.alterObject("STAGE IF EXISTS", database, schema, name, clause)
}
