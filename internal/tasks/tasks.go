package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
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

// TaskHistoryRow holds a single row from INFORMATION_SCHEMA.TASK_HISTORY().
type TaskHistoryRow struct {
	Name          string `json:"name"`
	State         string `json:"state"`
	ReturnValue   string `json:"returnValue"`
	ScheduledTime string `json:"scheduledTime"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
	ErrorCode     string `json:"errorCode"`
	ErrorMessage  string `json:"errorMessage"`
	RunID         string `json:"runId"`
	RootTaskID    string `json:"rootTaskId"`
}

// GetTaskRunHistory returns the execution history for a task from INFORMATION_SCHEMA.TASK_HISTORY().
// For root tasks (isRoot=true), it fetches all child task executions grouped by SCHEDULED_TIME.
// For child/standalone tasks, it fetches only that task's history.
func GetTaskRunHistory(ctx context.Context, client *snowflake.Client, database, schema, taskName string, isRoot bool, days int) ([]TaskHistoryRow, error) {
	if days <= 0 {
		days = 1
	}

	quotedDB := snowflake.QuoteIdent(database)

	// For root tasks, look up the task ID via SHOW TASKS so we can use ROOT_TASK_ID
	// (TASK_HISTORY accepts ROOT_TASK_ID but not ROOT_TASK_NAME).
	// For non-root tasks, TASK_NAME accepts a plain unqualified name.
	var filterClause string
	if isRoot {
		showSQL := fmt.Sprintf("SHOW TASKS LIKE '%s' IN SCHEMA %s.%s",
			snowflake.EscapeLikePattern(taskName), quotedDB, snowflake.QuoteIdent(schema))
		showRes, err := client.Execute(ctx, showSQL)
		if err != nil {
			return nil, fmt.Errorf("failed to look up task ID: %w", err)
		}
		nameIdx := colIdx(showRes.Columns, "name")
		idIdx := colIdx(showRes.Columns, "id")
		if idIdx < 0 || nameIdx < 0 || len(showRes.Rows) == 0 {
			return nil, fmt.Errorf("task %q not found in %s.%s", taskName, database, schema)
		}
		// SHOW TASKS LIKE is case-insensitive and may return multiple rows;
		// find the exact match by name.
		var taskID string
		for _, r := range showRes.Rows {
			if strings.EqualFold(toString(r[nameIdx]), taskName) {
				taskID = toString(r[idIdx])
				break
			}
		}
		if taskID == "" {
			return nil, fmt.Errorf("task %q not found in %s.%s", taskName, database, schema)
		}
		filterClause = fmt.Sprintf("ROOT_TASK_ID => '%s'", snowflake.EscapeStringLit(taskID))
	} else {
		filterClause = fmt.Sprintf("TASK_NAME => '%s'", snowflake.EscapeStringLit(taskName))
	}

	sql := fmt.Sprintf(
		`SELECT * FROM TABLE(%s.INFORMATION_SCHEMA.TASK_HISTORY(`+
			`%s,`+
			`SCHEDULED_TIME_RANGE_START => DATEADD('day', -%d, CURRENT_TIMESTAMP()),`+
			`RESULT_LIMIT => 10000))`+
			` ORDER BY SCHEDULED_TIME DESC NULLS FIRST, QUERY_START_TIME ASC NULLS LAST`,
		quotedDB, filterClause, days)

	res, err := client.Execute(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("failed to query task history: %w", err)
	}

	nameIdx := colIdx(res.Columns, "name", "task_name")
	stateIdx := colIdx(res.Columns, "state", "run_status")
	retIdx := colIdx(res.Columns, "return_value")
	schedIdx := colIdx(res.Columns, "scheduled_time")
	startIdx := colIdx(res.Columns, "query_start_time", "start_time")
	endIdx := colIdx(res.Columns, "completed_time", "end_time")
	ecIdx := colIdx(res.Columns, "error_code")
	emIdx := colIdx(res.Columns, "error_message", "exception_text")
	ridIdx := colIdx(res.Columns, "run_id")
	rtidIdx := colIdx(res.Columns, "root_task_id")

	rows := make([]TaskHistoryRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		safe := func(idx int) string {
			if idx < 0 || idx >= len(row) {
				return ""
			}
			return toString(row[idx])
		}
		rows = append(rows, TaskHistoryRow{
			Name:          safe(nameIdx),
			State:         safe(stateIdx),
			ReturnValue:   safe(retIdx),
			ScheduledTime: safe(schedIdx),
			StartTime:     safe(startIdx),
			EndTime:       safe(endIdx),
			ErrorCode:     safe(ecIdx),
			ErrorMessage:  safe(emIdx),
			RunID:         safe(ridIdx),
			RootTaskID:    safe(rtidIdx),
		})
	}
	return rows, nil
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
// caseSensitive controls whether newName is double-quoted (true) or left unquoted when valid (false).
func CloneChildTask(ctx context.Context, client *snowflake.Client, database, schema, oldName, newName string, caseSensitive bool, newPredecessors []string) error {
	formatPred := func(p string) string {
		p = strings.TrimSpace(p)
		parts := strings.Split(p, ".")
		var quotedParts []string
		for _, part := range parts {
			if cleanPart := bareIdent(part); cleanPart != "" {
				quotedParts = append(quotedParts, snowflake.QuoteIdent(cleanPart))
			}
		}
		if len(quotedParts) == 1 {
			return fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), quotedParts[0])
		}
		return strings.Join(quotedParts, ".")
	}

	fqnOld := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(oldName))
	fqnNew := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteOrBare(newName, caseSensitive))

	showSQL := fmt.Sprintf("SHOW TASKS LIKE '%s' IN SCHEMA %s.%s", snowflake.EscapeLikePattern(oldName), snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema))
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
	escName := snowflake.EscapeLikePattern(taskName)
	res, err := client.Execute(ctx, fmt.Sprintf(
		"SHOW TASKS LIKE '%s' IN SCHEMA %s.%s", escName, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)))
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
			fqn := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(taskName))
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
	fqn := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(taskName))
	if err := suspendIfRunning(ctx, client, database, schema, taskName); err != nil {
		return err
	}
	preds := make([]string, 0, len(parents))
	for _, p := range parents {
		preds = append(preds, fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(p)))
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
	fqn := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(taskName))
	if err := suspendIfRunning(ctx, client, database, schema, taskName); err != nil {
		return err
	}
	preds := make([]string, 0, len(parents))
	for _, p := range parents {
		preds = append(preds, fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(p)))
	}
	if _, err := client.Execute(ctx, fmt.Sprintf("ALTER TASK %s ADD AFTER %s", fqn, strings.Join(preds, ", "))); err != nil {
		return fmt.Errorf("failed to add predecessors to task %q: %w", taskName, err)
	}
	return nil
}

// SuspendGraph suspends the root task first (to stop new run scheduling), then
// suspends every descendant found via BFS over SHOW TASKS.
func SuspendGraph(ctx context.Context, client *snowflake.Client, database, schema, taskName string) error {
	fqn := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(taskName))
	if _, err := client.Execute(ctx, fmt.Sprintf("ALTER TASK IF EXISTS %s SUSPEND", fqn)); err != nil {
		return err
	}

	res, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)))
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
			"ALTER TASK IF EXISTS %s.%s.%s SUSPEND", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(child))); err != nil {
			return fmt.Errorf("suspending child task %q: %w", child, err)
		}
	}
	return nil
}

// ListFinalizableTasks returns every task in the schema along with an eligibility verdict.
func ListFinalizableTasks(ctx context.Context, client *snowflake.Client, database, schema string) ([]FinalizabilityRow, error) {
	res, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)))
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
	res, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)))
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
	res, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)))
	if err != nil {
		return err
	}

	nameIdx := colIdx(res.Columns, "name")
	predsIdx := colIdx(res.Columns, "predecessors", "predecessor")

	if nameIdx < 0 {
		_, err = client.Execute(ctx, fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s RESUME", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(taskName)))
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
		if _, err := client.Execute(ctx, fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s RESUME", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))); err != nil {
			return fmt.Errorf("resuming task %q: %w", name, err)
		}
	}
	return nil
}

// DropTree suspends and drops the named task and all of its descendants.
func DropTree(ctx context.Context, client *snowflake.Client, database, schema, taskName string) error {
	res, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)))
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
		_, _ = client.Execute(ctx, fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s SUSPEND", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name)))
		if _, err := client.Execute(ctx, fmt.Sprintf("DROP TASK IF EXISTS %s.%s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))); err != nil {
			return fmt.Errorf("dropping task %q: %w", name, err)
		}
	}
	return nil
}

// TopologicalOrder holds the result of a Kahn's-algorithm topological sort
// over a task graph rooted at a given task.
type TopologicalOrder struct {
	// Tasks in dependency-safe order (root first, each task after all its predecessors).
	TopoOrder []string `json:"topoOrder"`
	// Finalizer task names (tasks whose Finalize field references the root).
	FinalizerNames []string `json:"finalizerNames"`
	// Suspend order: topological order + finalizers last.
	SuspendOrder []string `json:"suspendOrder"`
	// Resume order: reverse topological (leaves first, excl. root) + finalizers + root last.
	ResumeOrder []string `json:"resumeOrder"`
}

// parsePredecessorRefs parses the predecessors string from a StatusRow and
// returns the bare upper-cased task names. SHOW TASKS returns predecessors in
// two formats:
//   - Valid JSON array with dotted FQNs: ["DB.SCHEMA.TASK1","DB.SCHEMA.TASK2"]
//   - Snowflake-quoted array-like:       ["DB"."SCHEMA"."TASK1","DB"."SCHEMA"."TASK2"]
//
// The function tries JSON parsing first (matching the frontend parsePredecessors
// in taskHierarchy.ts), then falls back to string splitting for the non-JSON format.
func parsePredecessorRefs(preds string) []string {
	if preds == "" || preds == "[]" || preds == "<nil>" || preds == "null" {
		return nil
	}

	extractLast := func(ref string) string {
		segs := strings.Split(ref, ".")
		return strings.ToUpper(bareIdent(segs[len(segs)-1]))
	}

	// Try JSON array first (handles ["DB.SCH.TASK_A","DB.SCH.TASK_B"]).
	var jsonArr []string
	if json.Unmarshal([]byte(preds), &jsonArr) == nil {
		var names []string
		for _, ref := range jsonArr {
			if name := extractLast(ref); name != "" {
				names = append(names, name)
			}
		}
		return names
	}

	// Fallback: Snowflake-quoted format like ["DB"."SCH"."TASK_A","DB"."SCH"."TASK_B"].
	// NOTE: Naive comma split may mis-split if a quoted identifier itself contains
	// commas (e.g. "TASK,WITH,COMMAS"). This is extremely rare in practice; the
	// JSON path above handles it correctly when available.
	preds = strings.TrimSuffix(strings.TrimPrefix(preds, "["), "]")
	var names []string
	for _, part := range strings.Split(preds, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if name := extractLast(part); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// GetTopologicalOrder computes a dependency-safe ordering of tasks in the graph
// rooted at rootName using Kahn's algorithm. rootName is matched case-insensitively.
// This is a pure function — no Snowflake connection required.
func GetTopologicalOrder(rows []StatusRow, rootName string) TopologicalOrder {
	empty := TopologicalOrder{
		TopoOrder:      []string{},
		FinalizerNames: []string{},
		SuspendOrder:   []string{},
		ResumeOrder:    []string{},
	}
	rootUpper := strings.ToUpper(rootName)

	byName := make(map[string]string)       // UPPER → original name
	childrenOf := make(map[string][]string)  // UPPER(parent) → [UPPER(child)]
	predecessorsOf := make(map[string][]string) // UPPER(child) → [UPPER(parent)]
	var finalizerNames []string

	for _, t := range rows {
		upper := strings.ToUpper(t.Name)
		byName[upper] = t.Name

		// Check if this task is a finalizer for the root.
		if t.Finalize != "" {
			finSegs := strings.Split(t.Finalize, ".")
			finName := strings.ToUpper(bareIdent(finSegs[len(finSegs)-1]))
			if finName == rootUpper {
				finalizerNames = append(finalizerNames, t.Name)
			}
		}

		for _, predUpper := range parsePredecessorRefs(t.Predecessors) {
			childrenOf[predUpper] = append(childrenOf[predUpper], upper)
			predecessorsOf[upper] = append(predecessorsOf[upper], predUpper)
		}
	}

	if _, ok := byName[rootUpper]; !ok {
		return empty
	}

	// Step 1: BFS from root to discover which tasks belong to this graph.
	included := make(map[string]bool)
	queue := []string{rootUpper}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if included[cur] {
			continue
		}
		included[cur] = true
		for _, child := range childrenOf[cur] {
			if !included[child] {
				queue = append(queue, child)
			}
		}
	}

	// Step 2: Kahn's algorithm within the included set.
	inDegree := make(map[string]int)
	for upper := range included {
		inDegree[upper] = 0
	}
	for upper := range included {
		for _, pred := range predecessorsOf[upper] {
			if included[pred] {
				inDegree[upper]++
			}
		}
	}

	topoOrder := make([]string, 0, len(included))
	var kahnQueue []string
	for upper, deg := range inDegree {
		if deg == 0 {
			kahnQueue = append(kahnQueue, upper)
		}
	}
	sort.Strings(kahnQueue) // Deterministic seed order for stable output across runs.
	for len(kahnQueue) > 0 {
		cur := kahnQueue[0]
		kahnQueue = kahnQueue[1:]
		topoOrder = append(topoOrder, byName[cur])
		var ready []string
		for _, child := range childrenOf[cur] {
			if !included[child] {
				continue
			}
			inDegree[child]--
			if inDegree[child] == 0 {
				ready = append(ready, child)
			}
		}
		sort.Strings(ready) // Deterministic order when multiple children become ready simultaneously.
		kahnQueue = append(kahnQueue, ready...)
	}

	// Filter finalizers to only those NOT already included in the BFS traversal.
	var filteredFinalizers []string
	for _, fn := range finalizerNames {
		if !included[strings.ToUpper(fn)] {
			filteredFinalizers = append(filteredFinalizers, fn)
		}
	}
	// Ensure non-nil for JSON serialization.
	if filteredFinalizers == nil {
		filteredFinalizers = []string{}
	}

	// Suspend: topological order (root first) + finalizers last.
	suspendOrder := make([]string, 0, len(topoOrder)+len(filteredFinalizers))
	suspendOrder = append(suspendOrder, topoOrder...)
	suspendOrder = append(suspendOrder, filteredFinalizers...)

	// Resume: reverse topological (leaves first, excl. root) + finalizers + root last.
	reversed := make([]string, len(topoOrder))
	for i, name := range topoOrder {
		reversed[len(topoOrder)-1-i] = name
	}
	resumeOrder := make([]string, 0, len(reversed)+len(filteredFinalizers))
	if len(reversed) > 1 {
		resumeOrder = append(resumeOrder, reversed[:len(reversed)-1]...)
	}
	resumeOrder = append(resumeOrder, filteredFinalizers...)
	if len(reversed) > 0 {
		resumeOrder = append(resumeOrder, reversed[len(reversed)-1])
	}

	return TopologicalOrder{
		TopoOrder:      topoOrder,
		FinalizerNames: filteredFinalizers,
		SuspendOrder:   suspendOrder,
		ResumeOrder:    resumeOrder,
	}
}

// ExportGraphDDLResult holds the output of a graph DDL export.
type ExportGraphDDLResult struct {
	// The assembled DDL script.
	DDL string `json:"ddl"`
	// Number of tasks whose DDL was successfully fetched.
	TaskCount int `json:"taskCount"`
	// Names of tasks whose DDL could not be fetched.
	FailedTasks []string `json:"failedTasks"`
}

// BuildGraphDDL is a pure function that assembles a DDL export script from
// a topological ordering and a map of task name → DDL text. Tasks present in
// the ordering but absent from ddlByName are treated as failed (skipped in
// the output but counted in FailedTasks).
func BuildGraphDDL(order TopologicalOrder, ddlByName map[string]string, database, schema string, includeSuspendResume bool) ExportGraphDDLResult {
	fqn := func(name string) string {
		return snowflake.QuoteIdent(database) + "." + snowflake.QuoteIdent(schema) + "." + snowflake.QuoteIdent(name)
	}

	// Partition suspend-order tasks into succeeded / failed based on DDL presence.
	var succeeded []struct{ name, ddl string }
	var failedTasks []string
	for _, name := range order.SuspendOrder {
		if ddl, ok := ddlByName[name]; ok {
			succeeded = append(succeeded, struct{ name, ddl string }{name, ddl})
		} else {
			failedTasks = append(failedTasks, name)
		}
	}

	if len(succeeded) == 0 {
		if failedTasks == nil {
			failedTasks = []string{}
		}
		return ExportGraphDDLResult{FailedTasks: failedTasks}
	}

	var output string
	if includeSuspendResume {
		succeededNames := make(map[string]bool, len(succeeded))
		for _, s := range succeeded {
			succeededNames[strings.ToUpper(s.name)] = true
		}

		// Suspend lines (topological order: root first).
		var suspendLines []string
		for _, n := range order.SuspendOrder {
			if succeededNames[strings.ToUpper(n)] {
				suspendLines = append(suspendLines, "ALTER TASK IF EXISTS "+fqn(n)+" SUSPEND;")
			}
		}

		// DDL lines.
		var createLines []string
		for _, s := range succeeded {
			createLines = append(createLines, s.ddl)
		}

		// Resume lines (leaves first, root last).
		var resumeLines []string
		for _, n := range order.ResumeOrder {
			if succeededNames[strings.ToUpper(n)] {
				resumeLines = append(resumeLines, "ALTER TASK IF EXISTS "+fqn(n)+" RESUME;")
			}
		}

		output = "-- Suspend all tasks (root first)\n" +
			strings.Join(suspendLines, "\n") + "\n\n" +
			"-- Create / replace tasks (topological order)\n" +
			strings.Join(createLines, "\n\n") + "\n\n" +
			"-- Resume all tasks (leaves first, root last)\n" +
			strings.Join(resumeLines, "\n")
	} else {
		ddls := make([]string, len(succeeded))
		for i, s := range succeeded {
			ddls[i] = s.ddl
		}
		output = strings.Join(ddls, "\n\n")
	}

	if failedTasks == nil {
		failedTasks = []string{}
	}
	return ExportGraphDDLResult{
		DDL:         output,
		TaskCount:   len(succeeded),
		FailedTasks: failedTasks,
	}
}

// ExportGraphDDL fetches task statuses and DDL for every task in the graph
// rooted at rootName, then assembles a dependency-ordered DDL script.
func ExportGraphDDL(ctx context.Context, client *snowflake.Client, database, schema, rootName string, includeSuspendResume bool) (ExportGraphDDLResult, error) {
	result, err := GetStatuses(ctx, client, database, schema)
	if err != nil {
		return ExportGraphDDLResult{}, fmt.Errorf("fetching task statuses: %w", err)
	}

	order := GetTopologicalOrder(result.Rows, rootName)
	if len(order.SuspendOrder) == 0 {
		return ExportGraphDDLResult{
			DDL:         "",
			TaskCount:   0,
			FailedTasks: []string{},
		}, nil
	}

	// Fetch DDL for each task in parallel. Errors are non-fatal per task.
	// Semaphore limits concurrent Snowflake round-trips to avoid overwhelming
	// the session pool or hitting API rate limits on large graphs.
	const maxConcurrentDDL = 8
	sem := make(chan struct{}, maxConcurrentDDL)
	type ddlEntry struct {
		name string
		ddl  string
	}
	entries := make([]ddlEntry, len(order.SuspendOrder))
	var wg sync.WaitGroup
	for i, name := range order.SuspendOrder {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ddl, err := client.GetObjectDDL(ctx, database, schema, "task", name, "")
			if err != nil {
				slog.Warn("ExportGraphDDL: failed to fetch DDL for task",
					"task", name, "database", database, "schema", schema, "error", err)
				return
			}
			entries[i] = ddlEntry{name: name, ddl: ddl}
		}(i, name)
	}
	wg.Wait()

	ddlByName := make(map[string]string, len(order.SuspendOrder))
	for _, e := range entries {
		if e.name != "" {
			ddlByName[e.name] = e.ddl
		}
	}

	return BuildGraphDDL(order, ddlByName, database, schema, includeSuspendResume), nil
}

// GetStatuses returns the current state and last-run result for every task in the given schema.
func GetStatuses(ctx context.Context, client *snowflake.Client, database, schema string) (StatusesResult, error) {
	showRes, err := client.Execute(ctx, fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)))
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
		snowflake.QuoteIdent(database))

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
