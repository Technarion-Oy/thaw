// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/keypair"
	"thaw/internal/snowflake"
	"thaw/internal/users"
)

// ListUsers returns all users visible to the current role.
// Returns an error if the role lacks the required privilege.
func (a *App) ListUsers() ([]snowflake.SnowflakeUser, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListUsers(a.fctx(FeatureUsersRoles))
}

// GetUserDDL returns a CREATE USER DDL statement for the given user.
func (a *App) GetUserDDL(name string) (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	return client.GetUserDDL(a.fctx(FeatureUsersRoles), name)
}

// AlterUserProperty applies a single SET/UNSET property change to a user.
// property must be one of the keys documented on users.BuildAlterUserPropertySQL
// (loginName, email, defaultWarehouse, minsToBypassMfa, type, …); an empty
// value UNSETs the property where Snowflake allows it.
func (a *App) AlterUserProperty(name, property, value string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return users.AlterProperty(a.fctx(FeatureUsersRoles), client, name, property, value)
}

// ResetUserPassword runs ALTER USER … RESET PASSWORD and returns Snowflake's
// status message, which carries the generated single-use password reset URL the
// admin must relay to the user. Each call issues a fresh one-time link.
func (a *App) ResetUserPassword(name string) (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	return users.ResetPassword(a.fctx(FeatureUsersRoles), client, name)
}

// RenameUser runs ALTER USER … RENAME TO. newName is typed free-hand (bare names
// fold; a name needing quoting must be typed quoted).
func (a *App) RenameUser(name, newName string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return users.Rename(a.fctx(FeatureUsersRoles), client, name, newName)
}

// AbortAllUserQueries runs ALTER USER … ABORT ALL QUERIES, canceling every
// running and queued query for the user across all sessions.
func (a *App) AbortAllUserQueries(name string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return users.AbortAllQueries(a.fctx(FeatureUsersRoles), client, name)
}

// RemoveUserMfaMethod runs ALTER USER … REMOVE MFA METHOD <method>. method is one
// of PASSKEY, TOTP, DUO.
func (a *App) RemoveUserMfaMethod(name, method string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return users.RemoveMfaMethod(a.fctx(FeatureUsersRoles), client, name, method)
}

// SetUserPolicy runs ALTER USER … SET { AUTHENTICATION | PASSWORD | SESSION }
// POLICY <policy_name> [ FORCE ]. kind is the policy kind; force detaches any
// same-kind policy already attached before attaching the new one.
func (a *App) SetUserPolicy(name, kind, policyName string, force bool) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return users.SetPolicy(a.fctx(FeatureUsersRoles), client, name, kind, policyName, force)
}

// UnsetUserPolicy runs ALTER USER … UNSET { AUTHENTICATION | PASSWORD | SESSION }
// POLICY.
func (a *App) UnsetUserPolicy(name, kind string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return users.UnsetPolicy(a.fctx(FeatureUsersRoles), client, name, kind)
}

// GetUserTagReferences returns the tags applied directly to the given user via
// the INFORMATION_SCHEMA.TAG_REFERENCES table function (object domain USER).
// Unlike the ACCOUNT_USAGE.TAG_REFERENCES view this reflects changes immediately
// (no propagation latency), so it backs the removable-chip tag editor in the
// user properties modal — mirroring App.GetModelTags.
//
// USER is an account-level object, so the reference is a bare user name and the
// results aren't scoped to any database — but an INFORMATION_SCHEMA table
// function still has to run inside *some* database. The session's current
// database is used when set; otherwise (common for account admins who connect
// without selecting one) any accessible database is picked, since the account-
// level result is identical regardless of which database's INFORMATION_SCHEMA
// hosts the call. When no database is accessible at all an error is returned —
// the caller treats that (and any other failure) as "no tags shown" and still
// allows SET/UNSET TAG.
func (a *App) GetUserTagReferences(name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	ctx := a.fctx(FeatureUsersRoles)
	db := client.GetCachedSessionContext().Database
	if db == "" {
		dbs, err := client.ListDatabases(ctx)
		if err != nil {
			return nil, err
		}
		if len(dbs) == 0 {
			return nil, fmt.Errorf("no accessible database to read user tags")
		}
		db = dbs[0]
	}
	sql := fmt.Sprintf(
		"SELECT TAG_DATABASE, TAG_SCHEMA, TAG_NAME, TAG_VALUE "+
			"FROM TABLE(%s.INFORMATION_SCHEMA.TAG_REFERENCES('%s', 'USER')) "+
			"ORDER BY TAG_DATABASE, TAG_SCHEMA, TAG_NAME",
		// EscapeTextLit (not EscapeStringLit): QuoteIdent doubles " but not \, so a
		// backslash in an identifier must be doubled to survive the single-quoted
		// literal rather than being read as a Snowflake escape sequence.
		snowflake.QuoteIdent(db), snowflake.EscapeTextLit(snowflake.QuoteIdent(name)))
	// QuerySingle (not Execute): its doc recommends it for TABLE() function calls,
	// matching sibling readers like GetAuthenticationPolicyReferences.
	return client.QuerySingle(ctx, sql)
}

// SetUserTags runs ALTER USER … SET TAG <t1> = '<v1>' [ , … ].
func (a *App) SetUserTags(name string, tags []users.TagPair) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return users.SetTags(a.fctx(FeatureUsersRoles), client, name, tags)
}

// UnsetUserTags runs ALTER USER … UNSET TAG <t1> [ , … ].
func (a *App) UnsetUserTags(name string, tagNames []string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return users.UnsetTags(a.fctx(FeatureUsersRoles), client, name, tagNames)
}

// AddUserDelegatedAuth runs ALTER USER … ADD DELEGATED AUTHORIZATION OF ROLE
// <role> TO SECURITY INTEGRATION <integration>.
func (a *App) AddUserDelegatedAuth(name, role, integration string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return users.AddDelegatedAuth(a.fctx(FeatureUsersRoles), client, name, role, integration)
}

// RemoveUserDelegatedAuth runs ALTER USER … REMOVE DELEGATED … FROM SECURITY
// INTEGRATION <integration>. An empty role removes every delegated authorization
// for the integration (the AUTHORIZATIONS form); a role removes just that one.
func (a *App) RemoveUserDelegatedAuth(name, role, integration string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return users.RemoveDelegatedAuth(a.fctx(FeatureUsersRoles), client, name, role, integration)
}

// CheckAvailableKeyTools returns the list of available key generation methods.
// "go" (Go built-in crypto) is always present. "openssl" and "ssh-keygen" are
// included only when their executables are found on PATH.
func (a *App) CheckAvailableKeyTools() []string {
	return keypair.CheckAvailableKeyTools()
}

// GenerateKeyPair generates an RSA-2048 key pair using the specified method
// ("go", "openssl", or "ssh-keygen").
//
// Registering the generated public key with a user goes through
// AlterUserProperty(name, "rsaPublicKey"|"rsaPublicKey2", value) — one tested SQL
// builder in internal/users — so there is no separate set-public-key IPC.
func (a *App) GenerateKeyPair(method, privateKeyPath, passphrase string) (keypair.KeyPairResult, error) {
	return keypair.GenerateKeyPair(method, privateKeyPath, passphrase)
}
