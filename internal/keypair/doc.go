// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package keypair generates RSA key pairs for Snowflake key-pair authentication
// via pure-Go crypto, openssl, or ssh-keygen, and builds the ALTER USER SET
// RSA_PUBLIC_KEY statement that registers a public key with a user.
//
// thaw:domain: Object Browser & Administration
package keypair
