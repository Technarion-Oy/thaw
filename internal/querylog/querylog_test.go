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
	"testing"
	"time"
)

func TestRecordAndEntries(t *testing.T) {
	l := New()
	id := l.Record(Entry{
		Timestamp: time.Now(),
		SQL:       "SELECT 1",
		Status:    StatusSuccess,
		Source:    SourceUser,
	})
	if id != 1 {
		t.Fatalf("expected id=1, got %d", id)
	}
	entries := l.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].SQL != "SELECT 1" {
		t.Fatalf("unexpected SQL: %s", entries[0].SQL)
	}
	if entries[0].ID != 1 {
		t.Fatalf("expected entry ID=1, got %d", entries[0].ID)
	}
}

func TestUpdateStatus(t *testing.T) {
	l := New()
	id := l.Record(Entry{
		Timestamp: time.Now(),
		SQL:       "SELECT 1",
		Status:    StatusRunning,
		Source:    SourceUser,
	})
	l.UpdateStatus(id, StatusSuccess, 150, "", "01abc-def-ghi")
	entries := l.Entries()
	if entries[0].Status != StatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", entries[0].Status)
	}
	if entries[0].DurationMs != 150 {
		t.Fatalf("expected 150ms, got %d", entries[0].DurationMs)
	}
	if entries[0].QueryID != "01abc-def-ghi" {
		t.Fatalf("expected queryID 01abc-def-ghi, got %s", entries[0].QueryID)
	}
}

func TestUpdateStatusNotFound(t *testing.T) {
	l := New()
	// Should not panic when the ID does not exist.
	l.UpdateStatus(999, StatusFail, 0, "err", "")
}

func TestClear(t *testing.T) {
	l := New()
	l.Record(Entry{SQL: "SELECT 1"})
	l.Record(Entry{SQL: "SELECT 2"})
	l.Clear()
	if len(l.Entries()) != 0 {
		t.Fatal("expected empty after Clear")
	}
	// IDs should reset after clear.
	id := l.Record(Entry{SQL: "SELECT 3"})
	if id != 1 {
		t.Fatalf("expected id=1 after Clear, got %d", id)
	}
}

func TestFIFOEviction(t *testing.T) {
	l := New()
	l.maxEntries = 3
	l.Record(Entry{SQL: "a"})
	l.Record(Entry{SQL: "b"})
	l.Record(Entry{SQL: "c"})
	l.Record(Entry{SQL: "d"})
	entries := l.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].SQL != "b" {
		t.Fatalf("expected first entry SQL='b', got %s", entries[0].SQL)
	}
	if entries[2].SQL != "d" {
		t.Fatalf("expected last entry SQL='d', got %s", entries[2].SQL)
	}
}

func TestEnableDisable(t *testing.T) {
	l := New()
	if l.IsEnabled() {
		t.Fatal("expected disabled by default")
	}
	l.SetEnabled(true)
	if !l.IsEnabled() {
		t.Fatal("expected enabled after SetEnabled(true)")
	}
	l.SetEnabled(false)
	if l.IsEnabled() {
		t.Fatal("expected disabled after SetEnabled(false)")
	}
}

func TestFilter(t *testing.T) {
	l := New()
	if l.Filter() != "all" {
		t.Fatalf("expected default filter 'all', got %s", l.Filter())
	}
	l.SetFilter("user")
	if l.Filter() != "user" {
		t.Fatalf("expected filter 'user', got %s", l.Filter())
	}
	l.SetFilter("internal")
	if l.Filter() != "internal" {
		t.Fatalf("expected filter 'internal', got %s", l.Filter())
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Default source should be internal.
	if src := GetSource(ctx); src != SourceInternal {
		t.Fatalf("expected default source=internal, got %s", src)
	}

	ctx = WithSource(ctx, SourceUser)
	if src := GetSource(ctx); src != SourceUser {
		t.Fatalf("expected source=user, got %s", src)
	}

	// Default tab ID should be empty.
	if tid := GetTabID(ctx); tid != "" {
		t.Fatalf("expected empty tab ID, got %s", tid)
	}

	ctx = WithTabID(ctx, "tab-1")
	if tid := GetTabID(ctx); tid != "tab-1" {
		t.Fatalf("expected tab-1, got %s", tid)
	}
}

func TestConcurrentAccess(t *testing.T) {
	l := New()
	l.SetEnabled(true)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Record(Entry{SQL: "SELECT 1", Status: StatusSuccess, Source: SourceUser})
		}()
	}
	wg.Wait()
	entries := l.Entries()
	if len(entries) != 100 {
		t.Fatalf("expected 100 entries, got %d", len(entries))
	}
}

func TestEntriesReturnsCopy(t *testing.T) {
	l := New()
	l.Record(Entry{SQL: "SELECT 1"})
	entries := l.Entries()
	entries[0].SQL = "MODIFIED"
	original := l.Entries()
	if original[0].SQL != "SELECT 1" {
		t.Fatal("Entries() should return a copy, but original was modified")
	}
}
