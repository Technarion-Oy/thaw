// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package authenticationpolicy

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"thaw/internal/snowflake"
)

// bareTokenRe matches a safe bare SQL keyword/identifier — letters, digits, and
// underscores only. The builders interpolate enum keywords, provider names, and
// driver names UNQUOTED (that is the grammar), so a value containing ')' or ';'
// could break out of a bag's parentheses. These values come from fixed Selects
// in the UI, but App.Build*Value are exported bound IPC methods that accept
// arbitrary input, so each bare token is validated here as defense-in-depth —
// a token that isn't a plain identifier is dropped rather than emitted.
var bareTokenRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func isBareToken(s string) bool { return bareTokenRe.MatchString(s) }

// This file models the four nested "property-bag" parameters of an
// authentication policy — MFA_POLICY, PAT_POLICY, WORKLOAD_IDENTITY_POLICY, and
// CLIENT_POLICY. Each is a parenthesized list of sub-properties with its own
// grammar. The frontend collects the values structurally (selects / numbers /
// toggles) but defers BOTH directions of the conversion to these builders /
// parsers (exposed over IPC via App.Build*Value / App.Parse*) so no SQL
// serialization or DESCRIBE parsing logic lives in TypeScript.
//
// Builders emit the value that follows `=` in an ALTER … SET <BAG> = <value>
// clause: a parenthesized, space-delimited property list (CLIENT_POLICY uses a
// comma-delimited entry list). Only sub-properties the caller set are emitted.
//
// Parsers read the value DESCRIBE AUTHENTICATION POLICY reports for a bag back
// into the struct so the editor can pre-fill. DESCRIBE renders these bags in
// Snowflake's structured-object notation — `{KEY=VALUE, KEY={NESTED=VALUE}}`,
// e.g. CLIENT_POLICY → `{GO_DRIVER={MINIMUM_VERSION=3.14.1}}` — NOT JSON, so the
// parsers run that grammar through parseDescribeBag (strict JSON is still accepted
// as a fallback). They are tolerant — an unrecognized / empty value yields a zero
// struct (the editor simply starts blank) rather than an error.
//
// The quoting / delimiter rules below are NOT uniform across the bags — each
// builder follows its own sub-grammar, verified verbatim against the
// CREATE AUTHENTICATION POLICY reference
// (https://docs.snowflake.com/en/sql-reference/sql/create-authentication-policy):
//
//	MFA_POLICY = ( ALLOWED_METHODS = ('TOTP', 'DUO')              -- list quoted, space-delimited props
//	               ENFORCE_MFA_ON_EXTERNAL_AUTHENTICATION = 'NONE' )   -- enum quoted
//	PAT_POLICY = ( DEFAULT_EXPIRY_IN_DAYS = 30                    -- numbers bare, space-delimited props
//	               NETWORK_POLICY_EVALUATION = ENFORCED_NOT_REQUIRED   -- enum BARE (unlike MFA's enum)
//	               REQUIRE_ROLE_RESTRICTION_FOR_SERVICE_USERS = FALSE ) -- bool bare
//	WORKLOAD_IDENTITY_POLICY = ( ALLOWED_PROVIDERS = (AWS, AZURE)  -- providers BARE (unlike ALLOWED_METHODS)
//	               ALLOWED_AWS_ACCOUNTS = ('123456789012') )      -- account/issuer lists quoted
//	CLIENT_POLICY = ( GO_DRIVER = (MINIMUM_VERSION = '1.14.1'),   -- entries COMMA-delimited (unlike the others)
//	               JDBC_DRIVER = (MINIMUM_VERSION = '3.25.0') )   -- version quoted, driver bare
//
// So: do not "normalize" these into a single quoting/delimiter rule — the
// asymmetry is the grammar. (That asymmetry is on the *serializer* side; the
// parsers read DESCRIBE's uniform structured-object rendering, so a single
// parseDescribeBag scanner handles all four.)

// ── MFA_POLICY ───────────────────────────────────────────────────────────────

