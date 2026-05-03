package architecture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type SemanticMap struct {
	Domains []struct {
		Name          string   `json:"name"`
		BackendPaths  []string `json:"backend_paths"`
		FrontendPaths []string `json:"frontend_paths"`
	} `json:"domains"`
}

// TestSemanticMapAccuracy ensures that all paths mentioned in the LLM marker function actually exist.
func TestSemanticMapAccuracy(t *testing.T) {
	mapJSON := GetCodebaseSemanticMap()

	var codebaseMap SemanticMap
	if err := json.Unmarshal([]byte(mapJSON), &codebaseMap); err != nil {
		t.Fatalf("Failed to parse semantic map JSON: %v", err)
	}

	// Move up to the project root assuming test runs from internal/architecture
	projectRoot := filepath.Join("..", "..")

	for _, domain := range codebaseMap.Domains {
		// Verify Backend Paths
		for _, path := range domain.BackendPaths {
			fullPath := filepath.Join(projectRoot, path)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				t.Errorf("Domain '%s' references missing backend path: %s", domain.Name, path)
			}
		}

		// Verify Frontend Paths
		for _, path := range domain.FrontendPaths {
			fullPath := filepath.Join(projectRoot, path)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				t.Errorf("Domain '%s' references missing frontend path: %s", domain.Name, path)
			}
		}
	}
}
