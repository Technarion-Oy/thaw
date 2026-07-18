// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// UsageDependencyDirection selects which side of the
// SNOWFLAKE.ACCOUNT_USAGE.OBJECT_DEPENDENCIES relation to resolve.
type UsageDependencyDirection string

const (
	// DependsOn returns the objects the target object references (downstream):
	// the target is the REFERENCING object, results are the REFERENCED ones.
	DependsOn UsageDependencyDirection = "depends_on"
	// ReferencedBy returns the objects that reference the target (upstream):
	// the target is the REFERENCED object, results are the REFERENCING ones.
	ReferencedBy UsageDependencyDirection = "referenced_by"
)

// ObjectDependencyRef is one row of the account-usage dependency graph — a
// single object on the far side of a dependency edge from the queried object.
// Domain is the object's Snowflake domain (TABLE, VIEW, MATERIALIZED VIEW,
// FUNCTION, PROCEDURE, …) as reported by OBJECT_DEPENDENCIES.
type ObjectDependencyRef struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Name     string `json:"name"`
	Domain   string `json:"domain"`
}

// buildUsageDependencyQuery constructs the OBJECT_DEPENDENCIES SELECT for the
// requested direction. filterPrefix is the side of the edge the queried object
// sits on; selectPrefix is the side we return.
func buildUsageDependencyQuery(database, schema, name string, direction UsageDependencyDirection) (string, error) {
	var filterPrefix, selectPrefix string
	switch direction {
	case DependsOn:
		filterPrefix, selectPrefix = "REFERENCING", "REFERENCED"
	case ReferencedBy:
		filterPrefix, selectPrefix = "REFERENCED", "REFERENCING"
	default:
		return "", fmt.Errorf("unknown dependency direction %q", direction)
	}

	return fmt.Sprintf(
		"SELECT %[1]s_DATABASE, %[1]s_SCHEMA, %[1]s_OBJECT_NAME, %[1]s_OBJECT_DOMAIN "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.OBJECT_DEPENDENCIES "+
			"WHERE %[2]s_DATABASE = '%[3]s' AND %[2]s_SCHEMA = '%[4]s' AND %[2]s_OBJECT_NAME = '%[5]s' "+
			"ORDER BY %[1]s_DATABASE, %[1]s_SCHEMA, %[1]s_OBJECT_NAME",
		selectPrefix, filterPrefix,
		EscapeStringLit(database), EscapeStringLit(schema), EscapeStringLit(name),
	), nil
}

// GetObjectUsageDependencies resolves object dependencies from
// SNOWFLAKE.ACCOUNT_USAGE.OBJECT_DEPENDENCIES in the requested direction.
//
// Unlike the DDL-parsing engine (GetObjectDependencies), this covers every
// object domain — including tables and non-SQL procedure/function bodies the
// parser cannot read — and can resolve the reverse (referenced-by) direction.
// The trade-off is that the view requires governance privileges (a grant on the
// SNOWFLAKE database or the ACCOUNTADMIN role) and has propagation latency, so
// freshly-created dependencies may not appear for some time.
//
// The result is a flat, de-duplicated, sorted list rather than a recursive tree:
// OBJECT_DEPENDENCIES records direct edges only, and expanding it recursively
// per node would multiply governance-view round-trips.
func (c *Client) GetObjectUsageDependencies(ctx context.Context, database, schema, name string, direction UsageDependencyDirection) ([]ObjectDependencyRef, error) {
	query, err := buildUsageDependencyQuery(database, schema, name, direction)
	if err != nil {
		return nil, err
	}

	rows, err := c.queryCtx(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	seen := map[string]bool{}
	var refs []ObjectDependencyRef
	for rows.Next() {
		if ctx.Err() != nil {
			break
		}
		var db, sc, nm, domain string
		if err := rows.Scan(&db, &sc, &nm, &domain); err != nil {
			continue
		}
		// A single referencing object can pull in the same referenced object via
		// several edges (multiple columns, self-joins); collapse those here.
		key := strings.ToUpper(db + "." + sc + "." + nm)
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, ObjectDependencyRef{
			Database: db,
			Schema:   sc,
			Name:     nm,
			Domain:   strings.ToUpper(domain),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Database != refs[j].Database {
			return refs[i].Database < refs[j].Database
		}
		if refs[i].Schema != refs[j].Schema {
			return refs[i].Schema < refs[j].Schema
		}
		return refs[i].Name < refs[j].Name
	})
	return refs, nil
}
