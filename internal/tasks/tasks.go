package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"thaw/internal/snowflake"
	"time"
)

// FinalizabilityRow describes a task and whether it can serve as a finalizer.
type FinalizabilityRow struct {
	Name           string `json:"name"`
	DisabledReason string `json:"disabledReason"`
}

// StatusRow holds the current state and last-run information for a single task.
type StatusRow struct {
	Name         string `json:"name"`
	TaskState    string `json:"taskState"`
	Predecessors string `json:"predecessors"`
	LastRunState string `json:"lastRunState"`
	LastRunTime  string `json:"lastRunTime"`
	ErrorMsg     string `json:"errorMsg"`
	Finalize     string `json:"finalize"`
}

// StatusesResult wraps the per-task rows and an optional history-query error message.
type StatusesResult struct {
	Rows         []StatusRow `json:"rows"`
	HistoryError string      `json:"historyError"`
}

// Helper functions for parsing Snowflake results
func colIdx(cols []string, names ...string) int {
	for i, c := range cols {
		lc := strings.ToLower(c)
		for _, n := range names {
			if lc == n {
				return i
			}
		}
	}
	return -1
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case []byte:
		return string(t)
	case string:
		return t
	case []interface{}:
		parts := make([]string, 0, len(t))
		for _, el := range t {
			if el != nil {
				parts = append(parts, fmt.Sprintf("%v", el))
			}
		}
		return "[" + strings.Join(parts, ",") + "]"
	case time.Time:
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func q(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// bareIdent strips exactly one surrounding double-quote pair from a Snowflake
// identifier segment and unescapes any internal "" → ".
// Input examples: `"MY_TASK"` → `MY_TASK`, `"my""task"` → `my"task`.
func bareIdent(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return strings.ReplaceAll(s, `""`, `"`)
}

// CloneChildTask clones a task and replaces its predecessors.
func CloneChildTask(ctx context.Context, client *snowflake.Client, database, schema, oldName, newName string, newPredecessors []string) error {
	escStr := func(s string) string { return strings.ReplaceAll(s, `'`, `''`) }
	formatPred := func(p string) string {
		p = strings.TrimSpace(p)
		parts := strings.Split(p, ".")
		var quotedParts []string
		for _, part := range parts {
			if cleanPart := bareIdent(part); cleanPart != "" {
				quotedParts = append(quotedParts, q(cleanPart))
			}
		}
		if len(quotedParts) == 1 {
			return fmt.Sprintf("%s.%s.%s", q(database), q(schema), quotedParts[0])
		}
		return strings.Join(quotedParts, ".")
	}

	fqnOld := fmt.Sprintf("%s.%s.%s", q(database), q(schema), q(oldName))
	fqnNew := fmt.Sprintf("%s.%s.%s", q(database), q(schema), q(newName))

	showSQL := fmt.Sprintf("SHOW TASKS LIKE '%s' IN SCHEMA %s.%s", escStr(oldName), q(database), q(schema))
	res, err := client.Execute(ctx, showSQL)
	if err != nil {
		return fmt.Errorf("failed to fetch original task details: %w", err)
	}

	var oldPredecessors []string
	predsIdx := colIdx(res.Columns, "predecessors", "predecessor")
	nameIdx := colIdx(res.Columns, "name")

	if predsIdx >= 0 && nameIdx >= 0 {
		for _, row := range res.Rows {
			if nameIdx < len(row) && row[nameIdx] != nil && strings.EqualFold(fmt.Sprint(row[nameIdx]), oldName) {
				if predsIdx < len(row) && row[predsIdx] != nil {
					predsStr := strings.TrimSpace(fmt.Sprint(row[predsIdx]))
					if predsStr != "" && predsStr != "[]" && predsStr != "<nil>" && predsStr != "null" {
						predsStr = strings.TrimPrefix(predsStr, "[")
						predsStr = strings.TrimSuffix(predsStr, "]")
						for _, p := range strings.Split(predsStr, ",") {
							if formatted := formatPred(p); formatted != "" {
								oldPredecessors = append(oldPredecessors, formatted)
							}
						}
					}
				}
				break
			}
		}
	}

	cloneSQL := fmt.Sprintf("CREATE TASK %s CLONE %s", fqnNew, fqnOld)
	if _, err := client.Execute(ctx, cloneSQL); err != nil {
		return fmt.Errorf("failed to clone task %q: %w", oldName, err)
	}

	if len(oldPredecessors) > 0 {
		removeSQL := fmt.Sprintf("ALTER TASK %s REMOVE AFTER %s", fqnNew, strings.Join(oldPredecessors, ", "))
		if _, err := client.Execute(ctx, removeSQL); err != nil {
			_, _ = client.Execute(ctx, fmt.Sprintf("DROP TASK IF EXISTS %s", fqnNew))
			return fmt.Errorf("failed to remove original predecessors from cloned task: %w", err)
		}
	}

	if len(newPredecessors) > 0 {
		var preds []string
		for _, p := range newPredecessors {
			if formatted := formatPred(p); formatted != "" {
				preds = append(preds, formatted)
			}
		}
		alterSQL := fmt.Sprintf("ALTER TASK %s ADD AFTER %s", fqnNew, strings.Join(preds, ", "))
		if _, err := client.Execute(ctx, alterSQL); err != nil {
			_, _ = client.Execute(ctx, fmt.Sprintf("DROP TASK IF EXISTS %s", fqnNew))
			return fmt.Errorf("failed to attach new predecessors to cloned task: %w", err)
		}
	}
	return nil
}

// suspendIfRunning suspends the named task if its current state is STARTED.
// Snowflake requires a task to be suspended before its AFTER list can be modified.
func suspendIfRunning(ctx context.Context, client *snowflake.Client, database, schema, taskName string) error {
	escName := strings.ReplaceAll(taskName, `'`, `''`)
	res, err := client.Execute(ctx, fmt.Sprintf(
		"SHOW TASKS LIKE '%s' IN SCHEMA %s.%s", escName, q(database), q(schema)))
	if err != nil {
		return fmt.Errorf("failed to check state for task %q: %w", taskName, err)
	}
	nameIdx := colIdx(res.Columns, "name")
	stateIdx := colIdx(res.Columns, "state")
	if nameIdx < 0 || stateIdx < 0 {
		return nil
	}
	for _, row := range res.Rows {
		if nameIdx >= len(row) || stateIdx >= len(row) {
			continue
		}
		if !strings.EqualFold(toString(row[nameIdx]), taskName) {
			continue
		}
		if strings.ToUpper(toString(row[stateIdx])) == "STARTED" {
			fqn := fmt.Sprintf("%s.%s.%s", q(database), q(schema), q(taskName))
			if _, err := client.Execute(ctx, fmt.Sprintf("ALTER TASK IF EXISTS %s SUSPEND", fqn)); err != nil {
				return fmt.Errorf("failed to suspend task %q: %w", taskName, err)
			}
		}
		break
	}
	return nil
}

// RemoveParents removes the given parent tasks from the AFTER list of taskName via
// ALTER TASK … REMOVE AFTER.  The task is suspended first if it is currently running;
// the caller is responsible for resuming it afterwards if desired.
func RemoveParents(ctx context.Context, client *snowflake.Client, database, schema, taskName string, parents []string) error {
	if len(parents) == 0 {
		return nil
	}
	fqn := fmt.Sprintf("%s.%s.%s", q(database), q(schema), q(taskName))
	if err := suspendIfRunning(ctx, client, database, schema, taskName); err != nil {
		return err
	}
	preds := make([]string, 0, len(parents))
	for _, p := range parents {
		preds = append(preds, fmt.Sprintf("%s.%s.%s", q(database), q(schema), q(p)))
	}
	if _, err := client.Execute(ctx, fmt.Sprintf("ALTER TASK %s REMOVE AFTER %s", fqn, strings.Join(preds, ", "))); err != nil {
		return fmt.Errorf("failed to remove predecessors from task %q: %w", taskName, err)
	}
	return nil
}

// AddParents appends the given parent tasks to the AFTER list of taskName via
// ALTER TASK … ADD AFTER.  The task is suspended first if it is currently running;
// the caller is responsible for resuming it afterwards if desired.
func AddParents(ctx context.Context, client *snowflake.Client, database, schema, taskName string, parents []string) error {
	if len(parents) == 0 {
		return nil
	}
	fqn := fmt.Sprintf("%s.%s.%s", q(database), q(schema), q(taskName))
	if err := suspendIfRunning(ctx, client, database, schema, taskName); err != nil {
		return err
	}
	preds := make([]string, 0, len(parents))
	for _, p := range parents {
		preds = append(preds, fmt.Sprintf("%s.%s.%s", q(database), q(schema), q(p)))
	}
	if _, err := client.Execute(ctx, fmt.Sprintf("ALTER TASK %s ADD AFTER %s", fqn, strings.Join(preds, ", "))); err != nil {
		return fmt.Errorf("failed to add predecessors to task %q: %w", taskName, err)
	}
	return nil
}

// SuspendGraph suspends the root task first (to stop new run scheduling), then
// suspends every descendant found via BFS over SHOW TASKS.
func SuspendGraph(ctx context.Context, client *snowflake.Client, database, schema, taskName string) error {
	fqn := fmt.Sprintf("%s.%s.%s", q(database), q(schema), q(taskName))
	if _, err := client.Execute(ctx, fmt.Sprintf("ALTER TASK IF EXISTS %s SUSPEND", fqn)); err != nil {
		return err
	}

	res, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return err
	}

	nameIdx := colIdx(res.Columns, "name")
	predsIdx := colIdx(res.Columns, "predecessors", "predecessor")
	if nameIdx < 0 {
		return nil
	}

	children := make(map[string][]string)
	taskNames := make(map[string]string)
	for _, row := range res.Rows {
		name := toString(row[nameIdx])
		if name == "" {
			continue
		}
		taskNames[strings.ToUpper(name)] = name
		if predsIdx < 0 || predsIdx >= len(row) {
			continue
		}
		preds := toString(row[predsIdx])
		if preds == "" || preds == "[]" || preds == "<nil>" || preds == "null" {
			continue
		}
		preds = strings.TrimSuffix(strings.TrimPrefix(preds, "["), "]")
		for _, part := range strings.Split(preds, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			segs := strings.Split(part, ".")
			parent := strings.ToUpper(bareIdent(segs[len(segs)-1]))
			children[parent] = append(children[parent], name)
		}
	}

	rootUpper := strings.ToUpper(taskName)
	visited := map[string]bool{rootUpper: true}
	queue := []string{rootUpper}
	var descendants []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range children[cur] {
			cu := strings.ToUpper(child)
			if !visited[cu] {
				visited[cu] = true
				descendants = append(descendants, child)
				queue = append(queue, cu)
			}
		}
	}

	for _, child := range descendants {
		if orig, ok := taskNames[strings.ToUpper(child)]; ok {
			child = orig
		}
		if _, err := client.Execute(ctx, fmt.Sprintf(
			"ALTER TASK IF EXISTS %s.%s.%s SUSPEND", q(database), q(schema), q(child))); err != nil {
			return fmt.Errorf("suspending child task %q: %w", child, err)
		}
	}
	return nil
}

