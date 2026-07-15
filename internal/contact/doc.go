// SPDX-License-Identifier: GPL-3.0-or-later

// Package contact builds SQL for Snowflake CONTACT objects — the CREATE CONTACT
// statement plus the structured config behind it. A contact is a schema-level
// object that names a notification target used by alerts and other
// notification-based features. It carries exactly one contact method plus an
// optional comment:
//
//	CREATE [OR REPLACE] CONTACT [IF NOT EXISTS] <fqn>
//	  [ { USERS = ('u1' [, 'u2' ...])
//	    | EMAIL_DISTRIBUTION_LIST = '<email>'
//	    | URL = '<url>' } ]
//	  [ COMMENT = '<string>' ]
//
// The three methods are mutually exclusive — a contact has a single "type" of
// USERS, EMAIL_DISTRIBUTION_LIST, or URL. ALTER CONTACT supports RENAME TO and a
// SET of the same method/comment options (no UNSET), all reachable from the
// sidebar (RENAME via the context-menu Rename… item) and the properties panel
// (App.AlterContact). GET_DDL supports contacts directly (object_type
// 'CONTACT'), so View Definition / Compare work without any kind normalization.
//
// thaw:domain: Object Browser & Administration
package contact