// MFAPolicy models the MFA_POLICY bag: which MFA methods are allowed and whether
// MFA is enforced for externally-authenticated logins.
type MFAPolicy struct {
	// ALLOWED_METHODS — any of ALL, PASSKEY, TOTP, OTP, DUO (quoted literals).
	AllowedMethods []string `json:"allowedMethods"`
	// ENFORCE_MFA_ON_EXTERNAL_AUTHENTICATION — ALL or NONE (quoted literal).
	EnforceMFAOnExternalAuthentication string `json:"enforceMfaOnExternalAuthentication"`
}

// BuildMFAPolicyValue serializes p into the `( … )` value for SET MFA_POLICY.
func BuildMFAPolicyValue(p MFAPolicy) string {
	var props []string
	if snowflake.HasNonBlankToken(p.AllowedMethods) {
		props = append(props, "ALLOWED_METHODS = "+snowflake.FormatStringLitList(p.AllowedMethods))
	}
	if v := strings.TrimSpace(p.EnforceMFAOnExternalAuthentication); v != "" {
		props = append(props, "ENFORCE_MFA_ON_EXTERNAL_AUTHENTICATION = '"+snowflake.EscapeTextLit(strings.ToUpper(v))+"'")
	}
	return wrapProps(props)
}

// ParseMFAPolicy reads a DESCRIBE MFA_POLICY value back into the struct.
func ParseMFAPolicy(raw string) MFAPolicy {
	m := parseDescribeBag(raw)
	return MFAPolicy{
		AllowedMethods:                     jsonStringList(m, "ALLOWED_METHODS"),
		EnforceMFAOnExternalAuthentication: jsonString(m, "ENFORCE_MFA_ON_EXTERNAL_AUTHENTICATION"),
	}
}

// ── PAT_POLICY ───────────────────────────────────────────────────────────────

// PAT token-lifetime bounds, per the CREATE AUTHENTICATION POLICY reference:
// DEFAULT_EXPIRY_IN_DAYS and MAX_EXPIRY_IN_DAYS both accept 1 to 365 days.
const (
	patExpiryMinDays = 1
	patExpiryMaxDays = 365
)

// PATPolicy models the PAT_POLICY bag governing programmatic access tokens.
// The two day counts are *int and the boolean a *bool so the builder can tell
// "leave unset" (nil) apart from a deliberate value (including 0 / false).
type PATPolicy struct {
	DefaultExpiryInDays                   *int   `json:"defaultExpiryInDays"`                   // DEFAULT_EXPIRY_IN_DAYS
	MaxExpiryInDays                       *int   `json:"maxExpiryInDays"`                       // MAX_EXPIRY_IN_DAYS
	NetworkPolicyEvaluation               string `json:"networkPolicyEvaluation"`               // ENFORCED_REQUIRED | ENFORCED_NOT_REQUIRED | NOT_ENFORCED
	RequireRoleRestrictionForServiceUsers *bool  `json:"requireRoleRestrictionForServiceUsers"` // TRUE | FALSE
}

// BuildPATPolicyValue serializes p into the `( … )` value for SET PAT_POLICY.
// The expiry day counts are range-checked against the documented 1–365 bound and
// dropped when out of range — defense-in-depth on this exported IPC method (the
// UI already clamps the inputs), matching the bare-token guards elsewhere here.
func BuildPATPolicyValue(p PATPolicy) string {
	var props []string
	if p.DefaultExpiryInDays != nil && inPATExpiryRange(*p.DefaultExpiryInDays) {
		props = append(props, fmt.Sprintf("DEFAULT_EXPIRY_IN_DAYS = %d", *p.DefaultExpiryInDays))
	}
	if p.MaxExpiryInDays != nil && inPATExpiryRange(*p.MaxExpiryInDays) {
		props = append(props, fmt.Sprintf("MAX_EXPIRY_IN_DAYS = %d", *p.MaxExpiryInDays))
	}
	if v := strings.ToUpper(strings.TrimSpace(p.NetworkPolicyEvaluation)); v != "" && isBareToken(v) {
		props = append(props, "NETWORK_POLICY_EVALUATION = "+v)
	}
	if p.RequireRoleRestrictionForServiceUsers != nil {
		b := "FALSE"
		if *p.RequireRoleRestrictionForServiceUsers {
			b = "TRUE"
		}
		props = append(props, "REQUIRE_ROLE_RESTRICTION_FOR_SERVICE_USERS = "+b)
	}
	return wrapProps(props)
}

