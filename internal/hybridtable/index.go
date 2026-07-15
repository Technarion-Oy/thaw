// SPDX-License-Identifier: GPL-3.0-or-later

package hybridtable

import "thaw/internal/snowflake"

// IndexColumn pairs a column name with its Snowflake data type. The index
// editors send these so the backend can decide which columns are eligible for
// each index role.
type IndexColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// IndexColumnOptions lists the column names eligible for each hybrid-table index
// role, used to populate the key-column and INCLUDE-column dropdowns.
type IndexColumnOptions struct {
	KeyColumns     []string `json:"keyColumns"`
	IncludeColumns []string `json:"includeColumns"`
}

// semiStructuredTypes and geospatialTypes are barred from both index roles;
// VECTOR and TIMESTAMP_TZ are additionally barred from key columns.
var (
	semiStructuredTypes = map[string]bool{"VARIANT": true, "OBJECT": true, "ARRAY": true}
	geospatialTypes     = map[string]bool{"GEOGRAPHY": true, "GEOMETRY": true}
)

// IsIndexableType reports whether a column of the given Snowflake data type may
// be a hybrid-table index KEY column. Snowflake bars semi-structured
// (VARIANT/OBJECT/ARRAY), geospatial (GEOGRAPHY/GEOMETRY), VECTOR, and
// TIMESTAMP_TZ columns from index keys (TIMESTAMP_NTZ is allowed).
func IsIndexableType(dataType string) bool {
	t := snowflake.BaseType(dataType)
	if semiStructuredTypes[t] || geospatialTypes[t] {
		return false
	}
	return t != "VECTOR" && t != "TIMESTAMP_TZ"
}

// IsIncludableType reports whether a column of the given Snowflake data type may
// be a hybrid-table index INCLUDE column. Snowflake bars only semi-structured
// and geospatial columns here.
func IsIncludableType(dataType string) bool {
	t := snowflake.BaseType(dataType)
	return !semiStructuredTypes[t] && !geospatialTypes[t]
}

// EligibleIndexColumns partitions cols into the names eligible as index key
// columns and as INCLUDE columns, per Snowflake's hybrid-table rules. The two
// lists are always non-nil so they marshal as [] rather than null.
func EligibleIndexColumns(cols []IndexColumn) IndexColumnOptions {
	opts := IndexColumnOptions{KeyColumns: []string{}, IncludeColumns: []string{}}
	for _, c := range cols {
		if IsIndexableType(c.Type) {
			opts.KeyColumns = append(opts.KeyColumns, c.Name)
		}
		if IsIncludableType(c.Type) {
			opts.IncludeColumns = append(opts.IncludeColumns, c.Name)
		}
	}
	return opts
}
