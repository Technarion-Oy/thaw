// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"thaw/internal/snowflake"
)

// countingSource is a metadataSource that records how many times each method is
// invoked, so tests can assert the cache collapses repeated fetches.
type countingSource struct {
	databases atomic.Int32
	schemas   atomic.Int32
	objects   atomic.Int32
	columns   atomic.Int32
	fks       atomic.Int32
	session   atomic.Int32

	// err, when non-nil, is returned by every list method.
	err error
	// block, when non-nil, is received on before each list method returns —
	// letting a test hold two concurrent fetches in flight to prove dedup.
	block chan struct{}
}

func (s *countingSource) maybeBlock() {
	if s.block != nil {
		<-s.block
	}
}

func (s *countingSource) GetSessionContext(context.Context) (snowflake.SessionContext, error) {
	s.session.Add(1)
	return snowflake.SessionContext{Database: "DB", Schema: "PUBLIC"}, s.err
}

func (s *countingSource) ListDatabases(context.Context) ([]string, error) {
	s.databases.Add(1)
	s.maybeBlock()
	if s.err != nil {
		return nil, s.err
	}
	return []string{"DB", "OTHER"}, nil
}

func (s *countingSource) ListSchemas(context.Context, string) ([]string, error) {
	s.schemas.Add(1)
	s.maybeBlock()
	if s.err != nil {
		return nil, s.err
	}
	return []string{"PUBLIC"}, nil
}

func (s *countingSource) ListObjects(context.Context, string, string) ([]snowflake.SnowflakeObject, error) {
	s.objects.Add(1)
	s.maybeBlock()
	if s.err != nil {
		return nil, s.err
	}
	return []snowflake.SnowflakeObject{{Name: "T", Kind: "TABLE", Schema: "PUBLIC"}}, nil
}

func (s *countingSource) GetTableColumnsWithTypes(context.Context, string, string, string) ([]snowflake.ColumnInfo, error) {
	s.columns.Add(1)
	s.maybeBlock()
	if s.err != nil {
		return nil, s.err
	}
	return []snowflake.ColumnInfo{{Name: "ID", DataType: "NUMBER(38,0)"}}, nil
}

func (s *countingSource) GetTableForeignKeys(context.Context, string, string, string) ([]snowflake.TableForeignKey, error) {
	s.fks.Add(1)
	s.maybeBlock()
	if s.err != nil {
		return nil, s.err
	}
	return nil, nil
}

// TestMetadataCacheHitsWithinTTL verifies that repeated identical fetches within
// the TTL issue exactly one underlying query per key (acceptance criterion 1).
func TestMetadataCacheHitsWithinTTL(t *testing.T) {
	src := &countingSource{}
	c := newMetadataCache(src, time.Minute)

	ctx := context.Background()
	for range 3 {
		if _, err := c.ListDatabases(ctx); err != nil {
			t.Fatalf("ListDatabases: %v", err)
		}
		if _, err := c.ListSchemas(ctx, "DB"); err != nil {
			t.Fatalf("ListSchemas: %v", err)
		}
		if _, err := c.ListObjects(ctx, "DB", "PUBLIC"); err != nil {
			t.Fatalf("ListObjects: %v", err)
		}
		if _, err := c.GetTableColumnsWithTypes(ctx, "DB", "PUBLIC", "T"); err != nil {
			t.Fatalf("GetTableColumnsWithTypes: %v", err)
		}
		if _, err := c.GetTableForeignKeys(ctx, "DB", "PUBLIC", "T"); err != nil {
			t.Fatalf("GetTableForeignKeys: %v", err)
		}
	}

	if got := src.databases.Load(); got != 1 {
		t.Errorf("ListDatabases called %d times, want 1", got)
	}
	if got := src.schemas.Load(); got != 1 {
		t.Errorf("ListSchemas called %d times, want 1", got)
	}
	if got := src.objects.Load(); got != 1 {
		t.Errorf("ListObjects called %d times, want 1", got)
	}
	if got := src.columns.Load(); got != 1 {
		t.Errorf("GetTableColumnsWithTypes called %d times, want 1", got)
	}
	if got := src.fks.Load(); got != 1 {
		t.Errorf("GetTableForeignKeys called %d times, want 1", got)
	}
}

