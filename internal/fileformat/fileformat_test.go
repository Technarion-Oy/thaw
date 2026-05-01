// Copyright (c) 2026 Technarion Oy. All rights reserved.
package fileformat

import (
	"strings"
	"testing"
)

func TestBuildCreateTemporaryFileFormatSql(t *testing.T) {
	cfg := FileFormatConfig{
		Type:                       "CSV",
		RecordDelimiter:            "\\n",
		SkipHeader:                 1,
		NullIf:                     []string{"\\N"},
		ErrorOnColumnCountMismatch: true,
		EmptyFieldAsNull:           true,
		SkipByteOrderMark:          true,
	}
	name := "TEST_FORMAT"
	sql := BuildCreateTemporaryFileFormatSql(name, cfg)

	expected := "CREATE OR REPLACE TEMPORARY FILE_FORMAT \"TEST_FORMAT\"\n  TYPE = CSV\n  RECORD_DELIMITER = '\\n'\n  SKIP_HEADER = 1\n  NULL_IF = ('\\N');"
	if strings.TrimSpace(sql) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, sql)
	}
}

func TestBuildInlineFileFormat(t *testing.T) {
	cfg := FileFormatConfig{
		Type:                       "CSV",
		RecordDelimiter:            "\\n",
		SkipHeader:                 1,
		NullIf:                     []string{"\\N"},
		ErrorOnColumnCountMismatch: true,
		EmptyFieldAsNull:           true,
		SkipByteOrderMark:          true,
	}
	inline := BuildInlineFileFormat(cfg)

	expected := "TYPE = CSV, RECORD_DELIMITER = '\\n', SKIP_HEADER = 1, NULL_IF = ('\\N')"
	if strings.TrimSpace(inline) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, inline)
	}
}
