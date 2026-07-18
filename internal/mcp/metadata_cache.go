// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"thaw/internal/snowflake"
)

// metadataCacheTTL is how long a cached catalog entry stays fresh. It is short
// by design: the diagnostics tools (validate_sql, suggest_join_conditions) are
// meant for an AI client iteratively refining SQL in a tight loop, so a few
// seconds collapses the repeated identical metadata fetches of one refinement
// burst while staying fresh enough that a schema change surfaces almost
// immediately on the next call. See issue #355.
const metadataCacheTTL = 5 * time.Second

// metadataSource is the subset of *snowflake.Client the diagnostics tools read.
// Declaring it as an interface lets metadataCache be unit-tested with a fake
// that counts calls (metadata_cache_test.go); *snowflake.Client satisfies it.
type metadataSource interface {
	GetSessionContext(ctx context.Context) (snowflake.SessionContext, error)
	ListDatabases(ctx context.Context) ([]string, error)
	ListSchemas(ctx context.Context, database string) ([]string, error)
	ListObjects(ctx context.Context, database, schema string) ([]snowflake.SnowflakeObject, error)
	GetTableColumnsWithTypes(ctx context.Context, database, schema, name string) ([]snowflake.ColumnInfo, error)
	GetTableForeignKeys(ctx context.Context, database, schema, table string) ([]snowflake.TableForeignKey, error)
}

// metadataCache is a short-lived, per-session catalog cache in front of a
// metadataSource (the session's long-lived *snowflake.Client). It memoizes
// databases / schemas / objects / column types / foreign keys keyed by
// qualified name for metadataCacheTTL, so repeated validate_sql or
// suggest_join_conditions calls for the same schema do not re-issue identical
// metadata queries against the live account. Concurrent identical fetches are
// deduplicated via singleflight so two tool calls in flight at once issue one
// query, not two.
//
// GetSessionContext is intentionally NOT cached: it is cheap and can change
// mid-session (USE DATABASE / USE SCHEMA), so it always reads live.
//
// Only successful results are cached; an error is returned to the caller
// without being stored, so a transient failure never poisons the window.
type metadataCache struct {
	src   metadataSource
	ttl   time.Duration
	now   func() time.Time // injectable clock for tests; defaults to time.Now
	group singleflight.Group

	mu      sync.Mutex
	entries map[string]cacheEntry
}

// cacheEntry is one memoized fetch result with its expiry.
type cacheEntry struct {
	val     any
	expires time.Time
}

// newMetadataCache builds a cache over src with the given TTL.
func newMetadataCache(src metadataSource, ttl time.Duration) *metadataCache {
	return &metadataCache{
		src:     src,
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]cacheEntry),
	}
}

// get returns the cached value for key when still fresh, otherwise runs fetch
// (deduplicated across concurrent callers) and caches a successful result.
func (c *metadataCache) get(ctx context.Context, key string, fetch func(context.Context) (any, error)) (any, error) {
	if v, ok := c.lookup(key); ok {
		return v, nil
	}

	// singleflight collapses concurrent misses for the same key into one fetch.
	v, err, _ := c.group.Do(key, func() (any, error) {
		// Re-check under the lock: a racing caller may have populated the entry
		// between our miss above and acquiring the singleflight slot.
		if v, ok := c.lookup(key); ok {
			return v, nil
		}
		val, err := fetch(ctx)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.entries[key] = cacheEntry{val: val, expires: c.now().Add(c.ttl)}
		c.mu.Unlock()
		return val, nil
	})
	return v, err
}

// lookup returns the cached value for key if present and unexpired.
func (c *metadataCache) lookup(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || !c.now().Before(e.expires) {
		return nil, false
	}
	return e.val, true
}

// cacheKey joins parts with a NUL separator that cannot appear in a Snowflake
// identifier, so distinct (category, db, schema, name) tuples never collide.
// Parts are used verbatim (case-sensitive): the diagnostics path already folds
// unquoted identifiers to upper-case, while quoted identifiers are case-sensitive
// in Snowflake, so folding here would wrongly merge distinct quoted objects.
func cacheKey(parts ...string) string {
	return strings.Join(parts, "\x00")
}

// SessionContext reads the live session context (never cached).
func (c *metadataCache) SessionContext(ctx context.Context) (snowflake.SessionContext, error) {
	return c.src.GetSessionContext(ctx)
}

// ListDatabases returns every database name, cached per session.
func (c *metadataCache) ListDatabases(ctx context.Context) ([]string, error) {
	v, err := c.get(ctx, "databases", func(ctx context.Context) (any, error) {
		return c.src.ListDatabases(ctx)
	})
	if err != nil {
		return nil, err
	}
	return v.([]string), nil
}

// ListSchemas returns the schema names in database, cached per database.
func (c *metadataCache) ListSchemas(ctx context.Context, database string) ([]string, error) {
	v, err := c.get(ctx, cacheKey("schemas", database), func(ctx context.Context) (any, error) {
		return c.src.ListSchemas(ctx, database)
	})
	if err != nil {
		return nil, err
	}
	return v.([]string), nil
}

// ListObjects returns the objects in database.schema, cached per db.schema.
func (c *metadataCache) ListObjects(ctx context.Context, database, schema string) ([]snowflake.SnowflakeObject, error) {
	v, err := c.get(ctx, cacheKey("objects", database, schema), func(ctx context.Context) (any, error) {
		return c.src.ListObjects(ctx, database, schema)
	})
	if err != nil {
		return nil, err
	}
	return v.([]snowflake.SnowflakeObject), nil
}

// GetTableColumnsWithTypes returns the columns of database.schema.name, cached
// per qualified table.
func (c *metadataCache) GetTableColumnsWithTypes(ctx context.Context, database, schema, name string) ([]snowflake.ColumnInfo, error) {
	v, err := c.get(ctx, cacheKey("columns", database, schema, name), func(ctx context.Context) (any, error) {
		return c.src.GetTableColumnsWithTypes(ctx, database, schema, name)
	})
	if err != nil {
		return nil, err
	}
	return v.([]snowflake.ColumnInfo), nil
}

// GetTableForeignKeys returns the foreign keys of database.schema.table, cached
// per qualified table.
func (c *metadataCache) GetTableForeignKeys(ctx context.Context, database, schema, table string) ([]snowflake.TableForeignKey, error) {
	v, err := c.get(ctx, cacheKey("fks", database, schema, table), func(ctx context.Context) (any, error) {
		return c.src.GetTableForeignKeys(ctx, database, schema, table)
	})
	if err != nil {
		return nil, err
	}
	return v.([]snowflake.TableForeignKey), nil
}