// TestMetadataCacheDistinctKeys verifies that different qualified names are
// cached independently (no cross-key collisions).
func TestMetadataCacheDistinctKeys(t *testing.T) {
	src := &countingSource{}
	c := newMetadataCache(src, time.Minute)
	ctx := context.Background()

	_, _ = c.ListObjects(ctx, "DB", "PUBLIC")
	_, _ = c.ListObjects(ctx, "DB", "STAGING")
	_, _ = c.ListObjects(ctx, "OTHER", "PUBLIC")
	// Repeat the first — should hit cache.
	_, _ = c.ListObjects(ctx, "DB", "PUBLIC")

	if got := src.objects.Load(); got != 3 {
		t.Errorf("ListObjects called %d times, want 3 (one per distinct db.schema)", got)
	}
}

// TestMetadataCacheExpiry verifies that an entry is re-fetched after the TTL
// elapses, using an injected clock.
func TestMetadataCacheExpiry(t *testing.T) {
	src := &countingSource{}
	c := newMetadataCache(src, time.Second)

	var now atomic.Int64
	now.Store(time.Unix(1000, 0).UnixNano())
	c.now = func() time.Time { return time.Unix(0, now.Load()) }

	ctx := context.Background()
	if _, err := c.ListDatabases(ctx); err != nil {
		t.Fatal(err)
	}
	// Advance past the TTL.
	now.Store(time.Unix(1002, 0).UnixNano())
	if _, err := c.ListDatabases(ctx); err != nil {
		t.Fatal(err)
	}

	if got := src.databases.Load(); got != 2 {
		t.Errorf("ListDatabases called %d times, want 2 (cache expired)", got)
	}
}

// TestMetadataCacheErrorNotCached verifies that a failing fetch is not stored,
// so a later successful call still reaches the source.
func TestMetadataCacheErrorNotCached(t *testing.T) {
	src := &countingSource{err: errors.New("boom")}
	c := newMetadataCache(src, time.Minute)
	ctx := context.Background()

	if _, err := c.ListDatabases(ctx); err == nil {
		t.Fatal("expected error from failing source")
	}
	// Recover: clear the error and call again — must re-hit the source.
	src.err = nil
	if _, err := c.ListDatabases(ctx); err != nil {
		t.Fatalf("ListDatabases after recovery: %v", err)
	}
	if _, err := c.ListDatabases(ctx); err != nil {
		t.Fatalf("ListDatabases cached: %v", err)
	}

	if got := src.databases.Load(); got != 2 {
		t.Errorf("ListDatabases called %d times, want 2 (error not cached, then one success cached)", got)
	}
}

// TestMetadataCacheSingleflight verifies that concurrent identical misses
// collapse into a single underlying fetch.
func TestMetadataCacheSingleflight(t *testing.T) {
	src := &countingSource{block: make(chan struct{})}
	c := newMetadataCache(src, time.Minute)
	ctx := context.Background()

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_, _ = c.ListDatabases(ctx)
		}()
	}

	// Give the goroutines time to reach the fetch, then release them together.
	// The first fetch increments the counter before blocking; singleflight
	// should keep all others waiting on it rather than issuing their own.
	waitFor(t, func() bool { return src.databases.Load() >= 1 })
	close(src.block)
	wg.Wait()

	if got := src.databases.Load(); got != 1 {
		t.Errorf("ListDatabases called %d times under concurrency, want 1 (singleflight)", got)
	}
}

// SessionContext must never be cached — every call reaches the source.
func TestMetadataCacheSessionContextNotCached(t *testing.T) {
	src := &countingSource{}
	c := newMetadataCache(src, time.Minute)
	ctx := context.Background()

	for range 3 {
		if _, err := c.SessionContext(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := src.session.Load(); got != 3 {
		t.Errorf("GetSessionContext called %d times, want 3 (never cached)", got)
	}
}

// waitFor polls cond until true or a short deadline, avoiding a fixed sleep.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}
