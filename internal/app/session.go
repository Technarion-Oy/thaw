// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"
	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// GetClientVersionInfo returns Snowflake's supported / recommended client &
// driver versions by running SELECT SYSTEM$CLIENT_VERSION_INFO(). General
// account-level info exposed for reuse — any feature needing client version data
// (e.g. the authentication-policy CLIENT_POLICY editor) can call it.
func (a *App) GetClientVersionInfo() ([]snowflake.ClientVersionInfo, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.GetClientVersionInfo(a.fctx(FeatureSessionSetup))
}

// GetSessionContext returns the currently active role, warehouse, database and
// schema for the given tab's isolated session.
// Fast path: if the tab session hasn't been created yet but the shared client
// is available (i.e. immediately after Connect()), use client to avoid
// triggering a full NewClient re-login just to read session variables.
func (a *App) GetSessionContext(tabId string) (snowflake.SessionContext, error) {
	if _, ok := a.tabSessions.Load(tabId); !ok {
		// Return cached context from an evicted session if available.
		if val, ok := a.evictedContexts.Load(tabId); ok {
			return val.(snowflake.SessionContext), nil
		}
		if client := a.currentClient(); client != nil {
			return client.GetSessionContext(a.fctx(FeatureSessionSetup))
		}
		return snowflake.SessionContext{}, apperrors.ErrNotConnected
	}
	ts, err := a.getOrInitTabSession(tabId)
	if err != nil {
		return snowflake.SessionContext{}, err
	}
	ts.inUse.Add(1)
	defer ts.inUse.Add(-1)
	return ts.client.GetSessionContext(a.fctx(FeatureSessionSetup))
}

// GetTabSessionID returns the Snowflake session ID for the given tab.
// Returns an empty string (no error) when the tab has no active session.
func (a *App) GetTabSessionID(tabId string) (string, error) {
	val, ok := a.tabSessions.Load(tabId)
	if !ok {
		return "", nil
	}
	ts := val.(*tabSession)
	ts.inUse.Add(1)
	defer ts.inUse.Add(-1)
	return ts.client.GetSessionID(a.fctx(FeatureSessionSetup))
}

// GetQuotedIdentifiersIgnoreCase returns true when the current session's
// QUOTED_IDENTIFIERS_IGNORE_CASE parameter is TRUE, meaning Snowflake treats
// quoted identifiers as case-insensitive (double-quoting does not preserve
// case). The frontend uses this to warn users when creating objects.
func (a *App) GetQuotedIdentifiersIgnoreCase() (bool, error) {
	client := a.currentClient()
	if client == nil {
		return false, apperrors.ErrNotConnected
	}
	return client.GetQuotedIdentifiersIgnoreCase(a.fctx(FeatureSessionSetup))
}

// ListRoles returns all roles visible to the current role (SHOW ROLES).
// Used for informational displays and user-management role pickers.
func (a *App) ListRoles() ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListRoles(a.fctx(FeatureSessionSetup))
}

// ListAvailableRoles returns only the roles the current user can switch to
// (CURRENT_AVAILABLE_ROLES). Used for the role-selection toolbar dropdown.
func (a *App) ListAvailableRoles() ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListAvailableRoles(a.fctx(FeatureSessionSetup))
}

// ListWarehouses returns all warehouses visible to the current role.
func (a *App) ListWarehouses() ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListWarehouses(a.fctx(FeatureSessionSetup))
}

// ListComputePools returns all compute pools visible to the current role. Used
// by the Create Service modal to populate the compute-pool picker.
func (a *App) ListComputePools() ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListComputePools(a.fctx(FeatureSessionSetup))
}

// UseRole switches the given tab's isolated session to the specified role.
func (a *App) UseRole(tabId string, role string) error {
	ts, err := a.getOrInitTabSession(tabId)
	if err != nil {
		return err
	}
	ts.inUse.Add(1)
	defer ts.inUse.Add(-1)
	return ts.client.UseRole(a.fctx(FeatureSessionSetup), role)
}

// UseWarehouse switches the given tab's isolated session to the specified warehouse.
func (a *App) UseWarehouse(tabId string, warehouse string) error {
	ts, err := a.getOrInitTabSession(tabId)
	if err != nil {
		return err
	}
	ts.inUse.Add(1)
	defer ts.inUse.Add(-1)
	return ts.client.UseWarehouse(a.fctx(FeatureSessionSetup), warehouse)
}

