// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"
	"strings"
	"thaw/internal/apperrors"
	"thaw/internal/integrations"
	"thaw/internal/snowflake"
	"time"
)

// ListSecurityIntegrations returns all security integrations.
func (a *App) ListSecurityIntegrations() ([]snowflake.SecurityIntegration, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListSecurityIntegrations(a.fctx(FeatureIntegrations))
}

// ListApiIntegrations returns all API integrations visible to the current role.
func (a *App) ListApiIntegrations() ([]snowflake.ApiIntegration, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListApiIntegrations(a.fctx(FeatureIntegrations))
}

// ListSecretsInAccount returns all secrets visible to the current role across the account.
func (a *App) ListSecretsInAccount() ([]snowflake.AccountSecret, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListSecretsInAccount(a.fctx(FeatureIntegrations))
}

// ListExternalAccessIntegrations returns all EXTERNAL ACCESS integrations.
func (a *App) ListExternalAccessIntegrations() ([]snowflake.IntegrationRow, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListIntegrations(a.fctx(FeatureIntegrations), "EXTERNAL ACCESS")
}

// ListNotificationIntegrations returns the names of all notification integrations.
func (a *App) ListNotificationIntegrations() ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListNotificationIntegrations(a.fctx(FeatureIntegrations))
}

// ListExternalVolumes returns the names of all external volumes visible to the current role.
func (a *App) ListExternalVolumes() ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListExternalVolumes(a.fctx(FeatureIntegrations))
}

// ListIntegrations runs SHOW <kind> INTEGRATIONS and returns structured rows.
// kind may be "STORAGE", "API", "CATALOG", "EXTERNAL ACCESS", "NOTIFICATION", or "SECURITY".
func (a *App) ListIntegrations(kind string) ([]snowflake.IntegrationRow, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListIntegrations(a.fctx(FeatureIntegrations), kind)
}

// GetIntegrationProperties runs DESCRIBE INTEGRATION for the named integration
// and returns the result as key/value pairs.
func (a *App) GetIntegrationProperties(name string) ([]snowflake.PropertyPair, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	esc := strings.ReplaceAll(name, `"`, `""`)
	res, err := client.Execute(a.fctx(FeatureIntegrations), fmt.Sprintf(`DESCRIBE INTEGRATION "%s"`, esc))
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return []snowflake.PropertyPair{}, nil
	}
	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprintf("%v", t)
		}
	}
	// DESCRIBE INTEGRATION returns rows of (property, property_type, property_value, property_default)
	// We return property / property_value pairs.
	var pairs []snowflake.PropertyPair
	for _, row := range res.Rows {
		if len(row) < 3 {
			continue
		}
		k := toString(row[0])
		v := toString(row[2])
		if k != "" {
			pairs = append(pairs, snowflake.PropertyPair{Key: k, Value: v})
		}
	}
	return pairs, nil
}

// DropIntegration drops the named integration.
func (a *App) DropIntegration(name string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return client.DropIntegration(a.fctx(FeatureIntegrations), name)
}

// CreateStorageIntegration builds and executes a CREATE STORAGE INTEGRATION DDL.
func (a *App) CreateStorageIntegration(params integrations.StorageIntegrationParams) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql, err := integrations.BuildStorageIntegrationSQL(params)
	if err != nil {
		return err
	}
	return client.ExecDDL(a.fctx(FeatureIntegrations), sql)
}

// CreateApiIntegration builds and executes a CREATE API INTEGRATION DDL.
func (a *App) CreateApiIntegration(params integrations.ApiIntegrationParams) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql, err := integrations.BuildApiIntegrationSQL(params)
	if err != nil {
		return err
	}
	return client.ExecDDL(a.fctx(FeatureIntegrations), sql)
}

// CreateCatalogIntegration builds and executes a CREATE CATALOG INTEGRATION DDL.
func (a *App) CreateCatalogIntegration(params integrations.CatalogIntegrationParams) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql, err := integrations.BuildCatalogIntegrationSQL(params)
	if err != nil {
		return err
	}
	return client.ExecDDL(a.fctx(FeatureIntegrations), sql)
}

// CreateExternalAccessIntegration builds and executes a CREATE EXTERNAL ACCESS INTEGRATION DDL.
func (a *App) CreateExternalAccessIntegration(params integrations.ExternalAccessIntegrationParams) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql, err := integrations.BuildExternalAccessIntegrationSQL(params)
	if err != nil {
		return err
	}
	return client.ExecDDL(a.fctx(FeatureIntegrations), sql)
}

// CreateNotificationIntegration builds and executes a CREATE NOTIFICATION INTEGRATION DDL.
func (a *App) CreateNotificationIntegration(params integrations.NotificationIntegrationParams) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql, err := integrations.BuildNotificationIntegrationSQL(params)
	if err != nil {
		return err
	}
	return client.ExecDDL(a.fctx(FeatureIntegrations), sql)
}

// CreateSecurityIntegration builds and executes a CREATE SECURITY INTEGRATION DDL.
func (a *App) CreateSecurityIntegration(params integrations.SecurityIntegrationParams) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql, err := integrations.BuildSecurityIntegrationSQL(params)
	if err != nil {
		return err
	}
	return client.ExecDDL(a.fctx(FeatureIntegrations), sql)
}

// BuildApiIntegrationPreviewSQL returns the DDL that would be executed for the
// given API integration parameters, without executing it. Used for live SQL preview.
func (a *App) BuildApiIntegrationPreviewSQL(params integrations.ApiIntegrationParams) (string, error) {
	return integrations.BuildApiIntegrationSQL(params)
}
