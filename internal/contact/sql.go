// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package contact

import (
	"strings"

	"thaw/internal/snowflake"
)

// Contact method identifiers. A contact carries exactly one of these methods —
// they are mutually exclusive in Snowflake's CREATE/ALTER CONTACT grammar.
const (
	MethodUsers = "users" // USERS = ('u1', 'u2', …)
	MethodEmail = "email" // EMAIL_DISTRIBUTION_LIST = '<email>'
	MethodURL   = "url"   // URL = '<url>'
)

// ContactConfig holds the parameters for creating a Snowflake CONTACT object. A
// contact names a notification target (a set of users, an email distribution
// list, or a URL) used by alerts and other notification-based features. Exactly
// one contact method applies; Method selects which of Users / Email / URL is
// emitted.
type ContactConfig struct {
	Name          string   `json:"name"`
	CaseSensitive bool     `json:"caseSensitive"`
	OrReplace     bool     `json:"orReplace"`
	IfNotExists   bool     `json:"ifNotExists"`
	Method        string   `json:"method"`  // one of MethodUsers/MethodEmail/MethodURL ("" = no method)
	Users         []string `json:"users"`   // Snowflake user names (Method == MethodUsers)
	Email         string   `json:"email"`   // email distribution list (Method == MethodEmail)
	URL           string   `json:"url"`     // contact URL (Method == MethodURL)
	Comment       string   `json:"comment"` // optional COMMENT
}

// FormatContactUsers renders the parenthesised, single-quoted user list emitted
// after `USERS = ` — FormatContactUsers([]string{"ALICE","BOB"}) yields
// "('ALICE', 'BOB')". Blank entries are dropped. Each name is a string literal
// (escaped via EscapeTextLit), matching the CREATE/ALTER CONTACT grammar
// (USERS = ('<user_name>' [, …])). It is exported so the properties panel can
// build the SET USERS clause without duplicating the quoting rules.
func FormatContactUsers(users []string) string {
	clean := snowflake.CleanList(users)
	quoted := make([]string, len(clean))
	for i, u := range clean {
		quoted[i] = snowflake.QuoteTextLit(u)
	}
	return "(" + strings.Join(quoted, ", ") + ")"
}

// methodClause renders the contact-method clause (USERS / EMAIL_DISTRIBUTION_LIST
// / URL) for the CREATE statement, or "" when the selected method has no value
// yet (so the live preview stays a completable template rather than emitting an
// empty clause). The three methods are mutually exclusive; Method selects which
// one is rendered.
func methodClause(cfg ContactConfig) string {
	switch strings.ToLower(strings.TrimSpace(cfg.Method)) {
	case MethodUsers:
		if len(snowflake.CleanList(cfg.Users)) == 0 {
			return ""
		}
		return "\n  USERS = " + FormatContactUsers(cfg.Users)
	case MethodEmail:
		if strings.TrimSpace(cfg.Email) == "" {
			return ""
		}
		return "\n  EMAIL_DISTRIBUTION_LIST = " + snowflake.QuoteTextLit(strings.TrimSpace(cfg.Email))
	case MethodURL:
		if strings.TrimSpace(cfg.URL) == "" {
			return ""
		}
		return "\n  URL = " + snowflake.QuoteTextLit(strings.TrimSpace(cfg.URL))
	}
	return ""
}

// BuildCreateContactSql constructs a CREATE CONTACT statement from the given
// config. When the name is blank a placeholder is substituted so the live
// preview reads as a completable template rather than invalid SQL. OR REPLACE
// and IF NOT EXISTS are mutually exclusive in Snowflake; the create modal
// prevents selecting both, and if both are set here OR REPLACE wins (IF NOT
// EXISTS is dropped by CreateClause).
//
//	CREATE [OR REPLACE] CONTACT [IF NOT EXISTS] <fqn>
//	  [USERS = (…) | EMAIL_DISTRIBUTION_LIST = '…' | URL = '…']
//	  [COMMENT = '…'];
func BuildCreateContactSql(db, schema string, cfg ContactConfig) (string, error) {
	createClause := snowflake.CreateClause("CONTACT", cfg.OrReplace, cfg.IfNotExists)

	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "contact_name"
	}

	var sb strings.Builder
	sb.WriteString(createClause)
	sb.WriteByte(' ')
	sb.WriteString(snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))
	sb.WriteString(methodClause(cfg))
	sb.WriteString(snowflake.CommentClause(cfg.Comment))
	sb.WriteString(";")
	return sb.String(), nil
}
