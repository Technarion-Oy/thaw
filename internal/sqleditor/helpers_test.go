package sqleditor

// ── Shared test helpers ──────────────────────────────────────────────────────

func getWarnings(markers []DiagMarker) []DiagMarker {
	var res []DiagMarker
	for _, m := range markers {
		if m.Severity == 4 {
			res = append(res, m)
		}
	}
	return res
}

func warnMsgs(markers []DiagMarker) []string {
	msgs := make([]string, len(markers))
	for i, m := range markers {
		msgs[i] = m.Message
	}
	return msgs
}

func getErrors(markers []DiagMarker) []DiagMarker {
	var res []DiagMarker
	for _, m := range markers {
		if m.Severity == 8 {
			res = append(res, m)
		}
	}
	return res
}

func getTestColCaches() []ColEntry {
	return []ColEntry{
		{
			DB: "DB", Schema: "SCH", Name: "EMPLOYEES",
			Cols: []ColInfo{
				{Name: "ID", DataType: "TEXT"},
				{Name: "FIRST_NAME", DataType: "TEXT"},
				{Name: "LAST_NAME", DataType: "TEXT"},
				{Name: "DEPT_ID", DataType: "TEXT"},
				{Name: "SALARY", DataType: "TEXT"},
			},
		},
		{
			DB: "DB", Schema: "SCH", Name: "DEPARTMENTS",
			Cols: []ColInfo{
				{Name: "DEPT_ID", DataType: "TEXT"},
				{Name: "DEPT_NAME", DataType: "TEXT"},
				{Name: "MANAGER_ID", DataType: "TEXT"},
			},
		},
	}
}

func getTestRefs() []ResolvedRef {
	return []ResolvedRef{
		{Alias: "e", DB: "DB", Schema: "SCH", Name: "EMPLOYEES"},
		{Alias: "EMPLOYEES", DB: "DB", Schema: "SCH", Name: "EMPLOYEES"},
		{Alias: "d", DB: "DB", Schema: "SCH", Name: "DEPARTMENTS"},
	}
}

func getLiveRefs() []ResolvedRef {
	return []ResolvedRef{
		{Alias: "l", DB: "DB", Schema: "SCH", Name: "LIVE_TABLE"},
	}
}
