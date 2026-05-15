// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package apperrors

import "errors"

// ErrNotConnected is returned by App methods that require an active Snowflake
// connection when none has been established yet.
var ErrNotConnected = errors.New("no active Snowflake connection")
