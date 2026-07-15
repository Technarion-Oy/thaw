// SPDX-License-Identifier: GPL-3.0-or-later

// Entry point for the monaco-yaml web worker bundled by Vite.
// The monaco-yaml yaml.worker module self-initializes on import — no
// explicit initialize() call is needed.
import "monaco-yaml/yaml.worker";
