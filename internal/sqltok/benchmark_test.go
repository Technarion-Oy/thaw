// SPDX-License-Identifier: GPL-3.0-or-later

package sqltok

import (
	"strings"
	"testing"
)

func buildBenchSQL() string {
	var b strings.Builder
	for range 100 {
		b.WriteString("SELECT 'hello; world', \"col;name\" FROM t WHERE x = 1;\n")
	}
	// Add a dollar-quoted function.
	b.WriteString("CREATE FUNCTION f() AS $$\n")
	for range 20 {
		b.WriteString("  var x = 'value; with; semis';\n")
	}
	b.WriteString("$$;\n")
	// Add block and line comments.
	for range 10 {
		b.WriteString("-- line comment with ; semicolon\n")
		b.WriteString("SELECT /* block; comment */ 1;\n")
	}
	return b.String()
}

var benchSQL = buildBenchSQL()

func BenchmarkTokenize(b *testing.B) {
	b.SetBytes(int64(len(benchSQL)))
	b.ReportAllocs()
	for range b.N {
		Tokenize(benchSQL)
	}
}

func BenchmarkSplit(b *testing.B) {
	b.SetBytes(int64(len(benchSQL)))
	b.ReportAllocs()
	for range b.N {
		Split(benchSQL)
	}
}

func BenchmarkSplitRanges(b *testing.B) {
	b.SetBytes(int64(len(benchSQL)))
	b.ReportAllocs()
	for range b.N {
		SplitRanges(benchSQL)
	}
}

func BenchmarkStripComments(b *testing.B) {
	b.SetBytes(int64(len(benchSQL)))
	b.ReportAllocs()
	for range b.N {
		StripComments(benchSQL)
	}
}

func BenchmarkInertRegions(b *testing.B) {
	b.SetBytes(int64(len(benchSQL)))
	b.ReportAllocs()
	for range b.N {
		InertRegions(benchSQL)
	}
}

func BenchmarkFirstToken(b *testing.B) {
	sql := "  -- comment\n  /* block */  SELECT 1 FROM t"
	b.SetBytes(int64(len(sql)))
	b.ReportAllocs()
	for range b.N {
		FirstToken(sql)
	}
}

func BenchmarkTokenizeIter(b *testing.B) {
	b.SetBytes(int64(len(benchSQL)))
	b.ReportAllocs()
	for range b.N {
		next := TokenizeIter(benchSQL)
		for {
			_, ok := next()
			if !ok {
				break
			}
		}
	}
}
