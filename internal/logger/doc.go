// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package logger configures the application-wide slog logger with
// lumberjack log rotation, driven by both size and a startup/periodic
// age-based rotation so old entries are pruned within the retention window.
//
// thaw:domain: Core IPC & App Lifecycle
package logger
