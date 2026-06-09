// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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
