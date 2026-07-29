//go:build ignore

// gen_semantic_map scans the Thaw source tree for domain annotations and
// regenerates internal/architecture/semantic_map.go.
//
// Annotation formats:
//
//	Go package (outputs directory path):   // thaw:domain: <Domain Name>
//	Go individual file (outputs file):     // thaw:file-domain: <Domain Name>
//	TypeScript / TSX (outputs file path):  // @thaw-domain: <Domain Name>
//
// Usage:
//
//	go generate ./internal/architecture/
//	go run scripts/gen_semantic_map.go     (from any directory inside the project)
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// rePkgDomain matches "thaw:domain: <name>" in a Go source comment.
// When found, the package's directory path is added to the domain's backend_paths.
var rePkgDomain = regexp.MustCompile(`thaw:domain:\s*(.+)`)

// reFileDomain matches "thaw:file-domain: <name>" in a Go source comment.
// When found, the file's own path is added to the domain's backend_paths.
var reFileDomain = regexp.MustCompile(`thaw:file-domain:\s*(.+)`)

// reTSDomain matches "@thaw-domain: <name>" anywhere in a TypeScript/TSX file.
// When found, the file's own path is added to the domain's frontend_paths.
var reTSDomain = regexp.MustCompile(`@thaw-domain:\s*(.+)`)

// domainDescriptions maps domain names to human-readable descriptions.
// Update this map when adding or renaming a domain.
var domainDescriptions = map[string]string{
	"Core IPC & App Lifecycle":        "Wails entry points, window state persistence, and Zustand state management.",
	"SQL Editor & Diagnostics":        "Proprietary SQL tokenizer, syntax validation, and Monaco editor UI components.",
	"Object Browser & Administration": "Database metadata exploration, user management, and warehouse metering.",
	"Schema Migration":                "DDL extraction, schema diffing, and the deployment wizard.",
	"AI Tooling":                      "API clients for LLM providers; inline completion and model management.",
	"Snowpark & Developer Workflows":  "Python environment management, Jupyter kernels, dbt and Streamlit project scaffolding, and local Streamlit preview.",
	"Git Integration":                 "Git repository operations, Snowflake Git repository objects, and schema export versioning.",
	"MCP Server":                       "Model Context Protocol servers exposing the Snowflake connection to external AI clients over localhost.",
	"ER Designer":                      "Entity-relationship diagram viewer, interactive table designer, join pathfinding, and SQL generation.",
}

// domainOrder controls the output ordering. Domains not listed here are
// appended at the end in alphabetical order.
var domainOrder = []string{
	"Core IPC & App Lifecycle",
	"SQL Editor & Diagnostics",
	"Object Browser & Administration",
	"Schema Migration",
	"AI Tooling",
	"Snowpark & Developer Workflows",
	"Git Integration",
	"MCP Server",
}

// staticFrontendPaths are frontend paths that cannot be annotated (e.g. auto-generated
// code) but should still appear in the semantic map.
var staticFrontendPaths = map[string][]string{
	"Core IPC & App Lifecycle": {"frontend/wailsjs/"},
}

type domainData struct {
	backendPaths  []string
	frontendPaths []string
}

func main() {
	root, err := findProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	domains := make(map[string]*domainData)

	if err := scanInternalPackages(root, domains); err != nil {
		fmt.Fprintf(os.Stderr, "error scanning internal packages: %v\n", err)
		os.Exit(1)
	}
	if err := scanRootGoFiles(root, domains); err != nil {
		fmt.Fprintf(os.Stderr, "error scanning root Go files: %v\n", err)
		os.Exit(1)
	}
	if err := scanTSFiles(root, domains); err != nil {
		fmt.Fprintf(os.Stderr, "error scanning TypeScript files: %v\n", err)
		os.Exit(1)
	}

	// Merge static paths (prepend so they appear before annotated paths).
	for domain, paths := range staticFrontendPaths {
		d := ensureDomain(domains, domain)
		d.frontendPaths = append(paths, d.frontendPaths...)
	}

	// Sort paths within each domain for deterministic output.
	for _, d := range domains {
		sort.Strings(d.backendPaths)
		sort.Strings(d.frontendPaths)
	}

	ordered := orderedDomains(domains)

	// Build JSON payload.
	type jsonDomain struct {
		Name          string   `json:"name"`
		BackendPaths  []string `json:"backend_paths"`
		FrontendPaths []string `json:"frontend_paths"`
		Description   string   `json:"description"`
	}
	type jsonMap struct {
		Domains []jsonDomain `json:"domains"`
	}

	jmap := jsonMap{}
	for _, name := range ordered {
		d := domains[name]
		desc := domainDescriptions[name]
		if desc == "" {
			fmt.Fprintf(os.Stderr, "warning: no description for domain %q — add one to domainDescriptions in gen_semantic_map.go\n", name)
		}
		jmap.Domains = append(jmap.Domains, jsonDomain{
			Name:          name,
			BackendPaths:  d.backendPaths,
			FrontendPaths: d.frontendPaths,
			Description:   desc,
		})
	}

	jsonBytes, err := json.MarshalIndent(jmap, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshalling JSON: %v\n", err)
		os.Exit(1)
	}

	src := "// Code generated by scripts/gen_semantic_map.go; DO NOT EDIT.\n" +
		"// Regenerate with: go generate ./internal/architecture/\n\n" +
		"package architecture\n\n" +
		"// GetCodebaseSemanticMap is a marker function exclusively for LLM workflow context.\n" +
		"// It returns a JSON-formatted string defining the domain boundaries of the Thaw app.\n" +
		"// ALL LLM agents MUST read this map before proposing architectural changes or new features.\n" +
		"func GetCodebaseSemanticMap() string {\n" +
		"\treturn `" + string(jsonBytes) + "`\n" +
		"}\n"

	formatted, err := format.Source([]byte(src))
	if err != nil {
		// Write unformatted so the error is visible; should not happen.
		formatted = []byte(src)
		fmt.Fprintf(os.Stderr, "warning: gofmt failed: %v\n", err)
	}

	outPath := filepath.Join(root, "internal", "architecture", "semantic_map.go")
	if err := os.WriteFile(outPath, formatted, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d domains)\n", outPath, len(ordered))
}

