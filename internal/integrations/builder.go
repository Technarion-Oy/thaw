// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package integrations builds Snowflake CREATE INTEGRATION DDL statements.
// All parameter values are validated and escaped before being embedded in SQL
// to prevent SQL injection.
//
// thaw:domain: Object Browser & Administration
package integrations

import (
	"fmt"
	"regexp"
	"strings"

	"thaw/internal/snowflake"
)

// ── Parameter structs ─────────────────────────────────────────────────────────

// StorageIntegrationParams holds form values for CREATE STORAGE INTEGRATION.
type StorageIntegrationParams struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	Enabled       bool   `json:"enabled"`
	Provider      string `json:"provider"` // S3, S3GOV, GCS, AZURE

	// S3 / S3GOV
	AwsRoleArn       string `json:"awsRoleArn"`
	AwsExternalId    string `json:"awsExternalId"`
	AllowedLocations string `json:"allowedLocations"` // newline/comma-separated
	BlockedLocations string `json:"blockedLocations"`
	UsePrivatelink   bool   `json:"usePrivatelink"`

	// GCS — uses AllowedLocations / BlockedLocations

	// AZURE
	AzureTenantId string `json:"azureTenantId"`
	// uses AllowedLocations / BlockedLocations / UsePrivatelink

	Comment string `json:"comment"`
}

// ApiIntegrationParams holds form values for CREATE API INTEGRATION.
type ApiIntegrationParams struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	Enabled       bool   `json:"enabled"`
	// Provider is the lowercase Snowflake API_PROVIDER value.
	Provider string `json:"provider"` // aws_api_gateway, aws_private_api_gateway, azure_api_management, google_api_gateway, git_https_api

	// OR REPLACE / IF NOT EXISTS (only honored for git_https_api)
	OrReplace   bool `json:"orReplace"`
	IfNotExists bool `json:"ifNotExists"`

	// Common prefixes
	AllowedPrefixes string `json:"allowedPrefixes"` // newline/comma-separated
	BlockedPrefixes string `json:"blockedPrefixes"`

	// AWS providers
	AwsRoleArn string `json:"awsRoleArn"`
	ApiKey     string `json:"apiKey"`

	// Azure
	AzureTenantId string `json:"azureTenantId"`
	AzureAdAppId  string `json:"azureAdAppId"`

	// Google
	GoogleAudience string `json:"googleAudience"`

	// git_https_api — auth mode: TOKEN | GITHUB_APP | OAUTH2 | PRIVATELINK
	GitAuthMode    string   `json:"gitAuthMode"`
	GithubAppPath  string   `json:"githubAppPath"`      // path after https://github.com/
	AllowedSecrets []string `json:"allowedAuthSecrets"` // ALL | NONE | secret names

	// OAuth2 mode
	OauthClientId      string `json:"oauthClientId"`
	OauthClientSecret  string `json:"oauthClientSecret"`
	OauthTokenEndpoint string `json:"oauthTokenEndpoint"`
	OauthScopes        string `json:"oauthScopes"` // comma-separated

	// PRIVATELINK mode
	UsePrivateLink  bool     `json:"usePrivateLink"`
	TlsCertificates []string `json:"tlsCertificates"`

	Comment string `json:"comment"`
}

// CatalogIntegrationParams holds form values for CREATE CATALOG INTEGRATION.
type CatalogIntegrationParams struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	Enabled       bool   `json:"enabled"`
	Source        string `json:"source"` // GLUE, OBJECT_STORE, POLARIS, ICEBERG_REST, SAP_BDC

	// GLUE
	GlueAwsRoleArn string `json:"glueAwsRoleArn"`
	GlueCatalogId  string `json:"glueCatalogId"`
	GlueRegion     string `json:"glueRegion"`

	// OBJECT_STORE
	TableFormat string `json:"tableFormat"` // ICEBERG | DELTA

	// POLARIS / ICEBERG_REST
	CatalogUri           string `json:"catalogUri"`
	CatalogName          string `json:"catalogName"`
	CatalogNamespace     string `json:"catalogNamespace"`
	CatalogApiType       string `json:"catalogApiType"`
	AccessDelegationMode string `json:"accessDelegationMode"`
	Prefix               string `json:"prefix"`

	// OAuth (shared by POLARIS and ICEBERG_REST with OAUTH auth type)
	OauthTokenUri     string `json:"oauthTokenUri"`
	OauthClientId     string `json:"oauthClientId"`
	OauthClientSecret string `json:"oauthClientSecret"`
	OauthScopes       string `json:"oauthScopes"` // comma-separated

	// ICEBERG_REST auth type (OAUTH | BEARER | SIGV4)
	IcebergAuthType string `json:"icebergAuthType"`
	BearerToken     string `json:"bearerToken"`

	// SAP_BDC
	SapInvitationLink string `json:"sapInvitationLink"`

	RefreshInterval int    `json:"refreshInterval"` // seconds; 0 = omit
	Comment         string `json:"comment"`
}

// ExternalAccessIntegrationParams holds form values for CREATE EXTERNAL ACCESS INTEGRATION.
type ExternalAccessIntegrationParams struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	Enabled       bool   `json:"enabled"`

	AllowedNetworkRules        string `json:"allowedNetworkRules"`
	AllowedApiAuthIntegrations string `json:"allowedApiAuthIntegrations"`
	AllowedAuthSecrets         string `json:"allowedAuthSecrets"` // "all", "none", or comma-separated names

	Comment string `json:"comment"`
}