func inPATExpiryRange(days int) bool {
	return days >= patExpiryMinDays && days <= patExpiryMaxDays
}

// ParsePATPolicy reads a DESCRIBE PAT_POLICY value back into the struct.
func ParsePATPolicy(raw string) PATPolicy {
	m := parseDescribeBag(raw)
	return PATPolicy{
		DefaultExpiryInDays:                   jsonIntPtr(m, "DEFAULT_EXPIRY_IN_DAYS"),
		MaxExpiryInDays:                       jsonIntPtr(m, "MAX_EXPIRY_IN_DAYS"),
		NetworkPolicyEvaluation:               jsonString(m, "NETWORK_POLICY_EVALUATION"),
		RequireRoleRestrictionForServiceUsers: jsonBoolPtr(m, "REQUIRE_ROLE_RESTRICTION_FOR_SERVICE_USERS"),
	}
}

// ── WORKLOAD_IDENTITY_POLICY ─────────────────────────────────────────────────

// WorkloadIdentityPolicy models the WORKLOAD_IDENTITY_POLICY bag. The providers
// are bare keywords (ALL/AWS/AZURE/GCP/OIDC); the account/issuer lists are
// quoted string literals.
type WorkloadIdentityPolicy struct {
	AllowedProviders    []string `json:"allowedProviders"`    // ALL | AWS | AZURE | GCP | OIDC (bare)
	AllowedAWSAccounts  []string `json:"allowedAwsAccounts"`  // 12-digit account IDs (quoted)
	AllowedAzureIssuers []string `json:"allowedAzureIssuers"` // authority URLs (quoted)
	AllowedOIDCIssuers  []string `json:"allowedOidcIssuers"`  // HTTPS URLs (quoted)
}

// BuildWorkloadIdentityPolicyValue serializes p into the `( … )` value for
// SET WORKLOAD_IDENTITY_POLICY.
func BuildWorkloadIdentityPolicyValue(p WorkloadIdentityPolicy) string {
	var props []string
	if snowflake.HasNonBlankToken(p.AllowedProviders) {
		props = append(props, "ALLOWED_PROVIDERS = "+formatBareList(p.AllowedProviders))
	}
	if snowflake.HasNonBlankToken(p.AllowedAWSAccounts) {
		props = append(props, "ALLOWED_AWS_ACCOUNTS = "+snowflake.FormatStringLitList(p.AllowedAWSAccounts))
	}
	if snowflake.HasNonBlankToken(p.AllowedAzureIssuers) {
		props = append(props, "ALLOWED_AZURE_ISSUERS = "+snowflake.FormatStringLitList(p.AllowedAzureIssuers))
	}
	if snowflake.HasNonBlankToken(p.AllowedOIDCIssuers) {
		props = append(props, "ALLOWED_OIDC_ISSUERS = "+snowflake.FormatStringLitList(p.AllowedOIDCIssuers))
	}
	return wrapProps(props)
}

// ParseWorkloadIdentityPolicy reads a DESCRIBE WORKLOAD_IDENTITY_POLICY value
// back into the struct.
func ParseWorkloadIdentityPolicy(raw string) WorkloadIdentityPolicy {
	m := parseDescribeBag(raw)
	return WorkloadIdentityPolicy{
		AllowedProviders:    jsonStringList(m, "ALLOWED_PROVIDERS"),
		AllowedAWSAccounts:  jsonStringList(m, "ALLOWED_AWS_ACCOUNTS"),
		AllowedAzureIssuers: jsonStringList(m, "ALLOWED_AZURE_ISSUERS"),
		AllowedOIDCIssuers:  jsonStringList(m, "ALLOWED_OIDC_ISSUERS"),
	}
}

// ── CLIENT_POLICY ────────────────────────────────────────────────────────────

