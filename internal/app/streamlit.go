// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
	"thaw/internal/streamlit"
)

// AlterStreamlit runs an ALTER STREAMLIT statement for the given app. clause is
// everything that follows the streamlit name, e.g. "SET QUERY_WAREHOUSE = MY_WH",
// "UNSET TITLE", or "RENAME TO \"DB\".\"SC\".NEW_NAME". The caller is responsible
// for correct SQL quoting inside the clause; this method only double-quotes the
// streamlit identifier.
func (a *App) AlterStreamlit(database, schema, name, clause string) error {
	return a.alterObject("STREAMLIT", database, schema, name, clause)
}

// DetectStreamlitMainFile inspects the root of a local Streamlit app folder and
// picks its entrypoint, preferring streamlit_app.py then app.py. When neither is
// present the returned MainFile is empty and the caller chooses from Candidates
// (all root-level *.py files). The deploy modal uses this to pre-fill / offer the
// main file. This does not touch Snowflake, so it works without a connection.
func (a *App) DetectStreamlitMainFile(dir string) (streamlit.MainFileResult, error) {
	return streamlit.DetectStreamlitMainFile(dir)
}

// DeployStreamlit uploads a local Streamlit app directory to a temporary
// Snowflake internal stage and creates a STREAMLIT object from it. The temporary
// stage is dropped automatically after the app is created (or on error).
//
// The upload uses PUT. PUT is always available in Thaw — the former PUT feature
// flag was removed in feature-flags v18 (issue #567), so there is no flag to gate
// on here.
func (a *App) DeployStreamlit(params snowflake.DeployStreamlitParams) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return client.DeployStreamlit(a.fctx(FeatureObjectEditor), params)
}