// NotificationIntegrationParams holds form values for CREATE NOTIFICATION INTEGRATION.
type NotificationIntegrationParams struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	Enabled       bool   `json:"enabled"`
	Subtype       string `json:"subtype"` // EMAIL, WEBHOOK, AZURE_STORAGE_QUEUE_INBOUND, ...

	// Azure queue inbound
	AzureQueueUri  string `json:"azureQueueUri"`
	AzureTenantId  string `json:"azureTenantId"`
	UsePrivatelink bool   `json:"usePrivatelink"`

	// GCP PubSub inbound
	GcpSubName string `json:"gcpSubName"`

	// AWS SNS outbound
	AwsSnsTopicArn string `json:"awsSnsTopicArn"`
	AwsSnsRoleArn  string `json:"awsSnsRoleArn"`

	// Azure Event Grid outbound
	AzureTopicEndpoint string `json:"azureTopicEndpoint"`

	// GCP PubSub outbound
	GcpTopicName string `json:"gcpTopicName"`

	// EMAIL
	AllowedRecipients string `json:"allowedRecipients"` // comma-separated emails
	DefaultRecipients string `json:"defaultRecipients"`
	DefaultSubject    string `json:"defaultSubject"`

	// WEBHOOK
	WebhookUrl          string `json:"webhookUrl"`
	WebhookSecret       string `json:"webhookSecret"`
	WebhookBodyTemplate string `json:"webhookBodyTemplate"`
	WebhookHeaders      string `json:"webhookHeaders"`

	Comment string `json:"comment"`
}