// ClientPolicyEntry pins a single driver/client to a minimum version.
type ClientPolicyEntry struct {
	Driver         string `json:"driver"`         // e.g. JDBC_DRIVER
	MinimumVersion string `json:"minimumVersion"` // e.g. 3.13.0
}

// ClientPolicy models the CLIENT_POLICY bag: a comma-delimited list of
// `<driver> = ( MINIMUM_VERSION = '<version>' )` entries.
type ClientPolicy struct {
	Entries []ClientPolicyEntry `json:"entries"`
}

// BuildClientPolicyValue serializes p into the `( … )` value for
// SET CLIENT_POLICY. Entries missing a driver or version are skipped, as is a
// repeated driver (first occurrence wins) — a duplicate key would make an invalid
// bag; the UI also blocks duplicates, so this is defense-in-depth on the exported
// method.
func BuildClientPolicyValue(p ClientPolicy) string {
	var entries []string
	seen := make(map[string]bool)
	for _, e := range p.Entries {
		d := strings.ToUpper(strings.TrimSpace(e.Driver))
		v := strings.TrimSpace(e.MinimumVersion)
		// The driver name is interpolated bare, so it must be a plain identifier;
		// the version is quoted/escaped below so it needs no such guard.
		if d == "" || v == "" || !isBareToken(d) || seen[d] {
			continue
		}
		seen[d] = true
		entries = append(entries, fmt.Sprintf("%s = ( MINIMUM_VERSION = '%s' )", d, snowflake.EscapeTextLit(v)))
	}
	if len(entries) == 0 {
		return "()"
	}
	return "( " + strings.Join(entries, ", ") + " )"
}

// ParseClientPolicy reads a DESCRIBE CLIENT_POLICY value (a JSON object keyed by
// driver name, each holding a MINIMUM_VERSION) back into the struct. Entries are
// sorted by driver so the editor renders deterministically.
func ParseClientPolicy(raw string) ClientPolicy {
	m := parseDescribeBag(raw)
	var cp ClientPolicy
	for driver, v := range m {
		obj, ok := v.(map[string]any)
		if !ok {
			continue
		}
		var ver string
		for ik, iv := range obj {
			if strings.EqualFold(ik, "MINIMUM_VERSION") {
				// Coerce via jsonScalarString so a numeric version in the strict-JSON
				// fallback (e.g. MINIMUM_VERSION: 3.14) isn't dropped — the structured
				// path always yields strings, but this matches the AWS-account path.
				ver = jsonScalarString(iv)
			}
		}
		if ver != "" {
			cp.Entries = append(cp.Entries, ClientPolicyEntry{Driver: driver, MinimumVersion: ver})
		}
	}
	sort.Slice(cp.Entries, func(i, j int) bool { return cp.Entries[i].Driver < cp.Entries[j].Driver })
	return cp
}

// ── shared helpers ───────────────────────────────────────────────────────────

// wrapProps wraps a space-delimited property list in parentheses (or returns the
// empty list `()` when nothing was set).
func wrapProps(props []string) string {
	if len(props) == 0 {
		return "()"
	}
	return "( " + strings.Join(props, " ") + " )"
}

