// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
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

// CheckAvailableKeyTools returns the list of available key generation methods.
// "go" (Go built-in crypto) is always present. "openssl" and "ssh-keygen" are
// included only when their executables are found on PATH.
func (a *App) CheckAvailableKeyTools() []string {
	return keypair.CheckAvailableKeyTools()
}

// GenerateKeyPair generates an RSA-2048 key pair using the specified method
// ("go", "openssl", or "ssh-keygen").
func (a *App) GenerateKeyPair(method, privateKeyPath, passphrase string) (keypair.KeyPairResult, error) {
	return keypair.GenerateKeyPair(method, privateKeyPath, passphrase)
}

// SetUserPublicKey applies an RSA public key to a Snowflake user.
func (a *App) SetUserPublicKey(username, publicKey string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	_, err := client.Execute(a.fctx(FeatureUsersRoles), keypair.BuildSetUserPublicKeySQL(username, publicKey))
	return err
}
