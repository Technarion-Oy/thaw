// SPDX-License-Identifier: GPL-3.0-or-later

package streamlit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MainFileResult is the outcome of detecting a Streamlit app's entrypoint in a
// local folder.
type MainFileResult struct {
	// MainFile is the chosen entrypoint relative to the folder root
	// ("streamlit_app.py" or "app.py"), or "" when neither preferred name is
	// present and the caller must pick from Candidates.
	MainFile string `json:"mainFile"`
	// Candidates lists every *.py file at the folder root (base names, sorted).
	// It is always populated — even when MainFile was detected — so the UI can
	// offer the full set and let the user override the default. Empty when the
	// root holds no Python files.
	Candidates []string `json:"candidates"`
}

// DetectStreamlitMainFile inspects the root of a local Streamlit app folder and
// picks its entrypoint. It prefers streamlit_app.py, then app.py; when neither is
// present MainFile is empty and the caller chooses from Candidates.
//
// Only the folder root is scanned: additional pages under pages/ are not
// entrypoints, so subdirectories are ignored. Hidden files (dot-prefixed, e.g.
// .DS_Store) are skipped, and the ".py" extension is matched case-insensitively.
func DetectStreamlitMainFile(dir string) (MainFileResult, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return MainFileResult{}, fmt.Errorf("resolve app dir: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return MainFileResult{}, fmt.Errorf("stat app dir: %w", err)
	}
	if !info.IsDir() {
		return MainFileResult{}, fmt.Errorf("app path is not a directory: %s", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return MainFileResult{}, fmt.Errorf("read app dir: %w", err)
	}

	candidates := []string{}
	have := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") { // hidden files / OS junk
			continue
		}
		if !strings.EqualFold(filepath.Ext(name), ".py") {
			continue
		}
		candidates = append(candidates, name)
		have[name] = true
	}
	sort.Strings(candidates)

	res := MainFileResult{Candidates: candidates}
	switch {
	case have["streamlit_app.py"]:
		res.MainFile = "streamlit_app.py"
	case have["app.py"]:
		res.MainFile = "app.py"
	}
	return res, nil
}
