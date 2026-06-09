// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package querylog

import (
	"context"
	"sync"
	"time"
)

// Status represents the execution status of a logged query.
type Status string

const (
	StatusRunning  Status = "RUNNING"
	StatusSuccess  Status = "SUCCESS"
	StatusFail     Status = "FAIL"
	StatusCanceled Status = "CANCELED"
)

// Source indicates who initiated the query.
type Source string

const (
	SourceUser     Source = "user"
	SourceInternal Source = "internal"
)

// Entry is a single query log record.
type Entry struct {
	ID         int       `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	SQL        string    `json:"sql"`
	QueryID    string    `json:"queryID"`
	Status     Status    `json:"status"`
	DurationMs int64     `json:"durationMs"`
	Error      string    `json:"error"`
	Source     Source    `json:"source"`
	TabID      string    `json:"tabID"`
}

const defaultMaxEntries = 5000

// Log is a thread-safe, session-scoped query log with FIFO eviction.
type Log struct {
	mu         sync.RWMutex
	entries    []Entry
	nextID     int
	enabled    bool
	filter     string // "all", "user", "internal"
	maxEntries int
}

// New creates a new Log with default settings.
func New() *Log {
	return &Log{
		filter:     "all",
		maxEntries: defaultMaxEntries,
	}
}

// Record appends an entry to the log and returns its assigned ID.
// If the log exceeds maxEntries, the oldest entry is evicted.
func (l *Log) Record(e Entry) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.nextID++
	e.ID = l.nextID
	l.entries = append(l.entries, e)
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}
	return e.ID
}

// UpdateStatus updates the status, duration, error message, and query ID of
// an existing entry identified by id. It is a no-op if the id is not found.
func (l *Log) UpdateStatus(id int, status Status, durationMs int64, errMsg string, queryID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.entries) - 1; i >= 0; i-- {
		if l.entries[i].ID == id {
			l.entries[i].Status = status
			l.entries[i].DurationMs = durationMs
			l.entries[i].Error = errMsg
			if queryID != "" {
				l.entries[i].QueryID = queryID
			}
			return
		}
	}
}

// Entries returns a copy of all log entries.
func (l *Log) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Clear removes all entries and resets the ID counter.
func (l *Log) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
	l.nextID = 0
}

// SetEnabled enables or disables logging.
func (l *Log) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// IsEnabled reports whether logging is currently active.
func (l *Log) IsEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.enabled
}

// SetFilter sets the source filter: "all", "user", or "internal".
func (l *Log) SetFilter(filter string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.filter = filter
}

// Filter returns the current source filter.
func (l *Log) Filter() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.filter
}

// ── Context helpers ─────────────────────────────────────────────────────────

type ctxKeySource struct{}
type ctxKeyTabID struct{}

// WithSource returns a context annotated with the query source.
func WithSource(ctx context.Context, s Source) context.Context {
	return context.WithValue(ctx, ctxKeySource{}, s)
}

// GetSource extracts the query source from ctx, defaulting to SourceInternal.
func GetSource(ctx context.Context) Source {
	if v, ok := ctx.Value(ctxKeySource{}).(Source); ok {
		return v
	}
	return SourceInternal
}

// WithTabID returns a context annotated with the tab ID.
func WithTabID(ctx context.Context, tabID string) context.Context {
	return context.WithValue(ctx, ctxKeyTabID{}, tabID)
}

// GetTabID extracts the tab ID from ctx, defaulting to "".
func GetTabID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyTabID{}).(string); ok {
		return v
	}
	return ""
}