// UseDatabase switches the given tab's isolated session to the specified database.
func (a *App) UseDatabase(tabId string, database string) error {
	ts, err := a.getOrInitTabSession(tabId)
	if err != nil {
		return err
	}
	ts.inUse.Add(1)
	defer ts.inUse.Add(-1)
	return ts.client.UseDatabase(a.fctx(FeatureSessionSetup), database)
}

// UseSchema switches the given tab's isolated session to the specified schema.
func (a *App) UseSchema(tabId string, schema string) error {
	ts, err := a.getOrInitTabSession(tabId)
	if err != nil {
		return err
	}
	ts.inUse.Add(1)
	defer ts.inUse.Add(-1)
	return ts.client.UseSchema(a.fctx(FeatureSessionSetup), schema)
}

// GetCurrentRegion returns the result of SELECT CURRENT_REGION(), which
// encodes both the cloud provider and the deployment region, e.g.
// "AWS_US_EAST_1", "AZURE_EASTUS2", or "GCP_US_CENTRAL1".
func (a *App) GetCurrentRegion() (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	qr, err := client.Execute(a.fctx(FeatureSessionSetup), `SELECT CURRENT_REGION()`)
	if err != nil {
		return "", err
	}
	if len(qr.Rows) > 0 && len(qr.Rows[0]) > 0 && qr.Rows[0][0] != nil {
		return fmt.Sprint(qr.Rows[0][0]), nil
	}
	return "", nil
}

// GetSnowsightURL returns the Snowsight URL for the current account in the
// canonical new-style form https://app.snowflake.com/<org>/<account> using
// CURRENT_ORGANIZATION_NAME() and CURRENT_ACCOUNT_NAME(). The older
// https://<org>-<account>.snowflakecomputing.com host simply redirects here, so
// emitting the new form directly avoids the redirect hop and yields URLs that
// concatenate cleanly with Snowsight route fragments (e.g. the Streamlit
// deep-link "/#/streamlit-apps/<DB>.<SCHEMA>.<NAME>"). No trailing slash is
// appended so callers can suffix a "/#/…" fragment without doubling the slash.
func (a *App) GetSnowsightURL() (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	qr, err := client.Execute(a.fctx(FeatureSessionSetup), `SELECT 'https://app.snowflake.com/' || LOWER(CURRENT_ORGANIZATION_NAME()) || '/' || LOWER(CURRENT_ACCOUNT_NAME())`)
	if err != nil {
		return "", err
	}
	if len(qr.Rows) > 0 && len(qr.Rows[0]) > 0 && qr.Rows[0][0] != nil {
		return fmt.Sprint(qr.Rows[0][0]), nil
	}
	return "", nil
}

// GetCurrentUser returns the result of SELECT CURRENT_USER(), which reflects
// the canonical Snowflake username exactly as stored (preserving case). Cached
// for the connection's lifetime — the user is constant, so repeated callers
// (e.g. each Query Activity modal open) don't re-run the round-trip.
func (a *App) GetCurrentUser() (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	return client.GetCurrentUserCached(a.fctx(FeatureSessionSetup))
}

// GetSessionParameters returns the current session parameters from SHOW PARAMETERS IN SESSION.
func (a *App) GetSessionParameters() ([]snowflake.SessionParam, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.GetSessionParameters(a.fctx(FeatureSessionSetup))
}

// GetSessionVariables returns the current session variables from SHOW VARIABLES.
func (a *App) GetSessionVariables() ([]snowflake.SessionVar, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.GetSessionVariables(a.fctx(FeatureSessionSetup))
}

// SetSessionParameter applies ALTER SESSION SET key = value for the given parameter.
func (a *App) SetSessionParameter(name, value, paramType string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	valExpr := snowflake.QuoteSessionParamValue(value, paramType)
	_, err := client.Execute(a.fctx(FeatureSessionSetup), "ALTER SESSION SET "+name+" = "+valExpr)
	return err
}

// SetSessionVariable applies SET name = value for the given session variable.
func (a *App) SetSessionVariable(name, value, varType string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	valExpr := snowflake.QuoteSessionParamValue(value, varType)
	_, err := client.Execute(a.fctx(FeatureSessionSetup), "SET "+name+" = "+valExpr)
	return err
}