// ListFinalizableTasks returns every task in the schema along with an eligibility verdict.
func ListFinalizableTasks(ctx context.Context, client *snowflake.Client, database, schema string) ([]FinalizabilityRow, error) {
	res, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return nil, err
	}

	nameIdx := colIdx(res.Columns, "name")
	predsIdx := colIdx(res.Columns, "predecessors", "predecessor")
	schedIdx := colIdx(res.Columns, "schedule")
	finalizeIdx := colIdx(res.Columns, "finalize", "finalize_task")

	if nameIdx < 0 {
		return nil, nil
	}

	type taskMeta struct {
		name, preds, schedule, finalize string
	}
	metas := make([]taskMeta, 0, len(res.Rows))
	for _, row := range res.Rows {
		name := toString(row[nameIdx])
		if name == "" {
			continue
		}
		metas = append(metas, taskMeta{
			name:     name,
			preds:    toString(row[predsIdx]),
			schedule: toString(row[schedIdx]),
			finalize: toString(row[finalizeIdx]),
		})
	}

	hasChildren := make(map[string]bool, len(metas))
	for _, r := range metas {
		p := r.preds
		if p == "" || p == "[]" || p == "<nil>" {
			continue
		}
		p = strings.TrimSuffix(strings.TrimPrefix(p, "["), "]")
		for _, part := range strings.Split(p, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			segs := strings.Split(part, ".")
			bare := bareIdent(segs[len(segs)-1])
			if bare != "" {
				hasChildren[strings.ToUpper(bare)] = true
			}
		}
	}

	isBlank := func(s string) bool { return s == "" || s == "[]" || s == "<nil>" }
	var eligible, disabled []FinalizabilityRow
	for _, r := range metas {
		var reason string
		switch {
		case !isBlank(r.preds):
			reason = "Already a child task (has predecessors)"
		case r.schedule != "" && r.schedule != "null":
			reason = "Has its own schedule"
		case hasChildren[strings.ToUpper(r.name)]:
			reason = "Has child tasks"
		case r.finalize != "" && r.finalize != "null":
			reason = "Already a finalizer for another task"
		}
		row := FinalizabilityRow{Name: r.name, DisabledReason: reason}
		if reason == "" {
			eligible = append(eligible, row)
		} else {
			disabled = append(disabled, row)
		}
	}
	return append(eligible, disabled...), nil
}

