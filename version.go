// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package main

// Version is the application version string. Override at build time with:
//
//	wails build -ldflags "-X main.Version=1.2.3"
//	go build   -ldflags "-X main.Version=1.2.3" .
var Version = "dev"