// formatBareList renders a token slice into a parenthesized bare-keyword list,
// e.g. []string{"AWS","AZURE"} → "(AWS, AZURE)". Blank tokens are skipped.
func formatBareList(tokens []string) string {
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.ToUpper(strings.TrimSpace(t))
		// Providers are emitted bare, so drop anything that isn't a plain
		// identifier (defense-in-depth — the UI only offers fixed keywords).
		if t == "" || !isBareToken(t) {
			continue
		}
		parts = append(parts, t)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// parseDescribeBag parses the value DESCRIBE AUTHENTICATION POLICY reports for a
// nested property bag into a generic map keyed by upper-cased top-level property.
// DESCRIBE renders these bags in Snowflake's structured-object notation — '='
// between key and value, unquoted keys/scalars, and nested `{}` / `[]` groups,
// e.g. CLIENT_POLICY → `{GO_DRIVER={MINIMUM_VERSION=3.14.1}}` — NOT JSON. (Format
// confirmed against the DESCRIBE AUTHENTICATION POLICY reference:
// https://docs.snowflake.com/en/sql-reference/sql/desc-authentication-policy.)
// A strict-JSON rendering is accepted as a fallback. Scalars come back as strings;
// the jsonIntPtr / jsonBoolPtr / jsonStringList helpers coerce them. Empty or
// unparseable input yields a nil map (reads from a nil map return zero values, so
// callers need no special-casing).
func parseDescribeBag(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	// Some editions may emit strict JSON; the structured notation never parses as
	// JSON (unquoted keys, '=' instead of ':'), so a successful unmarshal is JSON.
	// Decode numbers as json.Number (not float64) so a numeric value keeps its
	// original token — a version like 3.10 must not collapse to 3.1, and a 12-digit
	// account ID must not round-trip through a float.
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var jm map[string]any
	if dec.Decode(&jm) == nil {
		return upperKeys(jm)
	}
	s := &structScanner{s: raw}
	if obj, ok := s.parseValue().(map[string]any); ok {
		return upperKeys(obj)
	}
	return nil
}

// upperKeys returns m with its top-level keys upper-cased, or nil when empty so
// downstream reads fall through to zero values.
func upperKeys(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[strings.ToUpper(k)] = v
	}
	return out
}

// structScanner parses Snowflake's structured-object string rendering into Go
// values: `{...}` → map[string]any, `[...]` → []any, and any bare/quoted run → a
// trimmed string scalar. A `(...)` group is parsed too — it maps to an object when
// its entries carry `=` (the SQL bag grammar, `( KEY = VALUE … )`) or to a list
// otherwise (a value tuple, `('TOTP', 'DUO')`) — so a parenthesized DESCRIBE
// rendering parses as well as the documented brace form. Keys and values are
// separated by '='; entries by ',' or whitespace, so both the comma-delimited
// DESCRIBE rendering and the grammar's space-delimited form parse. A surrounding
// single/double-quote pair is unwrapped so a quoted value may contain a delimiter.
type structScanner struct {
	s   string
	pos int
}

func (p *structScanner) parseValue() any {
	p.skipSpace()
	if p.pos >= len(p.s) {
		return ""
	}
	switch p.s[p.pos] {
	case '{':
		return p.parseObject()
	case '[':
		return p.parseArray()
	case '(':
		return p.parseGroup()
	default:
		return p.parseToken()
	}
}

// parseGroup parses a `(...)` group, disambiguating the two shapes Snowflake's
// paren grammar uses: a property bag `( KEY = VALUE … )` becomes a map, while a
// value tuple `('A', 'B')` becomes a list. An entry whose first token is followed
// by '=' is a key/value pair; any other entry is a list element. If any pair is
// seen the group is an object (stray bare elements are dropped); otherwise it is
// the collected list.
func (p *structScanner) parseGroup() any {
	obj := map[string]any{}
	var list []any
	sawPair := false
	p.pos++ // consume '('
	for p.pos < len(p.s) {
		start := p.pos
		p.skipSpace()
		if p.pos < len(p.s) && p.s[p.pos] == ')' {
			p.pos++
			break
		}
		first := p.parseValue()
		p.skipSpace()
		if p.pos < len(p.s) && p.s[p.pos] == '=' {
			p.pos++ // consume '='
			val := p.parseValue()
			if key, ok := first.(string); ok && key != "" {
				obj[key] = val
				sawPair = true
			}
		} else {
			list = append(list, first)
		}
		p.skipSpace()
		if p.pos < len(p.s) && p.s[p.pos] == ',' {
			p.pos++ // consume entry separator
		}
		if p.pos == start { // guarantee forward progress on malformed input
			p.pos++
		}
	}
	if sawPair {
		return obj
	}
	return list
}

func (p *structScanner) parseObject() map[string]any {
	out := map[string]any{}
	p.pos++ // consume '{'
	for p.pos < len(p.s) {
		start := p.pos
		p.skipSpace()
		if p.pos < len(p.s) && p.s[p.pos] == '}' {
			p.pos++
			break
		}
		key := p.parseToken()
		p.skipSpace()
		if p.pos < len(p.s) && p.s[p.pos] == '=' {
			p.pos++ // consume '='
			val := p.parseValue()
			if key != "" {
				out[key] = val
			}
		}
		p.skipSpace()
		if p.pos < len(p.s) && p.s[p.pos] == ',' {
			p.pos++ // consume entry separator
		}
		if p.pos == start { // guarantee forward progress on malformed input
			p.pos++
		}
	}
	return out
}