// findProjectRoot walks parent directories until it finds go.mod.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found in any parent directory")
		}
		dir = parent
	}
}

func ensureDomain(m map[string]*domainData, name string) *domainData {
	if m[name] == nil {
		m[name] = &domainData{}
	}
	return m[name]
}

// scanInternalPackages finds thaw:domain annotations in internal/* packages.
// Each annotated package contributes its directory path (e.g. "internal/sqleditor/").
func scanInternalPackages(root string, domains map[string]*domainData) error {
	internalDir := filepath.Join(root, "internal")
	entries, err := os.ReadDir(internalDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkgDir := filepath.Join(internalDir, e.Name())
		files, err := os.ReadDir(pkgDir)
		if err != nil {
			continue
		}
		var pkgDomain string
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
				continue
			}
			domain := firstMatch(filepath.Join(pkgDir, f.Name()), rePkgDomain)
			if domain == "" {
				continue
			}
			if pkgDomain != "" && pkgDomain != domain {
				fmt.Fprintf(os.Stderr, "warning: conflicting domain annotations in internal/%s (%q vs %q); using first\n",
					e.Name(), pkgDomain, domain)
				continue
			}
			pkgDomain = domain
		}
		if pkgDomain != "" {
			d := ensureDomain(domains, pkgDomain)
			d.backendPaths = append(d.backendPaths, "internal/"+e.Name()+"/")
		}
	}
	return nil
}

// scanRootGoFiles finds thaw:file-domain annotations in root-level *.go files.
// Each annotated file contributes its own filename (e.g. "main.go").
func scanRootGoFiles(root string, domains map[string]*domainData) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		domain := firstMatch(filepath.Join(root, e.Name()), reFileDomain)
		if domain != "" {
			d := ensureDomain(domains, domain)
			d.backendPaths = append(d.backendPaths, e.Name())
		}
	}
	return nil
}

// scanTSFiles finds @thaw-domain annotations in TypeScript/TSX files under frontend/src/.
// Each annotated file contributes its own project-relative path.
func scanTSFiles(root string, domains map[string]*domainData) error {
	frontendSrc := filepath.Join(root, "frontend", "src")
	return filepath.WalkDir(frontendSrc, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if entry.IsDir() {
			if entry.Name() == "wailsjs" {
				return filepath.SkipDir // skip auto-generated bindings
			}
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".tsx") {
			return nil
		}
		domain := firstMatch(path, reTSDomain)
		if domain != "" {
			rel, _ := filepath.Rel(root, path)
			d := ensureDomain(domains, domain)
			d.frontendPaths = append(d.frontendPaths, filepath.ToSlash(rel))
		}
		return nil
	})
}

// firstMatch returns the first capture group of re found in any line of path, or "".
func firstMatch(path string, re *regexp.Regexp) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if m := re.FindStringSubmatch(scanner.Text()); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

// orderedDomains returns domain names in the preferred output order.
func orderedDomains(domains map[string]*domainData) []string {
	seen := make(map[string]bool)
	var result []string
	for _, name := range domainOrder {
		if _, ok := domains[name]; ok {
			result = append(result, name)
			seen[name] = true
		}
	}
	var extra []string
	for name := range domains {
		if !seen[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	return append(result, extra...)
}
