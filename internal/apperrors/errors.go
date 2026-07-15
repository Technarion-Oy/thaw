// SPDX-License-Identifier: GPL-3.0-or-later

package apperrors

import "errors"

// ErrNotConnected is returned by App methods that require an active Snowflake
// connection when none has been established yet.
var ErrNotConnected = errors.New("no active Snowflake connection")