// HasChildren reports whether any task in the schema lists taskName as a predecessor.
func HasChildren(ctx context.Context, client *snowflake.Client, database, schema, taskName string) (bool, error) {
	res, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return false, err
	}
	predsIdx := colIdx(res.Columns, "predecessors", "predecessor")
	if predsIdx < 0 {
		return false, nil
	}

	upper := strings.ToUpper(taskName)
	for _, row := range res.Rows {
		preds := toString(row[predsIdx])
		if preds == "" || preds == "[]" || preds == "<nil>" {
			continue
		}
		p := strings.TrimSuffix(strings.TrimPrefix(preds, "["), "]")
		for _, part := range strings.Split(p, ",") {
			segs := strings.Split(strings.TrimSpace(part), ".")
			if strings.ToUpper(bareIdent(segs[len(segs)-1])) == upper {
				return true, nil
			}
		}
	}
	return false, nil
}

// EnableDependents resumes the named task and all of its descendants in post-order.
func EnableDependents(ctx context.Context, client *snowflake.Client, database, schema, taskName string) error {
	res, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return err
	}

	nameIdx := colIdx(res.Columns, "name")
	predsIdx := colIdx(res.Columns, "predecessors", "predecessor")

	if nameIdx < 0 {
		_, err = client.Execute(ctx, fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s RESUME", q(database), q(schema), q(taskName)))
		return err
	}

	children := make(map[string][]string)
	taskNames := make(map[string]string)
	for _, row := range res.Rows {
		name := toString(row[nameIdx])
		if name == "" {
			continue
		}
		taskNames[strings.ToUpper(name)] = name
		if predsIdx < 0 || predsIdx >= len(row) {
			continue
		}
		preds := toString(row[predsIdx])
		if preds == "" || preds == "[]" || preds == "<nil>" || preds == "null" {
			continue
		}
		preds = strings.TrimSuffix(strings.TrimPrefix(preds, "["), "]")
		for _, part := range strings.Split(preds, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			segs := strings.Split(part, ".")
			parent := strings.ToUpper(bareIdent(segs[len(segs)-1]))
			children[parent] = append(children[parent], name)
		}
	}

	rootUpper := strings.ToUpper(taskName)
	visited := map[string]bool{rootUpper: true}
	bfsOrder := []string{rootUpper}
	queue := []string{rootUpper}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range children[cur] {
			cu := strings.ToUpper(child)
			if !visited[cu] {
				visited[cu] = true
				bfsOrder = append(bfsOrder, cu)
				queue = append(queue, cu)
			}
		}
	}

	for i := len(bfsOrder) - 1; i >= 0; i-- {
		upper := bfsOrder[i]
		name := upper
		if orig, ok := taskNames[upper]; ok {
			name = orig
		}
		if _, err := client.Execute(ctx, fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s RESUME", q(database), q(schema), q(name))); err != nil {
			return fmt.Errorf("resuming task %q: %w", name, err)
		}
	}
	return nil
}

