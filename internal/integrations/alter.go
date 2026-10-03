// SPDX-License-Identifier: GPL-3.0-or-later

package integrations

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"thaw/internal/snowflake"
)

// Value types of an alterable integration property. They double as the editor
// hint handed to the frontend (Property.Editor).
const (
	tBool      = "boolean"   // TRUE | FALSE
	tInt       = "number"    // non-negative integer
	tText      = "text"      // 'string literal'
	tEnum      = "select"    // bare keyword from propSpec.options
	tList      = "list"      // ('a', 'b')
	tIdent     = "ident"     // identifier reference
	tIdentList = "identList" // (a, b) — or one of the bare keywords in propSpec.options
)

// propSpec describes one property settable through ALTER <kind> INTEGRATION.
type propSpec struct {
	typ string
	// options: the allowed keywords for tEnum; for tIdentList the keywords
	// accepted in place of a list (ALL / NONE).
	options []string
	// unset: the grammar allows UNSET <prop>, so an empty value emits it.
	unset bool
	// secret: DESCRIBE never returns the value — the row is set-only.
	secret bool
	// wrap: the property is nested, e.g. REST_AUTHENTICATION = ( <prop> = … ).
	wrap string
	// requires: the property only applies to the integration subtype whose
	// DESCRIBE output carries one of these properties. Used for display only —
	// Snowflake itself rejects a property the subtype doesn't support.
	requires []string
}

// commonProps are settable on every integration kind.
//
// ponytail: ENABLED is listed for all six kinds although the transcribed
// ALTER CATALOG INTEGRATION grammar omits it; if Snowflake rejects it the
// error shows inline under the row. Move it into the per-kind tables if so.
var commonProps = map[string]propSpec{
	"ENABLED": {typ: tBool},
	"COMMENT": {typ: tText},
}

var (
	textUnset = propSpec{typ: tText, unset: true}
	listUnset = propSpec{typ: tList, unset: true}
	// apiAuth marks the External API Authentication (OAuth) properties; their
	// names collide with read-only generated values on Snowflake OAuth
	// integrations, which have no OAUTH_GRANT row.
	apiAuth = []string{"OAUTH_GRANT"}
)

