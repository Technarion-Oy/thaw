// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package ai

import (
	"strconv"
	"strings"
)

// csvFormat holds auto-detected CSV file format parameters.
type csvFormat struct {
	FieldDelimiter          string // e.g. "," or "\t" or "|"
	SkipHeader              int    // 1 if a header row is detected, 0 otherwise
	FieldOptionallyEnclosed string // `"` or `'` or empty string
}

// inspectCSVContent reads the first maxLines non-empty lines from content and
// deduces the CSV format without performing any file I/O.
func inspectCSVContent(content string, maxLines int) *csvFormat {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimRight(line, "\r")
		if t != "" {
			lines = append(lines, t)
		}
		if len(lines) >= maxLines {
			break
		}
	}
	if len(lines) == 0 {
		return nil
	}

	delim := detectDelimiter(lines)
	quote := detectQuoteChar(lines, delim)
	skipHeader := 0
	if detectHeader(lines, delim) {
		skipHeader = 1
	}

	delimStr := string(delim)
	if delim == '\t' {
		delimStr = `\t`
	}
	quoteStr := ""
	if quote != 0 {
		quoteStr = string(quote)
	}

	return &csvFormat{
		FieldDelimiter:          delimStr,
		SkipHeader:              skipHeader,
		FieldOptionallyEnclosed: quoteStr,
	}
}

// detectDelimiter counts candidate separators per line. The first candidate
// that has a non-zero and consistent count across all sample lines is chosen.
func detectDelimiter(lines []string) rune {
	candidates := []rune{',', ';', '\t', '|'}

	if len(lines) == 1 {
		best, maxCount := ',', 0
		for _, r := range candidates {
			if c := strings.Count(lines[0], string(r)); c > maxCount {
				maxCount, best = c, r
			}
		}
		return best
	}

	for _, r := range candidates {
		base := strings.Count(lines[0], string(r))
		if base == 0 {
			continue
		}
		consistent := true
		for _, line := range lines[1:] {
			if strings.Count(line, string(r)) != base {
				consistent = false
				break
			}
		}
		if consistent {
			return r
		}
	}
	return ','
}

// detectQuoteChar checks whether fields within sample lines are consistently
// enclosed by double or single quotes.
func detectQuoteChar(lines []string, delim rune) rune {
	double, single := 0, 0
	for _, line := range lines {
		for _, field := range strings.Split(line, string(delim)) {
			f := strings.TrimSpace(field)
			if len(f) >= 2 && f[0] == '"' && f[len(f)-1] == '"' {
				double++
			} else if len(f) >= 2 && f[0] == '\'' && f[len(f)-1] == '\'' {
				single++
			}
		}
	}
	if double > 0 && double >= single {
		return '"'
	}
	if single > 0 {
		return '\''
	}
	return 0
}

// detectHeader returns true when row 0 looks like a header: all text fields
// while row 1 contains at least one numeric field.
func detectHeader(lines []string, delim rune) bool {
	if len(lines) < 2 {
		return false
	}
	row1 := strings.Split(lines[0], string(delim))
	row2 := strings.Split(lines[1], string(delim))
	if len(row1) != len(row2) {
		return false
	}
	row1AllText, row2HasNum := true, false
	for i := range row1 {
		v1 := strings.Trim(strings.TrimSpace(row1[i]), `"'`)
		v2 := strings.Trim(strings.TrimSpace(row2[i]), `"'`)
		if _, err := strconv.ParseFloat(v1, 64); err == nil {
			row1AllText = false
		}
		if v2 != "" {
			if _, err := strconv.ParseFloat(v2, 64); err == nil {
				row2HasNum = true
			}
		}
	}
	return row1AllText && row2HasNum
}
