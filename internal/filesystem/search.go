// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package filesystem

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SearchMatch describes a single line that matched the search query.
type SearchMatch struct {
	Path        string `json:"path"`
	LineNumber  int    `json:"lineNumber"`
	LineContent string `json:"lineContent"`
	MatchStart  int    `json:"matchStart"`
	MatchEnd    int    `json:"matchEnd"`
}

const maxSearchResults = 200

// SearchFiles walks dir recursively and returns lines matching query.
// If useRegex is true, query is compiled as a regular expression;
// otherwise a case-insensitive substring search is performed.
// Hidden directories (names starting with ".") are skipped.
// Returns at most maxSearchResults matches.
func SearchFiles(dir, query string, useRegex bool) ([]SearchMatch, error) {
	if query == "" {
		return nil, nil
	}

	var re *regexp.Regexp
	if useRegex {
		compiled, err := regexp.Compile(query)
		if err != nil {
			return nil, err
		}
		re = compiled
	}

	lowerQuery := strings.ToLower(query)
	var results []SearchMatch

	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if len(results) >= maxSearchResults {
			return filepath.SkipAll
		}

		f, openErr := os.Open(path)
		if openErr != nil {
			return nil // skip unreadable files
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			var start, end int
			var matched bool

			if useRegex {
				loc := re.FindStringIndex(line)
				if loc != nil {
					matched = true
					start, end = loc[0], loc[1]
				}
			} else {
				idx := strings.Index(strings.ToLower(line), lowerQuery)
				if idx >= 0 {
					matched = true
					start = idx
					end = idx + len(query)
				}
			}

			if matched {
				results = append(results, SearchMatch{
					Path:        path,
					LineNumber:  lineNum,
					LineContent: line,
					MatchStart:  start,
					MatchEnd:    end,
				})
				if len(results) >= maxSearchResults {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	return results, walkErr
}