// DropTree suspends and drops the named task and all of its descendants.
func DropTree(ctx context.Context, client *snowflake.Client, database, schema, taskName string) error {
	res, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return err
	}

	nameIdx := colIdx(res.Columns, "name")
	predsIdx := colIdx(res.Columns, "predecessors", "predecessor")
	if nameIdx < 0 {
		return fmt.Errorf("SHOW TASKS did not return a name column")
	}

	childrenOf := make(map[string][]string)
	taskNames := make(map[string]string)
	for _, row := range res.Rows {
		name := toString(row[nameIdx])
		if name == "" {
			continue
		}
		taskNames[strings.ToUpper(name)] = name
		if predsIdx < 0 || predsIdx >= len(row) {
			continue
		}
		preds := toString(row[predsIdx])
		if preds == "" || preds == "[]" || preds == "<nil>" {
			continue
		}
		preds = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(preds), "["), "]")
		for _, part := range strings.Split(preds, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			segs := strings.Split(part, ".")
			parent := strings.ToUpper(bareIdent(segs[len(segs)-1]))
			childrenOf[parent] = append(childrenOf[parent], name)
		}
	}

	var dropOrder []string
	visited := make(map[string]bool)
	var dfs func(name string)
	dfs = func(name string) {
		upper := strings.ToUpper(name)
		if visited[upper] {
			return
		}
		visited[upper] = true
		for _, child := range childrenOf[upper] {
			dfs(child)
		}
		if orig, ok := taskNames[upper]; ok {
			name = orig
		}
		dropOrder = append(dropOrder, name)
	}
	dfs(taskName)

	for _, name := range dropOrder {
		_, _ = client.Execute(ctx, fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s SUSPEND", q(database), q(schema), q(name)))
		if _, err := client.Execute(ctx, fmt.Sprintf("DROP TASK IF EXISTS %s.%s.%s", q(database), q(schema), q(name))); err != nil {
			return fmt.Errorf("dropping task %q: %w", name, err)
		}
	}
	return nil
}

