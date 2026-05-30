// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"fmt"
	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
	"thaw/internal/tasks"
)

// AlterTask runs an ALTER TASK IF EXISTS statement on the given task.
// clause is everything that follows the task name in the ALTER statement,
// for example "RESUME", "SUSPEND", "SET COMMENT = 'hello'", or
// "MODIFY AS SELECT 1". The caller is responsible for correct SQL quoting
// inside the clause; this method only double-quotes the task identifier.
func (a *App) AlterTask(database, schema, name, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s %s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}

// ListFinalizableTasks returns every task in the schema along with an eligibility verdict.
func (a *App) ListFinalizableTasks(database, schema string) ([]tasks.FinalizabilityRow, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return tasks.ListFinalizableTasks(a.ctx, a.client, database, schema)
}

// CloneChildTask clones a task and replaces its predecessors.
// caseSensitive controls whether newName is double-quoted (preserving exact case)
// or left unquoted when it is a valid bare identifier (Snowflake uppercases it).
func (a *App) CloneChildTask(database, schema, oldName, newName string, caseSensitive bool, newPredecessors []string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return tasks.CloneChildTask(a.ctx, a.client, database, schema, oldName, newName, caseSensitive, newPredecessors)
}

// GetTaskStatuses returns the current state and last-run result for every task in the given schema.
func (a *App) GetTaskStatuses(database, schema string) (tasks.StatusesResult, error) {
	if a.client == nil {
		return tasks.StatusesResult{}, apperrors.ErrNotConnected
	}
	return tasks.GetStatuses(a.ctx, a.client, database, schema)
}

// GetTopologicalOrder computes a dependency-safe topological ordering of tasks
// in a graph rooted at rootName. It fetches current task statuses from
// Snowflake and runs Kahn's algorithm on the result.
func (a *App) GetTopologicalOrder(database, schema, rootName string) (tasks.TopologicalOrder, error) {
	if a.client == nil {
		return tasks.TopologicalOrder{}, apperrors.ErrNotConnected
	}
	result, err := tasks.GetStatuses(a.ctx, a.client, database, schema)
	if err != nil {
		return tasks.TopologicalOrder{}, err
	}
	return tasks.GetTopologicalOrder(result.Rows, rootName), nil
}

// ExportGraphDDL fetches task statuses and DDL for every task in the graph
// rooted at rootName and returns a dependency-ordered DDL script.
func (a *App) ExportGraphDDL(database, schema, rootName string, includeSuspendResume bool) (tasks.ExportGraphDDLResult, error) {
	if a.client == nil {
		return tasks.ExportGraphDDLResult{}, apperrors.ErrNotConnected
	}
	return tasks.ExportGraphDDL(a.ctx, a.client, database, schema, rootName, includeSuspendResume)
}

// GetTaskRunHistory returns the execution history for a task from INFORMATION_SCHEMA.TASK_HISTORY().
func (a *App) GetTaskRunHistory(database, schema, taskName string, isRoot bool, days int) ([]tasks.TaskHistoryRow, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return tasks.GetTaskRunHistory(a.ctx, a.client, database, schema, taskName, isRoot, days)
}

// ListRootTasks returns task finalizability rows for the given schema.
// Deprecated: use ListFinalizableTasks directly.
func (a *App) ListRootTasks(database, schema string) ([]tasks.FinalizabilityRow, error) {
	return a.ListFinalizableTasks(database, schema)
}

// TaskHasChildren reports whether any task in the schema lists taskName as a
// predecessor (i.e. the task has at least one dependent / child task).
func (a *App) TaskHasChildren(database, schema, taskName string) (bool, error) {
	if a.client == nil {
		return false, apperrors.ErrNotConnected
	}
	return tasks.HasChildren(a.ctx, a.client, database, schema, taskName)
}

// EnableTaskDependents resumes the named task and all of its descendants.
// Tasks are resumed in leaf-first (post-order) so that children are active
// before their parent, which Snowflake requires when enabling a task graph.
func (a *App) EnableTaskDependents(database, schema, taskName string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return tasks.EnableDependents(a.ctx, a.client, database, schema, taskName)
}

// SuspendTaskList suspends each task in the provided list in order.
// The caller is responsible for the correct ordering: the root task should
// appear first so it stops scheduling new runs before its children are touched.
// This is used by the frontend which already has the full graph state and can
// compute the correct order without re-parsing SHOW TASKS predecessor columns.
func (a *App) SuspendTaskList(database, schema string, names []string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	for _, name := range names {
		if _, err := a.client.Execute(a.ctx,
			fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s SUSPEND", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))); err != nil {
			return fmt.Errorf("suspending task %q: %w", name, err)
		}
	}
	return nil
}

// ResumeTaskList resumes each task in the provided list in order.
// The caller is responsible for the correct ordering: leaf tasks should appear
// first and the root task last, since Snowflake requires all predecessor tasks
// to be STARTED before a successor task can be resumed.
// This is used by the frontend which already has the full graph state and can
// compute the correct order without re-parsing SHOW TASKS predecessor columns.
func (a *App) ResumeTaskList(database, schema string, names []string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	for _, name := range names {
		if _, err := a.client.Execute(a.ctx,
			fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s RESUME", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))); err != nil {
			return fmt.Errorf("resuming task %q: %w", name, err)
		}
	}
	return nil
}

// SuspendTaskGraph suspends the root task first (to stop it from scheduling new
// runs) and then suspends every descendant task in the graph.
func (a *App) SuspendTaskGraph(database, schema, taskName string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return tasks.SuspendGraph(a.ctx, a.client, database, schema, taskName)
}

// DropTaskTree suspends and drops the named task and all of its descendants.
// Tasks are processed in post-order (leaves first, root last).
func (a *App) DropTaskTree(database, schema, taskName string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return tasks.DropTree(a.ctx, a.client, database, schema, taskName)
}

// ExecuteTask manually triggers a single run of a Snowflake Task.
// Pass a non-empty config JSON string to use USING CONFIG, or set
// retryLast to true to re-execute the last failed run.
func (a *App) ExecuteTask(database, schema, name, config string, retryLast bool) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return a.client.ExecuteTask(a.ctx, database, schema, name, config, retryLast)
}
