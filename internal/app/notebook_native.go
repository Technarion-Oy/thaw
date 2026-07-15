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
	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// ExecuteNotebook runs EXECUTE NOTEBOOK against a Snowflake Notebook object and
// returns the resulting query ID. Each element of params is treated as a string
// literal value and is automatically single-quoted in the generated SQL.
func (a *App) ExecuteNotebook(database, schema, name string, params []string) (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	return client.ExecuteNotebook(a.fctx(FeatureNotebooks), database, schema, name, params)
}

// GetNotebookQueryWarehouse returns the QUERY_WAREHOUSE currently configured on
// the given Snowflake Notebook, or an empty string if none is set.
func (a *App) GetNotebookQueryWarehouse(database, schema, name string) (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	return client.GetNotebookQueryWarehouse(a.fctx(FeatureNotebooks), database, schema, name)
}

// SetNotebookQueryWarehouse updates the QUERY_WAREHOUSE property of the given
// Snowflake Notebook via ALTER NOTEBOOK … SET QUERY_WAREHOUSE.
func (a *App) SetNotebookQueryWarehouse(database, schema, name, warehouse string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return client.SetNotebookQueryWarehouse(a.fctx(FeatureNotebooks), database, schema, name, warehouse)
}

// MakeNotebookLive promotes the latest saved version of the notebook to the
// live version via ALTER NOTEBOOK … ADD LIVE VERSION FROM LAST.
func (a *App) MakeNotebookLive(database, schema, name string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER NOTEBOOK %s.%s.%s ADD LIVE VERSION FROM LAST", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	_, err := client.Execute(a.fctx(FeatureNotebooks), sql)
	return err
}

// FetchNotebookContent retrieves the content of a Snowflake Notebook object.
// It describes the notebook to find its stage URI, downloads the .ipynb file
// to a temporary local directory, reads the file, and returns the nbformat JSON.
// The temporary directory is cleaned up automatically.
func (a *App) FetchNotebookContent(database, schema, name string) (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	return client.FetchNotebookContent(a.fctx(FeatureNotebooks), database, schema, name)
}

// DeployNotebook uploads a local .ipynb file to a temporary Snowflake internal
// stage and creates a NOTEBOOK object from it. The temporary stage is dropped
// automatically after the notebook is created (or on error).
func (a *App) DeployNotebook(params snowflake.DeployNotebookParams) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return client.DeployNotebook(a.fctx(FeatureNotebooks), params)
}