// GetStatuses returns the current state and last-run result for every task in the given schema.
func GetStatuses(ctx context.Context, client *snowflake.Client, database, schema string) (StatusesResult, error) {
	showRes, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return StatusesResult{}, err
	}

	nameIdx := colIdx(showRes.Columns, "name")
	stateIdx := colIdx(showRes.Columns, "state")
	predsIdx := colIdx(showRes.Columns, "predecessors", "predecessor")
	finalizeIdx := colIdx(showRes.Columns, "finalize", "finalize_task")
	taskRelIdx := colIdx(showRes.Columns, "task_relations")

	// extractTaskRelField extracts the value of a named key from a task_relations
	// VARIANT column, returned by gosnowflake as either a map or a JSON string.
	extractTaskRelField := func(v interface{}, key string) string {
		if v == nil {
			return ""
		}
		lkey := strings.ToLower(key)
		if m, ok := v.(map[string]interface{}); ok {
			for k, val := range m {
				if strings.ToLower(k) == lkey && val != nil {
					if s := fmt.Sprintf("%v", val); s != "" && s != "<nil>" && s != "null" {
						return s
					}
				}
			}
		}
		raw := ""
		switch t := v.(type) {
		case string:
			raw = t
		case []byte:
			raw = string(t)
		default:
			raw = fmt.Sprintf("%v", v)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			for k, val := range m {
				if strings.ToLower(k) == lkey {
					var s string
					if err := json.Unmarshal(val, &s); err == nil && s != "" {
						return s
					}
				}
			}
		}
		return ""
	}

	extractFinalize := func(v interface{}) string {
		// Snowflake stores the root-task FQN that a finalizer task finalizes under
		// both "finalize" and (in some versions) "finalize_task".
		if s := extractTaskRelField(v, "finalize"); s != "" {
			return s
		}
		return extractTaskRelField(v, "finalize_task")
	}

	// finalizerByRootUpper maps UPPER(finalizer task bare-name) → root task name.
	// Populated from root tasks' task_relations.finalizerTask field. Used as a
	// fallback when the finalizer task's own finalize column is not populated
	// (occurs in some Snowflake versions).
	finalizerByRootUpper := map[string]string{}

	var rows []StatusRow
	nameMap := map[string]int{}
	for _, row := range showRes.Rows {
		name := toString(row[nameIdx])
		if name == "" {
			continue
		}
		finalize := ""
		if finalizeIdx >= 0 && finalizeIdx < len(row) {
			val := toString(row[finalizeIdx])
			// gosnowflake may return the string "null" for SQL NULL VARIANT columns;
			// treat it the same as an empty value so the task_relations fallback fires.
			if val != "null" {
				finalize = val
			}
		}
		if finalize == "" && taskRelIdx >= 0 && taskRelIdx < len(row) {
			finalize = extractFinalize(row[taskRelIdx])
		}
		// Always check task_relations for the finalizerTask field so we can identify
		// the finalizer from the ROOT task's perspective (fallback for older Snowflake).
		if taskRelIdx >= 0 && taskRelIdx < len(row) {
			if ft := extractTaskRelField(row[taskRelIdx], "finalizertask"); ft != "" {
				segs := strings.Split(ft, ".")
				ftName := bareIdent(segs[len(segs)-1])
				if ftName != "" {
					finalizerByRootUpper[strings.ToUpper(ftName)] = name
				}
			}
		}
		nameMap[strings.ToUpper(name)] = len(rows)
		rows = append(rows, StatusRow{
			Name:         name,
			TaskState:    strings.ToUpper(toString(row[stateIdx])),
			Predecessors: toString(row[predsIdx]),
			Finalize:     finalize,
		})
	}

	// Second pass: fill in Finalize for finalizer tasks that were identified
	// from the root task's task_relations.finalizerTask but whose own finalize
	// column was empty (handles older Snowflake versions / missing columns).
	for ftUpper, rootName := range finalizerByRootUpper {
		if idx, ok := nameMap[ftUpper]; ok && rows[idx].Finalize == "" {
			rows[idx].Finalize = rootName
		}
	}

	// Fetch run history
	histSQL := fmt.Sprintf(
		`SELECT * FROM TABLE(%s.INFORMATION_SCHEMA.TASK_HISTORY(`+
			`SCHEDULED_TIME_RANGE_START => DATEADD('day', -7, CURRENT_TIMESTAMP()),`+
			`RESULT_LIMIT => 10000))`+
			` ORDER BY SCHEDULED_TIME DESC NULLS FIRST, COMPLETED_TIME DESC NULLS FIRST`,
		q(database))

	histRes, histErr := client.Execute(ctx, histSQL)
	if histErr != nil {
		return StatusesResult{Rows: rows, HistoryError: histErr.Error()}, nil
	}

	tnIdx := colIdx(histRes.Columns, "task_name", "name")
	rsIdx := colIdx(histRes.Columns, "run_status", "state", "status")
	ctIdx := colIdx(histRes.Columns, "completed_time", "completion_time")
	qsIdx := colIdx(histRes.Columns, "query_start_time", "start_time")
	exIdx := colIdx(histRes.Columns, "exception_text", "error_message", "error_msg")
	scIdx := colIdx(histRes.Columns, "task_schema", "schema_name", "schema")

	toTime := func(v interface{}) time.Time {
		if t, ok := v.(time.Time); ok {
			return t
		}
		return time.Time{}
	}

	type bestEntry struct {
		sortKey                time.Time
		runState, runTime, err string
	}
	best := map[string]bestEntry{}

	for _, row := range histRes.Rows {
		if scIdx >= 0 && scIdx < len(row) && !strings.EqualFold(toString(row[scIdx]), schema) {
			continue
		}
		upper := strings.ToUpper(toString(row[tnIdx]))
		if _, ok := nameMap[upper]; !ok {
			continue
		}

		sortKey := toTime(row[ctIdx])
		if sortKey.IsZero() {
			sortKey = toTime(row[qsIdx])
		}

		prev, hasPrev := best[upper]
		if !hasPrev || (!sortKey.IsZero() && sortKey.After(prev.sortKey)) {
			best[upper] = bestEntry{
				sortKey:  sortKey,
				runState: toString(row[rsIdx]),
				runTime:  toString(row[ctIdx]),
				err:      toString(row[exIdx]),
			}
		}
	}

	for upper, e := range best {
		if idx, ok := nameMap[upper]; ok {
			rows[idx].LastRunState = e.runState
			rows[idx].LastRunTime = e.runTime
			rows[idx].ErrorMsg = e.err
		}
	}

	return StatusesResult{Rows: rows}, nil
}
