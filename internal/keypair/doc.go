// SPDX-License-Identifier: GPL-3.0-or-later

// Package keypair generates RSA key pairs for Snowflake key-pair authentication
// via pure-Go crypto, openssl, or ssh-keygen, and builds the ALTER USER SET
// RSA_PUBLIC_KEY statement that registers a public key with a user.
//
// thaw:domain: Object Browser & Administration
package keypair
