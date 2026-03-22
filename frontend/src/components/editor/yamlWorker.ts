// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Entry point for the monaco-yaml web worker bundled by Vite.
// The monaco-yaml yaml.worker module self-initializes on import — no
// explicit initialize() call is needed.
import "monaco-yaml/yaml.worker";
