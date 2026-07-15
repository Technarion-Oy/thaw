// SPDX-License-Identifier: GPL-3.0-or-later

// thaw:domain: Core IPC & App Lifecycle

// Package app implements the Wails-bound App struct and its IPC methods.
// The App is the application's composition root: it owns the live Snowflake
// connection, per-tab sessions, and shared runtime state, and exposes
// connection-dependent methods to the frontend. Binding methods delegate to
// the internal/* domain packages for their business logic.
//
// IPC methods are grouped into per-domain files (query.go, objects.go,
// warehouse.go, …) but all belong to this single package, so Wails binds the
// whole method set regardless of which file a method lives in.
package app
