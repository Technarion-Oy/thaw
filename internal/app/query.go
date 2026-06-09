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
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"thaw/internal/apperrors"
	"thaw/internal/logger"
	"thaw/internal/queryhistory"
	"thaw/internal/querylog"
	"thaw/internal/queryprofile"
	"thaw/internal/snowflake"
	"thaw/internal/telemetry"
	"time"

	sf "github.com/snowflakedb/gosnowflake/v2"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExecuteQuery runs a SQL statement and returns the result set.
// Used by context-menu shortcuts (e.g. "Select Top 1000"). For the main editor
// flow use StartQuery + WaitForQueryResult to surface the query ID early.
func (a *App) ExecuteQuery(sql string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	qidChan := make(chan string, 1)
	ctx := sf.WithQueryIDChan(a.ctx, qidChan)
	ctx = querylog.WithSource(ctx, querylog.SourceUser)

	start := time.Now()
	result, err := a.client.Execute(ctx, sql)
	dur := time.Since(start)

	var qid string
	if result != nil {
		select {
		case q := <-qidChan:
			qid = q
			result.QueryID = q
		default:
		}
	}

	// Record the completed query in the session log.
	if a.queryLog.IsEnabled() {
		status := querylog.StatusSuccess
		var errMsg string
		if err != nil {
			if errors.Is(err, context.Canceled) {
				status = querylog.StatusCanceled
			} else {
				status = querylog.StatusFail
				errMsg = err.Error()
			}
		}
		entry := querylog.Entry{
			Timestamp:  start,
			SQL:        sql,
			QueryID:    qid,
			Status:     status,
			DurationMs: dur.Milliseconds(),
			Error:      errMsg,
			Source:     querylog.SourceUser,
		}
		entry.ID = a.queryLog.Record(entry)
		wailsruntime.EventsEmit(a.ctx, "querylog:entry", entry)
	}

	return result, err
}

// GetQueryOperatorStats runs GET_QUERY_OPERATOR_STATS for the given Snowflake
// query ID and returns the typed execution-plan operator statistics.  The JSON
// object columns (operator_statistics, execution_time_breakdown,
// operator_attributes) are pre-parsed so the frontend receives them as JSON
// objects rather than raw strings.
func (a *App) GetQueryOperatorStats(queryID string) ([]queryprofile.OperatorStat, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return queryprofile.GetOperatorStats(a.ctx, a.client, queryID)
}

// RunExplain runs EXPLAIN USING JSON for the provided SQL and returns both
// the parsed plan tree and detected performance issues in a single response.
// Used by the editor context-menu "Explain SQL" action.
func (a *App) RunExplain(tabId string, sql string) (*queryprofile.ExplainResult, error) {
	client := a.client
	if tabId != "" {
		if ts, err := a.getOrInitTabSession(tabId); err == nil {
			client = ts.client
		}
	}
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}

	// Use a single pinned connection for the entire explain operation.
	// This ensures that the context sync and the EXPLAIN command share
	// the same session and see the same database/schema context.
	conn, err := client.GetConn(a.ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	// 1. Sync session context on the pinned connection.
	if _, err := client.GetSessionContextOnConn(a.ctx, conn); err != nil {
		logger.L.Warn("RunExplain: failed to sync session context", "err", err)
	}

	// 2. Run EXPLAIN on the same pinned connection.
	return queryprofile.RunExplainOnConn(a.ctx, client, conn, sql)
}