func (p *structScanner) parseArray() []any {
	var out []any
	p.pos++ // consume '['
	for p.pos < len(p.s) {
		start := p.pos
		p.skipSpace()
		if p.pos < len(p.s) && p.s[p.pos] == ']' {
			p.pos++
			break
		}
		out = append(out, p.parseValue())
		p.skipSpace()
		if p.pos < len(p.s) && p.s[p.pos] == ',' {
			p.pos++ // consume element separator
		}
		if p.pos == start { // guarantee forward progress on malformed input
			p.pos++
		}
	}
	return out
}

// parseToken reads a bare scalar (or quoted string) up to the next structural
// delimiter — whitespace, '=', ',', or a brace/bracket/paren — unwrapping a
// surrounding single/double-quote pair. A doubled quote inside the string (” or
// "") is the SQL escape for a literal quote and is preserved as one (matching the
// shared sqltok tokenizer used by ParseSqlList).
func (p *structScanner) parseToken() string {
	p.skipSpace()
	var sb strings.Builder
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == '\'' || c == '"' {
			p.pos++ // opening quote
			for p.pos < len(p.s) {
				if p.s[p.pos] == c {
					// A doubled quote is an escaped literal quote, not the close.
					if p.pos+1 < len(p.s) && p.s[p.pos+1] == c {
						sb.WriteByte(c)
						p.pos += 2
						continue
					}
					p.pos++ // closing quote
					break
				}
				sb.WriteByte(p.s[p.pos])
				p.pos++
			}
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
			c == '=' || c == ',' || c == '{' || c == '}' || c == '[' || c == ']' ||
			c == '(' || c == ')' {
			break
		}
		sb.WriteByte(c)
		p.pos++
	}
	return strings.TrimSpace(sb.String())
}

func (p *structScanner) skipSpace() {
	for p.pos < len(p.s) {
		switch p.s[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func jsonString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// jsonScalarString renders a JSON scalar into string form. Numbers decoded with
// UseNumber arrive as json.Number, so the original token is returned verbatim — a
// large account/ID number survives intact and a version keeps its trailing zero
// (3.10 stays "3.10", not "3.1"). A bare float64 (no UseNumber) is rendered
// without an exponent as a fallback. Non-scalars yield "".
func jsonScalarString(v any) string {
	switch n := v.(type) {
	case string:
		return n
	case json.Number:
		return n.String()
	case float64:
		if i := int64(n); float64(i) == n {
			return strconv.FormatInt(i, 10)
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	case bool:
		if n {
			return "TRUE"
		}
		return "FALSE"
	}
	return ""
}

func jsonStringList(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			// The structScanner always yields string scalars, but a strict-JSON
			// DESCRIBE rendering could emit a bare number (e.g. a 12-digit AWS
			// account as a JSON number). Coerce any non-string scalar via
			// jsonScalarString rather than dropping it.
			if s := jsonScalarString(e); strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		// Tolerate a scalar rendering (some editions emit a single value bare).
		if strings.TrimSpace(t) != "" {
			return []string{t}
		}
	}
	return nil
}

func jsonIntPtr(m map[string]any, key string) *int {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch n := v.(type) {
	case json.Number:
		if iv, err := strconv.Atoi(strings.TrimSpace(n.String())); err == nil {
			return &iv
		}
	case float64:
		i := int(n)
		return &i
	case string:
		if iv, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return &iv
		}
	}
	return nil
}

func jsonBoolPtr(m map[string]any, key string) *bool {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch b := v.(type) {
	case bool:
		return &b
	case string:
		switch strings.ToUpper(strings.TrimSpace(b)) {
		case "TRUE":
			t := true
			return &t
		case "FALSE":
			f := false
			return &f
		}
	}
	return nil
}