// alterable is the single source of truth for what the Integration Properties
// modal may edit: kind → property → value type. Derived from the SET / UNSET
// lists in the internal/sqlgrammar ParseAlter*Integration* doc comments.
// Subtypes of a kind (notification email / webhook / queue, the security
// integration flavours) are unioned: DESCRIBE only returns the rows of the
// integration's own subtype, so the others never render.
var alterable = map[string]map[string]propSpec{
	"STORAGE": {
		"COMMENT":                   textUnset,
		"STORAGE_AWS_ROLE_ARN":      {typ: tText},
		"STORAGE_AWS_OBJECT_ACL":    {typ: tText},
		"AZURE_TENANT_ID":           {typ: tText},
		"STORAGE_ALLOWED_LOCATIONS": {typ: tList},
		"STORAGE_BLOCKED_LOCATIONS": listUnset,
		"USE_PRIVATELINK_ENDPOINT":  {typ: tBool},
	},
	"API": {
		"COMMENT":                 textUnset,
		"API_AWS_ROLE_ARN":        {typ: tText},
		"AZURE_AD_APPLICATION_ID": {typ: tText},
		"API_KEY": {typ: tText, secret: true, unset: true,
			requires: []string{"API_AWS_ROLE_ARN", "AZURE_AD_APPLICATION_ID"}},
		"API_ALLOWED_PREFIXES":           {typ: tList},
		"API_BLOCKED_PREFIXES":           listUnset,
		"ALLOWED_AUTHENTICATION_SECRETS": {typ: tIdentList, options: []string{"ALL", "NONE"}},
	},
	"CATALOG": {
		"REFRESH_INTERVAL_SECONDS": {typ: tInt},
		"OAUTH_CLIENT_SECRET": {typ: tText, secret: true, wrap: "REST_AUTHENTICATION",
			requires: []string{"OAUTH_CLIENT_ID"}},
		"BEARER_TOKEN": {typ: tText, secret: true, wrap: "REST_AUTHENTICATION",
			requires: []string{"CATALOG_URI"}},
	},
	"EXTERNAL ACCESS": {
		"COMMENT":               textUnset,
		"ALLOWED_NETWORK_RULES": {typ: tIdentList, unset: true},
		"ALLOWED_API_AUTHENTICATION_INTEGRATIONS": {typ: tIdentList, options: []string{"NONE"}, unset: true},
		"ALLOWED_AUTHENTICATION_SECRETS":          {typ: tIdentList, options: []string{"ALL", "NONE"}, unset: true},
	},
	// ponytail: WEBHOOK_HEADERS ('k'='v' pairs) and the Azure / GCP queue
	// fields stay read-only — the header pair syntax needs its own editor and
	// the transcribed queue grammars disagree with the CREATE property names.
	"NOTIFICATION": {
		"COMMENT":               textUnset,
		"ALLOWED_RECIPIENTS":    listUnset,
		"DEFAULT_RECIPIENTS":    listUnset,
		"DEFAULT_SUBJECT":       textUnset,
		"WEBHOOK_URL":           {typ: tText},
		"WEBHOOK_SECRET":        {typ: tIdent, unset: true},
		"WEBHOOK_BODY_TEMPLATE": textUnset,
		"AWS_SNS_TOPIC_ARN":     {typ: tText},
		"AWS_SNS_ROLE_ARN":      {typ: tText},
	},
	// ponytail: the bare-key properties (EXTERNAL_OAUTH_RSA_PUBLIC_KEY*,
	// OAUTH_CLIENT_RSA_PUBLIC_KEY*) and the REFRESH actions are not covered.
	"SECURITY": {
		// SCIM / shared
		"NETWORK_POLICY": textUnset,
		"SYNC_PASSWORD":  {typ: tBool},
		// AWS IAM authentication
		"AWS_ROLE_ARN": {typ: tText},
		// External API authentication
		"OAUTH_TOKEN_ENDPOINT":         {typ: tText, requires: apiAuth},
		"OAUTH_AUTHORIZATION_ENDPOINT": {typ: tText, requires: apiAuth},
		"OAUTH_CLIENT_ID":              {typ: tText, requires: apiAuth},
		"OAUTH_CLIENT_SECRET":          {typ: tText, secret: true, requires: apiAuth},
		"OAUTH_CLIENT_AUTH_METHOD": {typ: tEnum, requires: apiAuth,
			options: []string{"CLIENT_SECRET_BASIC", "CLIENT_SECRET_POST"}},
		"OAUTH_ACCESS_TOKEN_VALIDITY": {typ: tInt, requires: apiAuth},
		"OAUTH_ALLOWED_SCOPES":        {typ: tList, requires: apiAuth},
		// External OAuth
		"EXTERNAL_OAUTH_TYPE":                             {typ: tEnum, options: []string{"OKTA", "AZURE", "PING_FEDERATE", "CUSTOM"}},
		"EXTERNAL_OAUTH_ISSUER":                           {typ: tText},
		"EXTERNAL_OAUTH_TOKEN_USER_MAPPING_CLAIM":         {typ: tList},
		"EXTERNAL_OAUTH_SNOWFLAKE_USER_MAPPING_ATTRIBUTE": {typ: tText},
		"EXTERNAL_OAUTH_JWS_KEYS_URL":                     {typ: tText},
		"EXTERNAL_OAUTH_BLOCKED_ROLES_LIST":               {typ: tList},
		"EXTERNAL_OAUTH_ALLOWED_ROLES_LIST":               {typ: tList},
		"EXTERNAL_OAUTH_AUDIENCE_LIST":                    listUnset,
		"EXTERNAL_OAUTH_ANY_ROLE_MODE":                    {typ: tEnum, options: []string{"DISABLE", "ENABLE", "ENABLE_FOR_PRIVILEGE"}},
		"EXTERNAL_OAUTH_SCOPE_DELIMITER":                  {typ: tText},
		// Snowflake OAuth (partner + custom)
		"OAUTH_ISSUE_REFRESH_TOKENS":                 {typ: tBool},
		"OAUTH_REDIRECT_URI":                         {typ: tText},
		"OAUTH_REFRESH_TOKEN_VALIDITY":               {typ: tInt},
		"OAUTH_SINGLE_USE_REFRESH_TOKENS_REQUIRED":   {typ: tBool},
		"OAUTH_USE_SECONDARY_ROLES":                  {typ: tEnum, options: []string{"IMPLICIT", "NONE"}},
		"OAUTH_ALLOW_NON_TLS_REDIRECT_URI":           {typ: tBool},
		"OAUTH_ALTERNATE_REDIRECT_URIS":              {typ: tList},
		"OAUTH_ENFORCE_PKCE":                         {typ: tBool},
		"ALLOWED_ROLES_LIST":                         {typ: tList},
		"BLOCKED_ROLES_LIST":                         {typ: tList},
		"PRE_AUTHORIZED_ROLES_LIST":                  {typ: tList},
		"USE_PRIVATELINK_FOR_AUTHORIZATION_ENDPOINT": {typ: tBool},
		// SAML2
		"METADATA_URL":                        {typ: tText},
		"SAML2_ISSUER":                        {typ: tText},
		"SAML2_SSO_URL":                       {typ: tText},
		"SAML2_PROVIDER":                      {typ: tText},
		"SAML2_X509_CERT":                     {typ: tText},
		"ALLOWED_USER_DOMAINS":                {typ: tList},
		"ALLOWED_EMAIL_PATTERNS":              {typ: tList},
		"SAML2_SP_INITIATED_LOGIN_PAGE_LABEL": {typ: tText},
		"SAML2_ENABLE_SP_INITIATED":           {typ: tBool},
		"SAML2_SNOWFLAKE_X509_CERT":           {typ: tText},
		"SAML2_SIGN_REQUEST":                  {typ: tBool},
		"SAML2_REQUESTED_NAMEID_FORMAT":       {typ: tText},
		"SAML2_POST_LOGOUT_REDIRECT_URL":      {typ: tText},
		"SAML2_FORCE_AUTHN":                   {typ: tBool},
		"SAML2_SNOWFLAKE_ISSUER_URL":          {typ: tText},
		"SAML2_SNOWFLAKE_ACS_URL":             {typ: tText},
	},
}

