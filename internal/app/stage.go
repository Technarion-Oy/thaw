// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListStageEntries(a.ctx, database, schema, stageName, dirPath)
}

// ListWorkspaces returns all workspaces visible to the current user.
func (a *App) ListWorkspaces() ([]snowflake.WorkspaceInfo, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListWorkspaces(a.ctx)
}

// ListWorkspaceEntries returns directory-aware entries within a workspace.
func (a *App) ListWorkspaceEntries(database, schema, workspaceName, dirPath string) ([]snowflake.GitRepoEntry, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListWorkspaceEntries(a.ctx, database, schema, workspaceName, dirPath)
}

// ListStageFiles returns the list of files on a Snowflake stage.
func (a *App) ListStageFiles(stageName string, pattern string) ([]stage.StageFile, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return stage.ListStageFiles(a.ctx, a.client, stageName, pattern)
}

// UploadFileToStage executes a PUT command to upload a local file to an internal stage.
func (a *App) UploadFileToStage(localPath string, stageName string, parallel int, autoCompress bool, sourceCompression string, overwrite bool) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}

	flags := loadUserFeatureFlags()
	if !flags.PutCommand {
		return fmt.Errorf("PUT commands are disabled. Enable them under View → Enabled Features…")
	}

	return stage.UploadFileToStage(a.ctx, a.client, localPath, stageName, parallel, autoCompress, sourceCompression, overwrite)
}

// DownloadFileFromStage executes a GET command to download files from an internal stage to a local directory.
func (a *App) DownloadFileFromStage(stageName string, localDirPath string, parallel int, pattern string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}

	flags := loadUserFeatureFlags()
	if !flags.GetCommand {
		return fmt.Errorf("GET commands are disabled. Enable them under View → Enabled Features…")
	}

	return stage.DownloadFileFromStage(a.ctx, a.client, stageName, localDirPath, parallel, pattern)
}

// RemoveStageFiles deletes files from a stage using the REMOVE command.
func (a *App) RemoveStageFiles(stageName string, pattern string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}

	flags := loadUserFeatureFlags()
	if !flags.RemoveCommand {
		return fmt.Errorf("REMOVE commands are disabled. Enable them under View → Enabled Features…")
	}

	return stage.RemoveStageFiles(a.ctx, a.client, stageName, pattern)
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
	if a.client == nil {
		return fileformat.PreviewResult{}, apperrors.ErrNotConnected
	}
	return fileformat.PreviewStageFile(a.ctx, a.client, stagePath, cfg)
}

// ExecuteStageFile executes a SQL file from an internal named stage.
// Only .sql files are accepted; the frontend gates this too, but we validate server-side for defense-in-depth.
func (a *App) ExecuteStageFile(database, schema, stageName, filePath string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	if !strings.HasSuffix(strings.ToLower(filePath), ".sql") {
		return fmt.Errorf("only .sql files can be executed, got %q", filePath)
	}
	return a.client.ExecuteGitFile(a.ctx, database, schema, stageName, filePath) // SQL pattern is identical: EXECUTE IMMEDIATE FROM @db.schema.name/path
}

// AlterStage runs an ALTER STAGE IF EXISTS statement on the given stage.
// clause is everything that follows the stage name in the ALTER statement.
func (a *App) AlterStage(database, schema, name, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER STAGE IF EXISTS %s.%s.%s %s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}