// SecurityIntegrationParams holds form values for CREATE SECURITY INTEGRATION.
type SecurityIntegrationParams struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	Enabled       bool   `json:"enabled"`
	SecType       string `json:"secType"` // API_AUTHENTICATION, EXTERNAL_OAUTH, OAUTH_PARTNER, OAUTH_CUSTOM, SAML2, SCIM

	// API_AUTHENTICATION
	AuthType string `json:"authType"` // OAUTH2 | AWS_IAM
	// AWS_IAM
	AwsRoleArn string `json:"awsRoleArn"`
	// OAuth2
	OauthGrant         string `json:"oauthGrant"`
	OauthTokenEndpoint string `json:"oauthTokenEndpoint"`
	OauthClientId      string `json:"oauthClientId"`
	OauthClientSecret  string `json:"oauthClientSecret"`
	OauthScopes        string `json:"oauthScopes"` // comma-separated

	// EXTERNAL_OAUTH
	ExternalOauthType        string `json:"externalOauthType"`
	Issuer                   string `json:"issuer"`
	TokenUserMappingClaim    string `json:"tokenUserMappingClaim"`
	SnowflakeUserMappingAttr string `json:"snowflakeUserMappingAttr"`
	JwsKeysUrl               string `json:"jwsKeysUrl"`
	AudienceList             string `json:"audienceList"`
	AnyRoleMode              string `json:"anyRoleMode"`
	NetworkPolicy            string `json:"networkPolicy"`

	// OAUTH_PARTNER / OAUTH_CUSTOM
	OauthClient               string `json:"oauthClient"`
	OauthClientType           string `json:"oauthClientType"`
	OauthRedirectUri          string `json:"oauthRedirectUri"`
	OauthIssueRefreshTokens   bool   `json:"oauthIssueRefreshTokens"`
	OauthRefreshTokenValidity int    `json:"oauthRefreshTokenValidity"` // seconds; 0 = omit

	// SAML2
	SamlIdpMetadataUrl     string `json:"samlIdpMetadataUrl"`
	SamlIdpEntityId        string `json:"samlIdpEntityId"`
	SamlIdpSsoUrl          string `json:"samlIdpSsoUrl"`
	SamlIdpCert            string `json:"samlIdpCert"`
	SamlAllowedUserDomains string `json:"samlAllowedUserDomains"` // comma-separated
	SamlSignRequest        bool   `json:"samlSignRequest"`
	SamlForceAuthn         bool   `json:"samlForceAuthn"`

	// SCIM
	ScimClient   string `json:"scimClient"`
	RunAsRole    string `json:"runAsRole"`
	SyncPassword bool   `json:"syncPassword"`

	Comment string `json:"comment"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// boolKw returns "TRUE" or "FALSE".
func boolKw(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}

// identToken converts a name to a SQL identifier token, applying case-sensitivity rules.
// If caseSensitive is true the name is double-quoted as-is; otherwise it is upper-cased
// and double-quoted (which is equivalent to an unquoted identifier in Snowflake).
func identToken(name string, caseSensitive bool) string {
	if caseSensitive {
		return snowflake.QuoteIdent(name)
	}
	return snowflake.QuoteIdent(strings.ToUpper(name))
}

// squotedTuple returns a parenthesised, single-quoted list: ('a', 'b', 'c')
func squotedTuple(vals []string) string {
	quoted := make([]string, len(vals))
	for i, v := range vals {
		quoted[i] = snowflake.QuoteStringLit(v)
	}
	return "(" + strings.Join(quoted, ", ") + ")"
}

// squotedTupleFromString splits s and returns squotedTuple for the result.
// Returns "" when s is empty (caller should check and skip).
func squotedTupleFromString(s string) string {
	parts := snowflake.SplitValues(s)
	if len(parts) == 0 {
		return ""
	}
	return squotedTuple(parts)
}

// identListFromString splits s by newline/comma and returns a comma-joined list of
// raw (unquoted) tokens — used for identifier list parameters where Snowflake expects
// bare identifiers (e.g. ALLOWED_NETWORK_RULES).
// Each token is validated to contain only safe identifier characters.
func identListFromString(s string) (string, error) {
	parts := snowflake.SplitValues(s)
	for _, p := range parts {
		if _, err := validateIdentRef(p); err != nil {
			return "", err
		}
	}
	return strings.Join(parts, ", "), nil
}

// validIdentRef matches an optional DB.SCHEMA.NAME dot-path of identifiers.
// Identifiers may be double-quoted (allowing any chars inside) or unquoted
// (letters, digits, underscores, dollars only).
// Each component after the first must be preceded by a dot.
var validIdentRef = regexp.MustCompile(
	`^("(?:[^"]|"")*"|[A-Za-z_$][A-Za-z0-9_$]*)(\."(?:[^"]|"")*"|\.[A-Za-z_$][A-Za-z0-9_$]*)*$`,
)

// validateIdentRef validates that s looks like a (possibly qualified) Snowflake
// identifier reference (e.g. MY_DB.MY_SCHEMA.MY_OBJ or "My DB"."My Schema"."My Obj").
// Returns the original string on success, or an error.
func validateIdentRef(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !validIdentRef.MatchString(s) {
		return "", fmt.Errorf("invalid identifier reference: %q", s)
	}
	return s, nil
}

// mustBeOneOf validates that val is one of the allowed values (case-insensitive).
// Returns the matched allowed value (preserving original case from allowed list) or an error.
func mustBeOneOf(field, val string, allowed ...string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(val))
	for _, a := range allowed {
		if strings.ToUpper(a) == upper {
			return a, nil
		}
	}
	return "", fmt.Errorf("field %q: %q is not one of %v", field, val, allowed)
}

// secretsTuple builds the ALLOWED_AUTHENTICATION_SECRETS / TLS_TRUSTED_CERTIFICATES value.
// If refs is empty or nil → returns "", false (caller should omit the clause).
// If the single element is "ALL" or "NONE" → returns the keyword, true.
// Otherwise → each ref is validated as an identifier and the result is a parenthesised list.
func secretsTuple(refs []string) (string, bool, error) {
	var filtered []string
	for _, r := range refs {
		if t := strings.TrimSpace(r); t != "" {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		return "", false, nil
	}
	if len(filtered) == 1 {
		up := strings.ToUpper(filtered[0])
		if up == "ALL" || up == "NONE" {
			return up, true, nil
		}
	}
	validated := make([]string, len(filtered))
	for i, r := range filtered {
		v, err := validateIdentRef(r)
		if err != nil {
			return "", false, err
		}
		validated[i] = v
	}
	return "(" + strings.Join(validated, ", ") + ")", true, nil
}

// ── SQL builders ──────────────────────────────────────────────────────────────

// BuildStorageIntegrationSQL generates the CREATE STORAGE INTEGRATION DDL.
func BuildStorageIntegrationSQL(p StorageIntegrationParams) (string, error) {
	name := identToken(p.Name, p.CaseSensitive)

	provider, err := mustBeOneOf("provider", p.Provider, "S3", "S3GOV", "GCS", "AZURE")
	if err != nil {
		return "", err
	}

	lines := []string{
		fmt.Sprintf("CREATE STORAGE INTEGRATION %s", name),
		"  TYPE = EXTERNAL_STAGE",
		fmt.Sprintf("  STORAGE_PROVIDER = %s", snowflake.QuoteStringLit(provider)),
		fmt.Sprintf("  ENABLED = %s", boolKw(p.Enabled)),
	}

	switch provider {
	case "S3", "S3GOV":
		if p.AwsRoleArn != "" {
			lines = append(lines, fmt.Sprintf("  STORAGE_AWS_ROLE_ARN = %s", snowflake.QuoteStringLit(p.AwsRoleArn)))
		}
		if t := squotedTupleFromString(p.AllowedLocations); t != "" {
			lines = append(lines, fmt.Sprintf("  STORAGE_ALLOWED_LOCATIONS = %s", t))
		}
		if t := squotedTupleFromString(p.BlockedLocations); t != "" {
			lines = append(lines, fmt.Sprintf("  STORAGE_BLOCKED_LOCATIONS = %s", t))
		}
		if p.AwsExternalId != "" {
			lines = append(lines, fmt.Sprintf("  STORAGE_AWS_EXTERNAL_ID = %s", snowflake.QuoteStringLit(p.AwsExternalId)))
		}
		if p.UsePrivatelink {
			lines = append(lines, "  USE_PRIVATELINK_ENDPOINT = TRUE")
		}
	case "GCS":
		if t := squotedTupleFromString(p.AllowedLocations); t != "" {
			lines = append(lines, fmt.Sprintf("  STORAGE_ALLOWED_LOCATIONS = %s", t))
		}
		if t := squotedTupleFromString(p.BlockedLocations); t != "" {
			lines = append(lines, fmt.Sprintf("  STORAGE_BLOCKED_LOCATIONS = %s", t))
		}
	case "AZURE":
		if p.AzureTenantId != "" {
			lines = append(lines, fmt.Sprintf("  AZURE_TENANT_ID = %s", snowflake.QuoteStringLit(p.AzureTenantId)))
		}
		if t := squotedTupleFromString(p.AllowedLocations); t != "" {
			lines = append(lines, fmt.Sprintf("  STORAGE_ALLOWED_LOCATIONS = %s", t))
		}
		if t := squotedTupleFromString(p.BlockedLocations); t != "" {
			lines = append(lines, fmt.Sprintf("  STORAGE_BLOCKED_LOCATIONS = %s", t))
		}
		if p.UsePrivatelink {
			lines = append(lines, "  USE_PRIVATELINK_ENDPOINT = TRUE")
		}
	}

	if p.Comment != "" {
		lines = append(lines, fmt.Sprintf("  COMMENT = %s", snowflake.QuoteStringLit(p.Comment)))
	}
	return strings.Join(lines, "\n"), nil
}

// BuildApiIntegrationSQL generates the CREATE API INTEGRATION DDL.
func BuildApiIntegrationSQL(p ApiIntegrationParams) (string, error) {
	if p.Provider == "git_https_api" {
		return buildGitHttpsApiSQL(p)
	}

	validProviders := []string{
		"aws_api_gateway",
		"aws_private_api_gateway",
		"azure_api_management",
		"google_api_gateway",
	}
	provider, err := mustBeOneOf("provider", p.Provider, validProviders...)
	if err != nil {
		return "", err
	}

	name := identToken(p.Name, p.CaseSensitive)
	lines := []string{
		fmt.Sprintf("CREATE API INTEGRATION %s", name),
		fmt.Sprintf("  API_PROVIDER = %s", provider),
		fmt.Sprintf("  ENABLED = %s", boolKw(p.Enabled)),
	}

	if t := squotedTupleFromString(p.AllowedPrefixes); t != "" {
		lines = append(lines, fmt.Sprintf("  API_ALLOWED_PREFIXES = %s", t))
	}
	if t := squotedTupleFromString(p.BlockedPrefixes); t != "" {
		lines = append(lines, fmt.Sprintf("  API_BLOCKED_PREFIXES = %s", t))
	}

	switch provider {
	case "aws_api_gateway", "aws_private_api_gateway":
		if p.AwsRoleArn != "" {
			lines = append(lines, fmt.Sprintf("  API_AWS_ROLE_ARN = %s", snowflake.QuoteStringLit(p.AwsRoleArn)))
		}
		if p.ApiKey != "" {
			lines = append(lines, fmt.Sprintf("  API_KEY = %s", snowflake.QuoteStringLit(p.ApiKey)))
		}
	case "azure_api_management":
		if p.AzureTenantId != "" {
			lines = append(lines, fmt.Sprintf("  AZURE_TENANT_ID = %s", snowflake.QuoteStringLit(p.AzureTenantId)))
		}
		if p.AzureAdAppId != "" {
			lines = append(lines, fmt.Sprintf("  AZURE_AD_APPLICATION_ID = %s", snowflake.QuoteStringLit(p.AzureAdAppId)))
		}
		if p.ApiKey != "" {
			lines = append(lines, fmt.Sprintf("  API_KEY = %s", snowflake.QuoteStringLit(p.ApiKey)))
		}
	case "google_api_gateway":
		if p.GoogleAudience != "" {
			lines = append(lines, fmt.Sprintf("  GOOGLE_AUDIENCE = %s", snowflake.QuoteStringLit(p.GoogleAudience)))
		}
	}

	if p.Comment != "" {
		lines = append(lines, fmt.Sprintf("  COMMENT = %s", snowflake.QuoteStringLit(p.Comment)))
	}
	return strings.Join(lines, "\n"), nil
}

// buildGitHttpsApiSQL generates the DDL for a git_https_api API integration.
func buildGitHttpsApiSQL(p ApiIntegrationParams) (string, error) {
	if p.GitAuthMode == "" {
		p.GitAuthMode = "TOKEN"
	}
	mode, err := mustBeOneOf("gitAuthMode", p.GitAuthMode, "TOKEN", "GITHUB_APP", "OAUTH2", "PRIVATELINK")
	if err != nil {
		return "", err
	}

	name := identToken(p.Name, p.CaseSensitive)

	create := "CREATE"
	if p.OrReplace {
		create += " OR REPLACE"
	}
	create += " API INTEGRATION"
	if !p.OrReplace && p.IfNotExists {
		create += " IF NOT EXISTS"
	}

	lines := []string{
		fmt.Sprintf("%s %s", create, name),
		"  API_PROVIDER = git_https_api",
	}

	// API_ALLOWED_PREFIXES
	if mode == "GITHUB_APP" {
		path := strings.TrimLeft(strings.TrimSpace(p.GithubAppPath), "/")
		lines = append(lines, fmt.Sprintf("  API_ALLOWED_PREFIXES = (%s)", snowflake.QuoteStringLit("https://github.com/"+path)))
	} else {
		if t := squotedTupleFromString(p.AllowedPrefixes); t != "" {
			lines = append(lines, fmt.Sprintf("  API_ALLOWED_PREFIXES = %s", t))
		}
	}

	// API_BLOCKED_PREFIXES
	if t := squotedTupleFromString(p.BlockedPrefixes); t != "" {
		lines = append(lines, fmt.Sprintf("  API_BLOCKED_PREFIXES = %s", t))
	}

	// ALLOWED_AUTHENTICATION_SECRETS — TOKEN and PRIVATELINK modes
	if mode == "TOKEN" || mode == "PRIVATELINK" {
		if sec, ok, err := secretsTuple(p.AllowedSecrets); err != nil {
			return "", err
		} else if ok {
			lines = append(lines, fmt.Sprintf("  ALLOWED_AUTHENTICATION_SECRETS = %s", sec))
		}
	}

	// API_USER_AUTHENTICATION block
	switch mode {
	case "GITHUB_APP":
		lines = append(lines,
			"  API_USER_AUTHENTICATION = (",
			"    TYPE = SNOWFLAKE_GITHUB_APP",
			"  )",
		)
	case "OAUTH2":
		oauthLines := []string{"    TYPE = OAUTH2"}
		if p.OauthClientId != "" {
			oauthLines = append(oauthLines, fmt.Sprintf("    OAUTH_CLIENT_ID = %s", snowflake.QuoteStringLit(p.OauthClientId)))
		}
		if p.OauthClientSecret != "" {
			oauthLines = append(oauthLines, fmt.Sprintf("    OAUTH_CLIENT_SECRET = %s", snowflake.QuoteStringLit(p.OauthClientSecret)))
		}
		if p.OauthTokenEndpoint != "" {
			oauthLines = append(oauthLines, fmt.Sprintf("    OAUTH_TOKEN_ENDPOINT = %s", snowflake.QuoteStringLit(p.OauthTokenEndpoint)))
		}
		if p.OauthScopes != "" {
			var scopeQuoted []string
			for _, s := range snowflake.SplitValues(p.OauthScopes) {
				scopeQuoted = append(scopeQuoted, snowflake.QuoteStringLit(s))
			}
			if len(scopeQuoted) > 0 {
				oauthLines = append(oauthLines, fmt.Sprintf("    OAUTH_ALLOWED_SCOPES = (%s)", strings.Join(scopeQuoted, ", ")))
			}
		}
		lines = append(lines, "  API_USER_AUTHENTICATION = (")
		lines = append(lines, oauthLines...)
		lines = append(lines, "  )")
	case "PRIVATELINK":
		lines = append(lines, fmt.Sprintf("  USE_PRIVATELINK_ENDPOINT = %s", boolKw(p.UsePrivateLink)))
		if certs, ok, err := secretsTuple(p.TlsCertificates); err != nil {
			return "", err
		} else if ok {
			lines = append(lines, fmt.Sprintf("  TLS_TRUSTED_CERTIFICATES = %s", certs))
		}
	}

	lines = append(lines, fmt.Sprintf("  ENABLED = %s", boolKw(p.Enabled)))
	if p.Comment != "" {
		lines = append(lines, fmt.Sprintf("  COMMENT = %s", snowflake.QuoteStringLit(p.Comment)))
	}
	return strings.Join(lines, "\n"), nil
}

// BuildCatalogIntegrationSQL generates the CREATE CATALOG INTEGRATION DDL.
func BuildCatalogIntegrationSQL(p CatalogIntegrationParams) (string, error) {
	validSources := []string{"GLUE", "OBJECT_STORE", "POLARIS", "ICEBERG_REST", "SAP_BDC"}
	source, err := mustBeOneOf("source", p.Source, validSources...)
	if err != nil {
		return "", err
	}

	name := identToken(p.Name, p.CaseSensitive)
	lines := []string{
		fmt.Sprintf("CREATE CATALOG INTEGRATION %s", name),
		fmt.Sprintf("  CATALOG_SOURCE = %s", source),
		fmt.Sprintf("  ENABLED = %s", boolKw(p.Enabled)),
	}

	switch source {
	case "GLUE":
		if p.GlueAwsRoleArn != "" {
			lines = append(lines, fmt.Sprintf("  GLUE_AWS_ROLE_ARN = %s", snowflake.QuoteStringLit(p.GlueAwsRoleArn)))
		}
		if p.GlueCatalogId != "" {
			lines = append(lines, fmt.Sprintf("  GLUE_CATALOG_ID = %s", snowflake.QuoteStringLit(p.GlueCatalogId)))
		}
		if p.GlueRegion != "" {
			lines = append(lines, fmt.Sprintf("  GLUE_REGION = %s", snowflake.QuoteStringLit(p.GlueRegion)))
		}
		if p.CatalogNamespace != "" {
			lines = append(lines, fmt.Sprintf("  CATALOG_NAMESPACE = %s", snowflake.QuoteStringLit(p.CatalogNamespace)))
		}
	case "OBJECT_STORE":
		if p.TableFormat != "" {
			tf, err := mustBeOneOf("tableFormat", p.TableFormat, "ICEBERG", "DELTA")
			if err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("  TABLE_FORMAT = %s", tf))
		}
	case "POLARIS":
		if p.CatalogUri != "" {
			lines = append(lines, fmt.Sprintf("  CATALOG_URI = %s", snowflake.QuoteStringLit(p.CatalogUri)))
		}
		if p.CatalogName != "" {
			lines = append(lines, fmt.Sprintf("  CATALOG_NAME = %s", snowflake.QuoteStringLit(p.CatalogName)))
		}
		if p.CatalogNamespace != "" {
			lines = append(lines, fmt.Sprintf("  CATALOG_NAMESPACE = %s", snowflake.QuoteStringLit(p.CatalogNamespace)))
		}
		if p.CatalogApiType != "" {
			at, err := mustBeOneOf("catalogApiType", p.CatalogApiType, "PUBLIC", "PRIVATE")
			if err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("  CATALOG_API_TYPE = %s", at))
		}
		if p.AccessDelegationMode != "" {
			lines = append(lines, fmt.Sprintf("  ACCESS_DELEGATION_MODE = %s", p.AccessDelegationMode))
		}
		if p.OauthTokenUri != "" {
			lines = append(lines, fmt.Sprintf("  OAUTH_TOKEN_URI = %s", snowflake.QuoteStringLit(p.OauthTokenUri)))
		}
		if p.OauthClientId != "" {
			lines = append(lines, fmt.Sprintf("  OAUTH_CLIENT_ID = %s", snowflake.QuoteStringLit(p.OauthClientId)))
		}
		if p.OauthClientSecret != "" {
			lines = append(lines, fmt.Sprintf("  OAUTH_CLIENT_SECRET = %s", snowflake.QuoteStringLit(p.OauthClientSecret)))
		}
		if p.OauthScopes != "" {
			lines = append(lines, fmt.Sprintf("  OAUTH_ALLOWED_SCOPES = %s", squotedTupleFromString(p.OauthScopes)))
		}
	case "ICEBERG_REST":
		if p.CatalogUri != "" {
			lines = append(lines, fmt.Sprintf("  CATALOG_URI = %s", snowflake.QuoteStringLit(p.CatalogUri)))
		}
		if p.CatalogName != "" {
			lines = append(lines, fmt.Sprintf("  CATALOG_NAME = %s", snowflake.QuoteStringLit(p.CatalogName)))
		}
		if p.CatalogNamespace != "" {
			lines = append(lines, fmt.Sprintf("  CATALOG_NAMESPACE = %s", snowflake.QuoteStringLit(p.CatalogNamespace)))
		}
		if p.CatalogApiType != "" {
			at, err := mustBeOneOf("catalogApiType", p.CatalogApiType,
				"PUBLIC", "PRIVATE", "AWS_API_GATEWAY", "AWS_PRIVATE_API_GATEWAY", "AWS_GLUE", "AWS_PRIVATE_GLUE")
			if err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("  CATALOG_API_TYPE = %s", at))
		}
		if p.AccessDelegationMode != "" {
			lines = append(lines, fmt.Sprintf("  ACCESS_DELEGATION_MODE = %s", p.AccessDelegationMode))
		}

		authType := "OAUTH"
		if p.IcebergAuthType != "" {
			authType, err = mustBeOneOf("icebergAuthType", p.IcebergAuthType, "OAUTH", "BEARER", "SIGV4")
			if err != nil {
				return "", err
			}
		}
		lines = append(lines, fmt.Sprintf("  AUTH_TYPE = %s", authType))

		switch authType {
		case "OAUTH":
			if p.OauthTokenUri != "" {
				lines = append(lines, fmt.Sprintf("  OAUTH_TOKEN_URI = %s", snowflake.QuoteStringLit(p.OauthTokenUri)))
			}
			if p.OauthClientId != "" {
				lines = append(lines, fmt.Sprintf("  OAUTH_CLIENT_ID = %s", snowflake.QuoteStringLit(p.OauthClientId)))
			}
			if p.OauthClientSecret != "" {
				lines = append(lines, fmt.Sprintf("  OAUTH_CLIENT_SECRET = %s", snowflake.QuoteStringLit(p.OauthClientSecret)))
			}
			if p.OauthScopes != "" {
				lines = append(lines, fmt.Sprintf("  OAUTH_ALLOWED_SCOPES = %s", squotedTupleFromString(p.OauthScopes)))
			}
		case "BEARER":
			if p.BearerToken != "" {
				lines = append(lines, fmt.Sprintf("  BEARER_TOKEN = %s", snowflake.QuoteStringLit(p.BearerToken)))
			}
		}

		if p.Prefix != "" {
			lines = append(lines, fmt.Sprintf("  PREFIX = %s", snowflake.QuoteStringLit(p.Prefix)))
		}
	case "SAP_BDC":
		if p.SapInvitationLink != "" {
			lines = append(lines, fmt.Sprintf("  SAP_BDC_INVITATION_LINK = %s", snowflake.QuoteStringLit(p.SapInvitationLink)))
		}
		if p.AccessDelegationMode != "" {
			lines = append(lines, fmt.Sprintf("  ACCESS_DELEGATION_MODE = %s", p.AccessDelegationMode))
		}
	}

	if p.RefreshInterval > 0 {
		lines = append(lines, fmt.Sprintf("  REFRESH_INTERVAL_SECONDS = %d", p.RefreshInterval))
	}
	if p.Comment != "" {
		lines = append(lines, fmt.Sprintf("  COMMENT = %s", snowflake.QuoteStringLit(p.Comment)))
	}
	return strings.Join(lines, "\n"), nil
}

// BuildExternalAccessIntegrationSQL generates the CREATE EXTERNAL ACCESS INTEGRATION DDL.
func BuildExternalAccessIntegrationSQL(p ExternalAccessIntegrationParams) (string, error) {
	name := identToken(p.Name, p.CaseSensitive)
	lines := []string{
		fmt.Sprintf("CREATE EXTERNAL ACCESS INTEGRATION %s", name),
	}

	if p.AllowedNetworkRules != "" {
		list, err := identListFromString(p.AllowedNetworkRules)
		if err != nil {
			return "", fmt.Errorf("allowedNetworkRules: %w", err)
		}
		lines = append(lines, fmt.Sprintf("  ALLOWED_NETWORK_RULES = (%s)", list))
	}
	if p.AllowedApiAuthIntegrations != "" {
		list, err := identListFromString(p.AllowedApiAuthIntegrations)
		if err != nil {
			return "", fmt.Errorf("allowedApiAuthIntegrations: %w", err)
		}
		lines = append(lines, fmt.Sprintf("  ALLOWED_API_AUTHENTICATION_INTEGRATIONS = (%s)", list))
	}
	if p.AllowedAuthSecrets != "" {
		sec := strings.TrimSpace(p.AllowedAuthSecrets)
		up := strings.ToUpper(sec)
		if up == "ALL" || up == "NONE" {
			lines = append(lines, fmt.Sprintf("  ALLOWED_AUTHENTICATION_SECRETS = %s", up))
		} else {
			list, err := identListFromString(sec)
			if err != nil {
				return "", fmt.Errorf("allowedAuthSecrets: %w", err)
			}
			lines = append(lines, fmt.Sprintf("  ALLOWED_AUTHENTICATION_SECRETS = (%s)", list))
		}
	}

	lines = append(lines, fmt.Sprintf("  ENABLED = %s", boolKw(p.Enabled)))
	if p.Comment != "" {
		lines = append(lines, fmt.Sprintf("  COMMENT = %s", snowflake.QuoteStringLit(p.Comment)))
	}
	return strings.Join(lines, "\n"), nil
}

// BuildNotificationIntegrationSQL generates the CREATE NOTIFICATION INTEGRATION DDL.
func BuildNotificationIntegrationSQL(p NotificationIntegrationParams) (string, error) {
	validSubtypes := []string{
		"EMAIL", "WEBHOOK",
		"AZURE_STORAGE_QUEUE_INBOUND", "GCP_PUBSUB_INBOUND",
		"AWS_SNS_OUTBOUND", "AZURE_EVENT_GRID_OUTBOUND", "GCP_PUBSUB_OUTBOUND",
	}
	subtype, err := mustBeOneOf("subtype", p.Subtype, validSubtypes...)
	if err != nil {
		return "", err
	}

	name := identToken(p.Name, p.CaseSensitive)
	lines := []string{fmt.Sprintf("CREATE NOTIFICATION INTEGRATION %s", name)}

	switch subtype {
	case "AZURE_STORAGE_QUEUE_INBOUND":
		lines = append(lines,
			"  TYPE = QUEUE",
			"  NOTIFICATION_PROVIDER = AZURE_STORAGE_QUEUE",
			"  DIRECTION = INBOUND",
		)
		if p.AzureQueueUri != "" {
			lines = append(lines, fmt.Sprintf("  AZURE_STORAGE_QUEUE_PRIMARY_URI = %s", snowflake.QuoteStringLit(p.AzureQueueUri)))
		}
		if p.AzureTenantId != "" {
			lines = append(lines, fmt.Sprintf("  AZURE_TENANT_ID = %s", snowflake.QuoteStringLit(p.AzureTenantId)))
		}
		if p.UsePrivatelink {
			lines = append(lines, "  USE_PRIVATELINK_ENDPOINT = TRUE")
		}
	case "GCP_PUBSUB_INBOUND":
		lines = append(lines,
			"  TYPE = QUEUE",
			"  NOTIFICATION_PROVIDER = GCP_PUBSUB",
			"  DIRECTION = INBOUND",
		)
		if p.GcpSubName != "" {
			lines = append(lines, fmt.Sprintf("  GCP_PUBSUB_SUBSCRIPTION_NAME = %s", snowflake.QuoteStringLit(p.GcpSubName)))
		}
	case "AWS_SNS_OUTBOUND":
		lines = append(lines,
			"  TYPE = QUEUE",
			"  NOTIFICATION_PROVIDER = AWS_SNS",
			"  DIRECTION = OUTBOUND",
		)
		if p.AwsSnsTopicArn != "" {
			lines = append(lines, fmt.Sprintf("  AWS_SNS_TOPIC_ARN = %s", snowflake.QuoteStringLit(p.AwsSnsTopicArn)))
		}
		if p.AwsSnsRoleArn != "" {
			lines = append(lines, fmt.Sprintf("  AWS_SNS_ROLE_ARN = %s", snowflake.QuoteStringLit(p.AwsSnsRoleArn)))
		}
	case "AZURE_EVENT_GRID_OUTBOUND":
		lines = append(lines,
			"  TYPE = QUEUE",
			"  NOTIFICATION_PROVIDER = AZURE_EVENT_GRID",
			"  DIRECTION = OUTBOUND",
		)
		if p.AzureTopicEndpoint != "" {
			lines = append(lines, fmt.Sprintf("  AZURE_EVENT_GRID_TOPIC_ENDPOINT = %s", snowflake.QuoteStringLit(p.AzureTopicEndpoint)))
		}
		if p.AzureTenantId != "" {
			lines = append(lines, fmt.Sprintf("  AZURE_TENANT_ID = %s", snowflake.QuoteStringLit(p.AzureTenantId)))
		}
	case "GCP_PUBSUB_OUTBOUND":
		lines = append(lines,
			"  TYPE = QUEUE",
			"  NOTIFICATION_PROVIDER = GCP_PUBSUB",
			"  DIRECTION = OUTBOUND",
		)
		if p.GcpTopicName != "" {
			lines = append(lines, fmt.Sprintf("  GCP_PUBSUB_TOPIC_NAME = %s", snowflake.QuoteStringLit(p.GcpTopicName)))
		}
	case "EMAIL":
		lines = append(lines, "  TYPE = EMAIL")
		if t := squotedTupleFromString(p.AllowedRecipients); t != "" {
			lines = append(lines, fmt.Sprintf("  ALLOWED_RECIPIENTS = %s", t))
		}
		if t := squotedTupleFromString(p.DefaultRecipients); t != "" {
			lines = append(lines, fmt.Sprintf("  DEFAULT_RECIPIENTS = %s", t))
		}
		if p.DefaultSubject != "" {
			lines = append(lines, fmt.Sprintf("  DEFAULT_SUBJECT = %s", snowflake.QuoteStringLit(p.DefaultSubject)))
		}
	case "WEBHOOK":
		lines = append(lines, "  TYPE = WEBHOOK")
		if p.WebhookUrl != "" {
			lines = append(lines, fmt.Sprintf("  WEBHOOK_URL = %s", snowflake.QuoteStringLit(p.WebhookUrl)))
		}
		if p.WebhookSecret != "" {
			lines = append(lines, fmt.Sprintf("  WEBHOOK_SECRET = %s", snowflake.QuoteStringLit(p.WebhookSecret)))
		}
		if p.WebhookBodyTemplate != "" {
			lines = append(lines, fmt.Sprintf("  WEBHOOK_BODY_TEMPLATE = %s", snowflake.QuoteStringLit(p.WebhookBodyTemplate)))
		}
		if p.WebhookHeaders != "" {
			lines = append(lines, fmt.Sprintf("  WEBHOOK_HEADERS = (%s)", p.WebhookHeaders))
		}
	}

	lines = append(lines, fmt.Sprintf("  ENABLED = %s", boolKw(p.Enabled)))
	if p.Comment != "" {
		lines = append(lines, fmt.Sprintf("  COMMENT = %s", snowflake.QuoteStringLit(p.Comment)))
	}
	return strings.Join(lines, "\n"), nil
}

// BuildSecurityIntegrationSQL generates the CREATE SECURITY INTEGRATION DDL.
func BuildSecurityIntegrationSQL(p SecurityIntegrationParams) (string, error) {
	validSecTypes := []string{"API_AUTHENTICATION", "EXTERNAL_OAUTH", "OAUTH_PARTNER", "OAUTH_CUSTOM", "SAML2", "SCIM"}
	secType, err := mustBeOneOf("secType", p.SecType, validSecTypes...)
	if err != nil {
		return "", err
	}

	name := identToken(p.Name, p.CaseSensitive)
	lines := []string{fmt.Sprintf("CREATE SECURITY INTEGRATION %s", name)}

	switch secType {
	case "API_AUTHENTICATION":
		lines = append(lines, "  TYPE = API_AUTHENTICATION")
		authType := "OAUTH2"
		if p.AuthType != "" {
			authType, err = mustBeOneOf("authType", p.AuthType, "OAUTH2", "AWS_IAM")
			if err != nil {
				return "", err
			}
		}
		lines = append(lines, fmt.Sprintf("  AUTH_TYPE = %s", authType))
		if authType == "AWS_IAM" {
			if p.AwsRoleArn != "" {
				lines = append(lines, fmt.Sprintf("  AWS_ROLE_ARN = %s", snowflake.QuoteStringLit(p.AwsRoleArn)))
			}
		} else {
			grant := "CLIENT_CREDENTIALS"
			if p.OauthGrant != "" {
				grant, err = mustBeOneOf("oauthGrant", p.OauthGrant, "CLIENT_CREDENTIALS", "AUTHORIZATION_CODE", "JWT_BEARER")
				if err != nil {
					return "", err
				}
			}
			lines = append(lines, fmt.Sprintf("  OAUTH_GRANT = %s", grant))
			if p.OauthTokenEndpoint != "" {
				lines = append(lines, fmt.Sprintf("  OAUTH_TOKEN_ENDPOINT = %s", snowflake.QuoteStringLit(p.OauthTokenEndpoint)))
			}
			if p.OauthClientId != "" {
				lines = append(lines, fmt.Sprintf("  OAUTH_CLIENT_ID = %s", snowflake.QuoteStringLit(p.OauthClientId)))
			}
			if p.OauthClientSecret != "" {
				lines = append(lines, fmt.Sprintf("  OAUTH_CLIENT_SECRET = %s", snowflake.QuoteStringLit(p.OauthClientSecret)))
			}
			if p.OauthScopes != "" {
				lines = append(lines, fmt.Sprintf("  OAUTH_ALLOWED_SCOPES = %s", squotedTupleFromString(p.OauthScopes)))
			}
		}

	case "EXTERNAL_OAUTH":
		lines = append(lines, "  TYPE = EXTERNAL_OAUTH")
		if p.ExternalOauthType != "" {
			ot, err := mustBeOneOf("externalOauthType", p.ExternalOauthType, "OKTA", "AZURE", "PING_FEDERATE", "CUSTOM")
			if err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("  EXTERNAL_OAUTH_TYPE = %s", ot))
		}
		if p.Issuer != "" {
			lines = append(lines, fmt.Sprintf("  EXTERNAL_OAUTH_ISSUER = %s", snowflake.QuoteStringLit(p.Issuer)))
		}
		if p.TokenUserMappingClaim != "" {
			lines = append(lines, fmt.Sprintf("  EXTERNAL_OAUTH_TOKEN_USER_MAPPING_CLAIM = %s", snowflake.QuoteStringLit(p.TokenUserMappingClaim)))
		}
		if p.SnowflakeUserMappingAttr != "" {
			uma, err := mustBeOneOf("snowflakeUserMappingAttr", p.SnowflakeUserMappingAttr, "LOGIN_NAME", "EMAIL_ADDRESS")
			if err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("  EXTERNAL_OAUTH_SNOWFLAKE_USER_MAPPING_ATTRIBUTE = %s", snowflake.QuoteStringLit(uma)))
		}
		if p.JwsKeysUrl != "" {
			lines = append(lines, fmt.Sprintf("  EXTERNAL_OAUTH_JWS_KEYS_URL = %s", snowflake.QuoteStringLit(p.JwsKeysUrl)))
		}
		if p.AudienceList != "" {
			lines = append(lines, fmt.Sprintf("  EXTERNAL_OAUTH_AUDIENCE_LIST = %s", squotedTupleFromString(p.AudienceList)))
		}
		if p.AnyRoleMode != "" {
			arm, err := mustBeOneOf("anyRoleMode", p.AnyRoleMode, "DISABLE", "ENABLE", "ENABLE_FOR_PRIVILEGE")
			if err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("  EXTERNAL_OAUTH_ANY_ROLE_MODE = %s", arm))
		}
		if p.NetworkPolicy != "" {
			lines = append(lines, fmt.Sprintf("  NETWORK_POLICY = %s", snowflake.QuoteIdent(p.NetworkPolicy)))
		}

	case "OAUTH_PARTNER":
		lines = append(lines, "  TYPE = OAUTH")
		if p.OauthClient != "" {
			oc, err := mustBeOneOf("oauthClient", p.OauthClient, "LOOKER", "TABLEAU_DESKTOP", "TABLEAU_SERVER", "POWER_BI")
			if err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("  OAUTH_CLIENT = %s", oc))
		}
		if p.OauthRedirectUri != "" {
			lines = append(lines, fmt.Sprintf("  OAUTH_REDIRECT_URI = %s", snowflake.QuoteStringLit(p.OauthRedirectUri)))
		}
		lines = append(lines, fmt.Sprintf("  OAUTH_ISSUE_REFRESH_TOKENS = %s", boolKw(p.OauthIssueRefreshTokens)))
		if p.OauthRefreshTokenValidity > 0 {
			lines = append(lines, fmt.Sprintf("  OAUTH_REFRESH_TOKEN_VALIDITY = %d", p.OauthRefreshTokenValidity))
		}

	case "OAUTH_CUSTOM":
		lines = append(lines, "  TYPE = OAUTH", "  OAUTH_CLIENT = CUSTOM")
		if p.OauthClientType != "" {
			ct, err := mustBeOneOf("oauthClientType", p.OauthClientType, "CONFIDENTIAL", "PUBLIC")
			if err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("  OAUTH_CLIENT_TYPE = %s", ct))
		}
		if p.OauthRedirectUri != "" {
			lines = append(lines, fmt.Sprintf("  OAUTH_REDIRECT_URI = %s", snowflake.QuoteStringLit(p.OauthRedirectUri)))
		}
		lines = append(lines, fmt.Sprintf("  OAUTH_ISSUE_REFRESH_TOKENS = %s", boolKw(p.OauthIssueRefreshTokens)))
		if p.OauthRefreshTokenValidity > 0 {
			lines = append(lines, fmt.Sprintf("  OAUTH_REFRESH_TOKEN_VALIDITY = %d", p.OauthRefreshTokenValidity))
		}
		if p.NetworkPolicy != "" {
			lines = append(lines, fmt.Sprintf("  NETWORK_POLICY = %s", snowflake.QuoteIdent(p.NetworkPolicy)))
		}

	case "SAML2":
		lines = append(lines, "  TYPE = SAML2")
		if p.SamlIdpMetadataUrl != "" {
			lines = append(lines, fmt.Sprintf("  SAML2_IDP_METADATA_URL = %s", snowflake.QuoteStringLit(p.SamlIdpMetadataUrl)))
		} else {
			if p.SamlIdpEntityId != "" {
				lines = append(lines, fmt.Sprintf("  SAML2_IDP_ENTITY_ID = %s", snowflake.QuoteStringLit(p.SamlIdpEntityId)))
			}
			if p.SamlIdpSsoUrl != "" {
				lines = append(lines, fmt.Sprintf("  SAML2_IDP_SSO_URL = %s", snowflake.QuoteStringLit(p.SamlIdpSsoUrl)))
			}
			if p.SamlIdpCert != "" {
				lines = append(lines, fmt.Sprintf("  SAML2_IDP_CERTIFICATE = %s", snowflake.QuoteStringLit(p.SamlIdpCert)))
			}
		}
		if p.SamlAllowedUserDomains != "" {
			lines = append(lines, fmt.Sprintf("  SAML2_ALLOWED_EMAIL_PATTERNS = %s", squotedTupleFromString(p.SamlAllowedUserDomains)))
		}
		lines = append(lines, fmt.Sprintf("  SAML2_SIGN_REQUEST = %s", boolKw(p.SamlSignRequest)))
		lines = append(lines, fmt.Sprintf("  SAML2_FORCE_AUTHN = %s", boolKw(p.SamlForceAuthn)))

	case "SCIM":
		lines = append(lines, "  TYPE = SCIM")
		if p.ScimClient != "" {
			sc, err := mustBeOneOf("scimClient", p.ScimClient, "OKTA", "AZURE", "GENERIC")
			if err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("  SCIM_CLIENT = %s", snowflake.QuoteStringLit(sc)))
		}
		if p.RunAsRole != "" {
			lines = append(lines, fmt.Sprintf("  RUN_AS_SERVICE_USER = %s", snowflake.QuoteIdent(p.RunAsRole)))
		}
		if p.NetworkPolicy != "" {
			lines = append(lines, fmt.Sprintf("  NETWORK_POLICY = %s", snowflake.QuoteIdent(p.NetworkPolicy)))
		}
		lines = append(lines, fmt.Sprintf("  SYNC_PASSWORD = %s", boolKw(p.SyncPassword)))
	}

	lines = append(lines, fmt.Sprintf("  ENABLED = %s", boolKw(p.Enabled)))
	if p.Comment != "" {
		lines = append(lines, fmt.Sprintf("  COMMENT = %s", snowflake.QuoteStringLit(p.Comment)))
	}
	return strings.Join(lines, "\n"), nil
}
