// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"sync"
	"testing"
)

func TestEditorContextStore_SetAndGetActiveSQL(t *testing.T) {
	s := NewEditorContextStore()

	// No active tab initially.
	sql, ok := s.ActiveEditorSQL()
	if ok || sql != "" {
		t.Errorf("expected empty, got %q ok=%v", sql, ok)
	}

	s.SetActiveTab("tab1", "SELECT 1")
	sql, ok = s.ActiveEditorSQL()
	if !ok || sql != "SELECT 1" {
		t.Errorf("expected SELECT 1, got %q ok=%v", sql, ok)
	}

	// Switching active tab.
	s.SetActiveTab("tab2", "SELECT 2")
	sql, ok = s.ActiveEditorSQL()
	if !ok || sql != "SELECT 2" {
		t.Errorf("expected SELECT 2, got %q ok=%v", sql, ok)
	}
}

func TestEditorContextStore_SetTabSQL(t *testing.T) {
	s := NewEditorContextStore()
	s.SetActiveTab("tab1", "original")
	s.SetTabSQL("tab1", "updated")

	sql, ok := s.ActiveEditorSQL()
	if !ok || sql != "updated" {
		t.Errorf("expected updated, got %q ok=%v", sql, ok)
	}
}

func TestEditorContextStore_SetTabResult(t *testing.T) {
	s := NewEditorContextStore()
	s.SetActiveTab("tab1", "SELECT 1")

	summary := &ResultSummary{
		TabID:   "tab1",
		Columns: []string{"A", "B"},
		RowCount: 10,
		SampleRows: [][]any{{1, "x"}, {2, "y"}},
		QueryID: "qid-123",
	}
	s.SetTabResult("tab1", summary)

	got := s.QueryResultSummary("")
	if got == nil {
		t.Fatal("expected non-nil result summary")
	}
	if got.RowCount != 10 || got.QueryID != "qid-123" {
		t.Errorf("unexpected summary: %+v", got)
	}
	if len(got.Columns) != 2 || got.Columns[0] != "A" {
		t.Errorf("unexpected columns: %v", got.Columns)
	}
}

func TestEditorContextStore_QueryResultSummaryByTabID(t *testing.T) {
	s := NewEditorContextStore()
	s.SetActiveTab("tab1", "SELECT 1")
	s.SetTabResult("tab2", &ResultSummary{
		TabID:    "tab2",
		Columns:  []string{"X"},
		RowCount: 5,
	})

	// Explicit tab ID.
	got := s.QueryResultSummary("tab2")
	if got == nil || got.RowCount != 5 {
		t.Errorf("expected tab2 result, got %+v", got)
	}

	// Active tab (tab1) has no result.
	got = s.QueryResultSummary("")
	if got != nil {
		t.Errorf("expected nil for active tab, got %+v", got)
	}
}

func TestEditorContextStore_ClearTabResult(t *testing.T) {
	s := NewEditorContextStore()
	s.SetActiveTab("tab1", "SELECT 1")
	s.SetTabResult("tab1", &ResultSummary{RowCount: 10, QueryID: "qid-1"})

	s.ClearTabResult("tab1")

	// Result should be nil, but SQL should remain.
	if got := s.QueryResultSummary("tab1"); got != nil {
		t.Errorf("expected nil result after clear, got %+v", got)
	}
	sql, ok := s.ActiveEditorSQL()
	if !ok || sql != "SELECT 1" {
		t.Errorf("expected SQL to be preserved, got %q ok=%v", sql, ok)
	}

	// Clearing a non-existent tab should not panic.
	s.ClearTabResult("no-such-tab")
}

func TestEditorContextStore_RemoveTab(t *testing.T) {
	s := NewEditorContextStore()
	s.SetActiveTab("tab1", "SELECT 1")
	s.SetTabResult("tab1", &ResultSummary{RowCount: 1})

	s.RemoveTab("tab1")

	sql, ok := s.ActiveEditorSQL()
	if ok || sql != "" {
		t.Errorf("expected empty after remove, got %q ok=%v", sql, ok)
	}
	if got := s.QueryResultSummary("tab1"); got != nil {
		t.Errorf("expected nil result after remove, got %+v", got)
	}
}

func TestEditorContextStore_RemoveNonActiveTab(t *testing.T) {
	s := NewEditorContextStore()
	s.SetActiveTab("tab1", "SELECT 1")
	s.SetTabSQL("tab2", "SELECT 2")

	s.RemoveTab("tab2")

	// Active tab should be unaffected.
	sql, ok := s.ActiveEditorSQL()
	if !ok || sql != "SELECT 1" {
		t.Errorf("expected SELECT 1, got %q ok=%v", sql, ok)
	}
}

func TestEditorContextStore_ConcurrentAccess(t *testing.T) {
	s := NewEditorContextStore()
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tabID := "tab"
			sql := "SELECT " + string(rune('0'+n%10))
			s.SetActiveTab(tabID, sql)
			s.SetTabSQL(tabID, sql+"_updated")
			s.SetTabResult(tabID, &ResultSummary{RowCount: n})
			s.ClearTabResult(tabID)
			s.ActiveEditorSQL()
			s.QueryResultSummary("")
			s.QueryResultSummary(tabID)
		}(i)
	}
	wg.Wait()
}