// NormalizeKind validates an integration kind (STORAGE, API, CATALOG,
// EXTERNAL ACCESS, NOTIFICATION, SECURITY) and returns it upper-cased, safe to
// embed as the `ALTER <kind> INTEGRATION` keyword.
func NormalizeKind(kind string) (string, error) {
	k := strings.ToUpper(strings.TrimSpace(kind))
	if _, ok := alterable[k]; !ok {
		return "", fmt.Errorf("unknown integration kind %q", kind)
	}
	return k, nil
}

// specFor looks up property (upper-cased) for an already-normalized kind.
func specFor(kind, property string) (propSpec, bool) {
	if s, ok := alterable[kind][property]; ok {
		return s, true
	}
	s, ok := commonProps[property]
	return s, ok
}

// listTokens splits a list value on commas / newlines. It tolerates the
// bracketed form DESCRIBE INTEGRATION reports for some lists ("[a, b]") so an
// edit that starts from the displayed value round-trips.
// ponytail: a value containing a comma can't be expressed; needs a real list
// editor if that ever matters.
func listTokens(s string, stripQuotes bool) []string {
	parts := snowflake.SplitValues(strings.Trim(strings.TrimSpace(s), "[]()"))
	if !stripQuotes {
		return parts
	}
	out := parts[:0]
	for _, p := range parts {
		if p = strings.Trim(p, `'"`); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// literal renders value as the SQL right-hand side for the property's type.
func (s propSpec) literal(prop, value string) (string, error) {
	switch s.typ {
	case tBool:
		return mustBeOneOf(prop, value, "TRUE", "FALSE")
	case tInt:
		n, err := strconv.ParseUint(value, 10, 63)
		if err != nil {
			return "", fmt.Errorf("%s must be a non-negative integer", prop)
		}
		return strconv.FormatUint(n, 10), nil
	case tEnum:
		return mustBeOneOf(prop, value, s.options...)
	case tText:
		return snowflake.QuoteTextLit(value), nil
	case tIdent:
		return validateIdentRef(value)
	case tList:
		toks := listTokens(value, true)
		if len(toks) == 0 {
			return "", fmt.Errorf("%s requires at least one value", prop)
		}
		for i, t := range toks {
			toks[i] = snowflake.QuoteTextLit(t)
		}
		return "(" + strings.Join(toks, ", ") + ")", nil
	case tIdentList:
		if kw, err := mustBeOneOf(prop, value, s.options...); err == nil {
			return kw, nil
		}
		toks := listTokens(value, false)
		if len(toks) == 0 {
			return "", fmt.Errorf("%s requires at least one value", prop)
		}
		for _, t := range toks {
			if _, err := validateIdentRef(t); err != nil {
				return "", fmt.Errorf("%s: %w", prop, err)
			}
		}
		return "(" + strings.Join(toks, ", ") + ")", nil
	}
	return "", fmt.Errorf("%s: unsupported value type %q", prop, s.typ)
}

// BuildAlterIntegrationPropertySQL builds
// `ALTER <KIND> INTEGRATION <name> SET <PROP> = <value>` for one property, or
// `… UNSET <PROP>` when value is empty and the grammar allows unsetting it.
// Only properties in the alterable allow-list are accepted; the value is
// validated and quoted according to the property's type.
func BuildAlterIntegrationPropertySQL(kind, name, property, value string) (string, error) {
	k, err := NormalizeKind(kind)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("integration name is required")
	}
	prop := strings.ToUpper(strings.TrimSpace(property))
	spec, ok := specFor(k, prop)
	if !ok {
		return "", fmt.Errorf("property %q cannot be altered on a %s integration", property, k)
	}
	head := fmt.Sprintf("ALTER %s INTEGRATION %s", k, snowflake.QuoteIdent(name))
	value = strings.TrimSpace(value)
	if value == "" {
		if spec.unset {
			return head + " UNSET " + prop, nil
		}
		// Only a plain text property (e.g. COMMENT where UNSET isn't in the
		// grammar) may be set to the empty string.
		if spec.typ != tText || spec.secret {
			return "", fmt.Errorf("%s requires a value", prop)
		}
	}
	lit, err := spec.literal(prop, value)
	if err != nil {
		return "", err
	}
	assign := prop + " = " + lit
	if spec.wrap != "" {
		assign = spec.wrap + " = (" + assign + ")"
	}
	return head + " SET " + assign, nil
}

// Property is one DESCRIBE INTEGRATION row plus the edit hints the Properties
// modal renders from. Editable rows save through
// BuildAlterIntegrationPropertySQL; everything else is read-only.
type Property struct {
	Name    string `json:"name"`
	Type    string `json:"type"`    // DESCRIBE property_type
	Value   string `json:"value"`   // DESCRIBE property_value
	Default string `json:"default"` // DESCRIBE property_default

	Editable bool `json:"editable"`
	// Editor is the value type: boolean | number | text | select | list | ident | identList.
	Editor string `json:"editor"`
	// Options: select choices, or the keywords an identList accepts in place of a list.
	Options []string `json:"options"`
	// Unsettable: clearing the value emits UNSET.
	Unsettable bool `json:"unsettable"`
	// Secret: the value is never returned by DESCRIBE — render a set-only row.
	Secret bool `json:"secret"`
}

// AnnotateProperties marks the DESCRIBE INTEGRATION rows that are editable for
// the given kind and appends a set-only row for each applicable secret that
// DESCRIBE doesn't return. An unknown kind leaves every row read-only.
func AnnotateProperties(kind string, rows []Property) []Property {
	k := strings.ToUpper(strings.TrimSpace(kind))
	present := make(map[string]bool, len(rows))
	for _, r := range rows {
		present[strings.ToUpper(r.Name)] = true
	}
	applies := func(s propSpec) bool {
		return len(s.requires) == 0 || slices.ContainsFunc(s.requires, func(p string) bool { return present[p] })
	}
	annotate := func(p *Property, s propSpec) {
		p.Editable = true
		p.Editor = s.typ
		p.Options = append([]string{}, s.options...)
		p.Unsettable = s.unset
		p.Secret = s.secret
	}
	for i := range rows {
		rows[i].Options = []string{}
		if _, known := alterable[k]; !known {
			continue
		}
		if s, ok := specFor(k, strings.ToUpper(rows[i].Name)); ok && applies(s) {
			annotate(&rows[i], s)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(alterable[k])) {
		if s := alterable[k][name]; s.secret && !present[name] && applies(s) {
			p := Property{Name: name}
			annotate(&p, s)
			rows = append(rows, p)
		}
	}
	return rows
}