// StartQuery submits a SQL statement and returns the Snowflake query ID as
// soon as Snowflake assigns one.  For queries that need more than one HTTP
// round-trip (slow queries) this returns while execution is still in progress,
// giving the frontend a chance to display the query ID in the loading spinner.
// Call WaitForQueryResult afterwards to obtain the actual rows.
// An in-flight query can be stopped with CancelQuery.
func (a *App) StartQuery(tabId string, sql string) (string, error) {
	ts, err := a.getOrInitTabSession(tabId)
	if err != nil {
		return "", err
	}

	// Enforce PUT/GET feature flags before execution.
	flags := loadUserFeatureFlags()
	trimmed := strings.TrimSpace(strings.ToUpper(sql))
	if strings.HasPrefix(trimmed, "PUT ") || strings.HasPrefix(trimmed, "PUT\t") {
		if !flags.PutCommand {
			return "", fmt.Errorf("PUT commands are disabled. Enable them under View → Enabled Features…")
		}
	}
	if strings.HasPrefix(trimmed, "GET ") || strings.HasPrefix(trimmed, "GET\t") {
		if !flags.GetCommand {
			return "", fmt.Errorf("GET commands are disabled. Enable them under View → Enabled Features…")
		}
	}

	// Create a per-query cancellable context and replace any previous one.
	ctx, cancel := context.WithCancel(a.ctx)
	ctx = querylog.WithSource(ctx, querylog.SourceUser)

	// Record a RUNNING entry in the query log before execution begins.
	var logEntryID int
	logStart := time.Now()
	if a.queryLog.IsEnabled() {
		entry := querylog.Entry{
			Timestamp: logStart,
			SQL:       sql,
			Status:    querylog.StatusRunning,
			Source:    querylog.SourceUser,
			TabID:     tabId,
		}
		entry.ID = a.queryLog.Record(entry)
		logEntryID = entry.ID
		wailsruntime.EventsEmit(a.ctx, "querylog:entry", entry)
	}

	ts.queryMu.Lock()
	if ts.queryCancelFunc != nil {
		ts.queryCancelFunc() // cancel any still-running previous query
	}
	ts.queryCancelFunc = cancel
	ts.queryCancelCtxDone = ctx.Done()
	ts.queryDone = nil // clear stale channel from previous query
	ts.queryID = ""
	ts.queryLogEntryID = logEntryID
	ts.queryLogStart = logStart
	ts.queryMu.Unlock()

	qidChan := make(chan string, 1)
	ctx = sf.WithQueryIDChan(ctx, qidChan)
	ctx = sf.WithAsyncMode(ctx) // ask Snowflake to return query ID immediately, before results are ready
	done := make(chan struct{})

	// Execute the query in a background goroutine so this method can return
	// as soon as the query ID arrives (before results are ready).
	var wg sync.WaitGroup
	go func() {
		result, err := ts.client.Execute(ctx, sql, func(idx, total int, stmtQidChan <-chan string) {
			// Notify the frontend which statement is about to run.
			wailsruntime.EventsEmit(a.ctx, "query:statement-start",
				map[string]int{"index": idx, "total": total})
			// Watch for the per-statement query ID.  The channel is closed
			// by Execute once queryOnConn returns, so this goroutine always
			// terminates without needing ctx.Done().
			wg.Add(1)
			go func(i int, ch <-chan string) {
				defer wg.Done()
				// The gosnowflake driver closes ch after writing the qid, so
				// this select always terminates.  ctx.Done() is a fallback for
				// the rare case where the query is canceled before the driver
				// writes to the channel.
				select {
				case qid := <-ch:
					if qid != "" {
						// Keep ts.queryID up to date so WaitForQueryResult can
						// embed the last statement's query ID in the result.
						ts.queryMu.Lock()
						ts.queryID = qid
						ts.queryMu.Unlock()
						wailsruntime.EventsEmit(a.ctx, "query:statement-qid",
							map[string]interface{}{"index": i, "queryID": qid})
					}
				case <-ctx.Done():
				}
			}(idx, stmtQidChan)
		})
		// Wait for every per-statement qid goroutine to finish before
		// closing done, so WaitForQueryResult always reads a complete ts.queryID.
		wg.Wait()
		ts.queryMu.Lock()
		ts.queryResult = result
		ts.queryErr = err
		ts.queryMu.Unlock()
		close(done)
	}()

	// Block until the driver assigns a query ID (arrives with the first HTTP
	// response), the background goroutine finishes (fast query), or the query
	// is canceled.
	var queryID string
	select {
	case qid := <-qidChan:
		queryID = qid
	case <-done:
		// Fast query: results arrived before our select ran. Drain the channel.
		select {
		case qid := <-qidChan:
			queryID = qid
		default:
		}
	case <-ctx.Done():
		return "", ctx.Err()
	}

	ts.queryMu.Lock()
	// For single-statement queries, queryID comes from the outer qidChan
	// (async mode) and should be stored.  For multi-statement queries the
	// outer qidChan never fires (queryID = ""), so we leave ts.queryID as-is:
	// the per-statement qid goroutines (guarded by wg.Wait before close(done))
	// have already written the last statement's query ID into ts.queryID.
	if queryID != "" {
		ts.queryID = queryID
	}
	ts.queryDone = done
	ts.queryMu.Unlock()

	logger.L.Info("query started", "queryID", queryID)
	telemetry.Track(telemetry.EventQueryStarted, nil)
	return queryID, nil
}

