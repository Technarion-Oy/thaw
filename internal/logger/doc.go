// SPDX-License-Identifier: GPL-3.0-or-later

// Package logger configures the application-wide slog logger with
// lumberjack log rotation, driven by both size and a startup/periodic
// age-based rotation so old entries are pruned within the retention window.
//
// thaw:domain: Core IPC & App Lifecycle
package logger
