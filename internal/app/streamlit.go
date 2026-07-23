// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"time"

	"thaw/internal/apperrors"
	"thaw/internal/streamlit"
	"thaw/internal/streamlittemplate"
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
func (a *App) DeployStreamlit(params streamlit.DeployStreamlitParams) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return streamlit.DeployStreamlit(a.fctx(FeatureObjectEditor), client, params)
}

// ListStreamlitTemplates fetches the catalog of Streamlit app templates from the
// Snowflake-Labs/snowflake-demo-streamlit repo (Apache-2.0). It never rejects on
// network/rate-limit failures: those return a Degraded catalog carrying a
// built-in fallback list, so the picker stays usable and additive. No Snowflake
// connection is required.
func (a *App) ListStreamlitTemplates() streamlittemplate.Catalog {
	ctx, cancel := context.WithTimeout(a.ctx, 25*time.Second)
	defer cancel()
	return streamlittemplate.ListTemplates(ctx)
}

// CreateStreamlitFromTemplate scaffolds the chosen template folder into destDir
// (files preserved, plus the Apache-2.0 LICENSE and a NOTICE provenance line). It
// refuses a non-empty destination. No Snowflake connection is required — the
// scaffolded folder is deployed separately via DeployStreamlit.
func (a *App) CreateStreamlitFromTemplate(templateName, destDir string) error {
	ctx, cancel := context.WithTimeout(a.ctx, 2*time.Minute)
	defer cancel()
	return streamlittemplate.DownloadTemplate(ctx, templateName, destDir)
}