// CancelQuery cancels the query currently in flight for tabId (started by
// StartQuery).  It is a no-op if no query is running for that tab.  In addition
// to canceling the local context, it issues SYSTEM$CANCEL_QUERY so that
// Snowflake stops the query server-side and stops consuming credits.
func (a *App) CancelQuery(tabId string) {
	val, ok := a.tabSessions.Load(tabId)
	if !ok {
		return
	}
	ts := val.(*tabSession)
	ts.queryMu.Lock()
	cancel := ts.queryCancelFunc
	queryID := ts.queryID
	ts.queryMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if queryID != "" && ts.client != nil {
		logger.L.Info("canceling query", "queryID", queryID)
		telemetry.Track(telemetry.EventQueryCancelled, nil)
		go func() {
			ctx, done := context.WithTimeout(a.ctx, 15*time.Second)
			defer done()
			if err := ts.client.CancelSnowflakeQuery(ctx, queryID); err != nil {
				logger.L.Warn("SYSTEM$CANCEL_QUERY failed", "queryID", queryID, "err", err)
			}
		}()
	}
}

// WaitForQueryResult blocks until the query submitted by StartQuery for tabId
// completes and returns the result set with the query ID embedded.
//
// If CancelQuery is called and the background goroutine does not finish within
// a 2-second grace period (e.g. the gosnowflake driver stalls while draining
// Arrow chunks after context cancellation), WaitForQueryResult returns
// context.Canceled immediately so the UI can reset without waiting for the
// driver to recover.  The background goroutine continues running and will clean
// up on its own once the driver eventually releases the connection.
func (a *App) WaitForQueryResult(tabId string) (*snowflake.QueryResult, error) {
	val, ok := a.tabSessions.Load(tabId)
	if !ok {
		return nil, fmt.Errorf("no query in progress")
	}
	ts := val.(*tabSession)

	ts.queryMu.Lock()
	done := ts.queryDone
	ctxDone := ts.queryCancelCtxDone
	ts.queryMu.Unlock()

	if done == nil {
		return nil, fmt.Errorf("no query in progress")
	}

	select {
	case <-done:
		// Normal path: background goroutine finished.
	case <-a.ctx.Done():
		// App is shutting down.
		return nil, a.ctx.Err()
	case <-ctxDone:
		// CancelQuery was called.  Give the driver a short window to respond
		// cleanly (it usually does — the Arrow error is logged before returning).
		select {
		case <-done:
			// Finished in time; fall through to the normal result-read below.
		case <-time.After(2 * time.Second):
			// Driver is stuck (Arrow chunk drain blocked on network I/O).
			// Unblock the UI now; the goroutine will clean up asynchronously.
			logger.L.Warn("query goroutine did not finish after cancellation; unblocking UI")
			ts.queryMu.Lock()
			if ts.queryCancelFunc != nil {
				ts.queryCancelFunc()
				ts.queryCancelFunc = nil
			}
			ts.queryDone = nil
			ts.queryID = ""
			ts.queryCancelCtxDone = nil
			stuckLogID := ts.queryLogEntryID
			stuckLogStart := ts.queryLogStart
			ts.queryLogEntryID = 0
			ts.queryMu.Unlock()
			// Update log entry for the stuck-canceled query.
			if stuckLogID > 0 {
				durationMs := time.Since(stuckLogStart).Milliseconds()
				a.queryLog.UpdateStatus(stuckLogID, querylog.StatusCanceled, durationMs, "", "")
				wailsruntime.EventsEmit(a.ctx, "querylog:update", map[string]interface{}{
					"id":         stuckLogID,
					"status":     querylog.StatusCanceled,
					"durationMs": durationMs,
				})
			}
			return nil, context.Canceled
		}
	}

	ts.queryMu.Lock()
	result := ts.queryResult
	err := ts.queryErr
	// Read queryID after done fires so multi-statement queries get the last
	// per-statement qid (updated by wg-tracked goroutines before close(done)).
	queryID := ts.queryID
	logEntryID := ts.queryLogEntryID
	logStart := ts.queryLogStart
	// Snapshot whether the query was explicitly canceled by the user BEFORE
	// calling queryCancelFunc: the cancel func also closes ctxDone, so
	// checking after cleanup would always report "canceled".
	var wasExplicitlyCancelled bool
	select {
	case <-ctxDone:
		wasExplicitlyCancelled = true
	default:
	}
	// Clean up so a subsequent call does not re-read stale state.
	if ts.queryCancelFunc != nil {
		ts.queryCancelFunc() // no-op if already canceled; ensures context resources are freed
		ts.queryCancelFunc = nil
	}
	ts.queryDone = nil
	ts.queryID = ""
	ts.queryCancelCtxDone = nil
	ts.queryLogEntryID = 0
	ts.queryMu.Unlock()

	if result != nil && queryID != "" {
		result.QueryID = queryID
	}
	// Backstop: if the query was explicitly canceled (user called CancelQuery)
	// but the driver still returned a driver-level error (e.g. "Object does not
	// exist" from an aborted S3 pre-signed URL), replace it with
	// context.Canceled so the frontend shows "query canceled", not a
	// misleading error message.
	if err != nil && wasExplicitlyCancelled {
		err = context.Canceled
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			logger.L.Info("query canceled", "queryID", queryID)
		} else {
			logger.L.Error("query failed", "queryID", queryID, "err", err)
			telemetry.Track(telemetry.EventQueryFailed, nil)
		}
	} else {
		logger.L.Info("query completed", "queryID", queryID)
		telemetry.Track(telemetry.EventQueryCompleted, nil)
	}

	// Update the query log entry with the final status.
	if logEntryID > 0 {
		durationMs := time.Since(logStart).Milliseconds()
		var status querylog.Status
		var errMsg string
		if err != nil {
			if errors.Is(err, context.Canceled) {
				status = querylog.StatusCanceled
			} else {
				status = querylog.StatusFail
				errMsg = err.Error()
			}
		} else {
			status = querylog.StatusSuccess
		}
		a.queryLog.UpdateStatus(logEntryID, status, durationMs, errMsg, queryID)
		wailsruntime.EventsEmit(a.ctx, "querylog:update", map[string]interface{}{
			"id":         logEntryID,
			"status":     status,
			"durationMs": durationMs,
			"error":      errMsg,
			"queryID":    queryID,
		})
	}

	return result, err
}

// GetQueryHistory queries SNOWFLAKE.INFORMATION_SCHEMA.QUERY_HISTORY* table
// functions and returns a slice of QueryHistoryRow ordered by start time desc.
//
//   - filterType:             "session" | "user" | "warehouse" | "all"
//   - sessionID:              non-empty → SESSION_ID => <id> (filterType="session")
//   - userName:               non-empty → USER_NAME => '<name>'
//   - warehouseName:          non-empty → WAREHOUSE_NAME => '<name>'
//   - endTimeStart/End:       RFC3339 strings or "" for no filter
//   - resultLimit:            max rows returned (1–10 000)
//   - includeClientGenerated: include client-generated statements
func (a *App) GetQueryHistory(
	filterType string,
	sessionID string,
	userName string,
	warehouseName string,
	endTimeStart string,
	endTimeEnd string,
	resultLimit int,
	includeClientGenerated bool,
) ([]queryhistory.QueryHistoryRow, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return queryhistory.GetQueryHistory(a.ctx, a.client, filterType, sessionID, userName, warehouseName, endTimeStart, endTimeEnd, resultLimit, includeClientGenerated)
}
