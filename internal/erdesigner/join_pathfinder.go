// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package erdesigner

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ── Types (JSON-serializable for Wails IPC) ─────────────────────────────────

// TableRef identifies a table by schema and name.
type TableRef struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
}

// FKColRef identifies a column in a specific table.
type FKColRef struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
	Col    string `json:"col"`
}

// FKPair represents a single column-to-column FK mapping within a join.
type FKPair struct {
	From FKColRef `json:"from"`
	To   FKColRef `json:"to"`
}

// JoinPathEdge is an edge in a join path carrying FK column references.
type JoinPathEdge struct {
	From FKColRef `json:"from"`
	To   FKColRef `json:"to"`
}

// JoinPath is a sequence of tables and edges connecting them via FK
// relationships, as returned by FindJoinPaths.
type JoinPath struct {
	Tables []TableRef     `json:"tables"`
	Edges  []JoinPathEdge `json:"edges"`
}

// JoinEntry represents a single JOIN clause in the built query state.
type JoinEntry struct {
	Table          TableRef `json:"table"`
	JoinType       string   `json:"joinType"`
	OnCondition    string   `json:"onCondition"`
	FKPairs        []FKPair `json:"fkPairs"`
	IsIntermediate bool     `json:"isIntermediate"`
}

// JoinQueryState holds the complete state for generating a JOIN SQL query.
type JoinQueryState struct {
	Database        string              `json:"database"`
	BaseTable       TableRef            `json:"baseTable"`
	Joins           []JoinEntry         `json:"joins"`
	SelectedColumns map[string][]string `json:"selectedColumns"`
}

// ── Internal adjacency types ────────────────────────────────────────────────

// adjEdge is an edge in the adjacency list, carrying FK column info.
type adjEdge struct {
	neighbor                                        string
	fromSchema, fromTable, fromCol                  string
	toSchema, toTable, toCol                        string
}

// bfsResult holds a single BFS path and its traversed edges.
type bfsResult struct {
	path  []string
	edges []adjEdge
}

// ── Adjacency graph ─────────────────────────────────────────────────────────

// buildAdjacency builds a bidirectional adjacency list from FK edges.
func buildAdjacency(fks []snowflake.ERForeignKey) map[string][]adjEdge {
	adj := make(map[string][]adjEdge)
	for _, fk := range fks {
		fromKey := snowflake.TableKey(fk.FromSchema, fk.FromTable)
		toKey := snowflake.TableKey(fk.ToSchema, fk.ToTable)

		// Forward: from -> to
		adj[fromKey] = append(adj[fromKey], adjEdge{
			neighbor:   toKey,
			fromSchema: fk.FromSchema, fromTable: fk.FromTable, fromCol: fk.FromCol,
			toSchema: fk.ToSchema, toTable: fk.ToTable, toCol: fk.ToCol,
		})

		// Reverse: to -> from (same FK info, traversed backwards)
		adj[toKey] = append(adj[toKey], adjEdge{
			neighbor:   fromKey,
			fromSchema: fk.FromSchema, fromTable: fk.FromTable, fromCol: fk.FromCol,
			toSchema: fk.ToSchema, toTable: fk.ToTable, toCol: fk.ToCol,
		})
	}
	return adj
}

// ── BFS ─────────────────────────────────────────────────────────────────────

type parentEntry struct {
	parent string
	edge   adjEdge
}

// bfsAllShortest returns all shortest paths from start to end, capped at
// maxPaths to avoid overwhelming the disambiguation UI in dense schemas.
func bfsAllShortest(adj map[string][]adjEdge, start, end string, maxPaths int) []bfsResult {
	if start == end {
		return []bfsResult{{path: []string{start}}}
	}

	parents := make(map[string][]parentEntry)
	dist := make(map[string]int)
	dist[start] = 0
	parents[start] = nil

	queue := []string{start}
	head := 0
	found := false
	targetDist := int(^uint(0) >> 1) // max int

	for head < len(queue) {
		current := queue[head]
		head++
		currentDist := dist[current]

		if found && currentDist >= targetDist {
			break
		}

		for _, edge := range adj[current] {
			next := edge.neighbor
			nextDist := currentDist + 1

			if _, seen := dist[next]; !seen {
				dist[next] = nextDist
				parents[next] = []parentEntry{{parent: current, edge: edge}}
				queue = append(queue, next)

				if next == end {
					found = true
					targetDist = nextDist
				}
			} else if dist[next] == nextDist {
				parents[next] = append(parents[next], parentEntry{parent: current, edge: edge})
			}
		}
	}

	if !found {
		return nil
	}

	// Reconstruct shortest paths via backtracking.
	var results []bfsResult
	var backtrack func(node string, path []string, edges []adjEdge)
	backtrack = func(node string, path []string, edges []adjEdge) {
		if len(results) >= maxPaths {
			return
		}
		if node == start {
			// Reverse copies of path and edges.
			rp := make([]string, len(path))
			re := make([]adjEdge, len(edges))
			for i, j := 0, len(path)-1; i < len(path); i, j = i+1, j-1 {
				rp[i] = path[j]
			}
			for i, j := 0, len(edges)-1; i < len(edges); i, j = i+1, j-1 {
				re[i] = edges[j]
			}
			results = append(results, bfsResult{path: rp, edges: re})
			return
		}
		for _, pe := range parents[node] {
			if len(results) >= maxPaths {
				return
			}
			backtrack(pe.parent, append(path, pe.parent), append(edges, pe.edge))
		}
	}

	backtrack(end, []string{end}, nil)
	return results
}

// parseKey splits a table key "SCHEMA.TABLE" back into schema + name.
func parseKey(key string) TableRef {
	schema, name, _ := strings.Cut(key, ".")
	return TableRef{Schema: schema, Name: name}
}

// bfsResultToJoinPath converts a bfsResult to a JoinPath.
func bfsResultToJoinPath(r bfsResult) JoinPath {
	tables := make([]TableRef, len(r.path))
	for i, k := range r.path {
		tables[i] = parseKey(k)
	}
	edges := make([]JoinPathEdge, len(r.edges))
	for i, e := range r.edges {
		edges[i] = JoinPathEdge{
			From: FKColRef{Schema: e.fromSchema, Table: e.fromTable, Col: e.fromCol},
			To:   FKColRef{Schema: e.toSchema, Table: e.toTable, Col: e.toCol},
		}
	}
	return JoinPath{Tables: tables, Edges: edges}
}

// ── Public API ──────────────────────────────────────────────────────────────

// FindJoinPaths finds join paths connecting the selected tables via FK
// relationships.
//
// For 2 tables: BFS from A to B, returns all shortest paths (up to 10).
// For 3+ tables: Steiner tree approximation — iteratively connects each
// unconnected table to the nearest already-connected table via BFS.
//
// Returns an empty slice if tables cannot be connected or fewer than 2 are
// selected.
func FindJoinPaths(selectedTables []TableRef, fks []snowflake.ERForeignKey) []JoinPath {
	if len(selectedTables) < 2 {
		return []JoinPath{}
	}

	adj := buildAdjacency(fks)
	selectedKeys := make([]string, len(selectedTables))
	for i, t := range selectedTables {
		selectedKeys[i] = snowflake.TableKey(t.Schema, t.Name)
	}

	// Check all selected tables exist in the graph.
	for _, key := range selectedKeys {
		if _, ok := adj[key]; !ok {
			return []JoinPath{}
		}
	}

	if len(selectedKeys) == 2 {
		results := bfsAllShortest(adj, selectedKeys[0], selectedKeys[1], 10)
		paths := make([]JoinPath, len(results))
		for i, r := range results {
			paths[i] = bfsResultToJoinPath(r)
		}
		return paths
	}

	// 3+ tables: Steiner tree approximation (greedy).
	connected := map[string]bool{selectedKeys[0]: true}
	treeKeys := []string{selectedKeys[0]}
	treeKeySet := map[string]bool{selectedKeys[0]: true}
	var treeEdges []adjEdge
	treeEdgeKeys := map[string]bool{}
	remaining := map[string]bool{}
	for _, k := range selectedKeys[1:] {
		remaining[k] = true
	}

	for len(remaining) > 0 {
		var bestPath *bfsResult
		var bestTarget string

		for target := range remaining {
			for source := range connected {
				paths := bfsAllShortest(adj, source, target, 1)
				if len(paths) == 0 {
					continue
				}
				if bestPath == nil || len(paths[0].path) < len(bestPath.path) {
					p := paths[0]
					bestPath = &p
					bestTarget = target
				}
			}
		}

		if bestPath == nil {
			return []JoinPath{} // disconnected graph
		}

		for _, key := range bestPath.path {
			connected[key] = true
			if !treeKeySet[key] {
				treeKeySet[key] = true
				treeKeys = append(treeKeys, key)
			}
		}
		for _, e := range bestPath.edges {
			eKey := fmt.Sprintf("%s.%s.%s-%s.%s.%s",
				e.fromSchema, e.fromTable, e.fromCol,
				e.toSchema, e.toTable, e.toCol)
			if !treeEdgeKeys[eKey] {
				treeEdgeKeys[eKey] = true
				treeEdges = append(treeEdges, e)
			}
		}
		delete(remaining, bestTarget)
	}

	edges := make([]JoinPathEdge, len(treeEdges))
	for i, e := range treeEdges {
		edges[i] = JoinPathEdge{
			From: FKColRef{Schema: e.fromSchema, Table: e.fromTable, Col: e.fromCol},
			To:   FKColRef{Schema: e.toSchema, Table: e.toTable, Col: e.toCol},
		}
	}
	tables := make([]TableRef, len(treeKeys))
	for i, k := range treeKeys {
		tables[i] = parseKey(k)
	}

	return []JoinPath{{Tables: tables, Edges: edges}}
}

// BuildJoinState converts a JoinPath into a JoinQueryState ready for SQL
// generation. The first table in the path becomes the base table. Tables
// not in the user's original selection are marked as intermediate.
func BuildJoinState(path JoinPath, selectedTables []TableRef, database string) JoinQueryState {
	selectedSet := make(map[string]bool, len(selectedTables))
	for _, t := range selectedTables {
		selectedSet[snowflake.TableKey(t.Schema, t.Name)] = true
	}

	baseTable := path.Tables[0]
	visited := map[string]bool{snowflake.TableKey(baseTable.Schema, baseTable.Name): true}
	joins := make([]JoinEntry, 0, len(path.Tables)-1)

	for i := 1; i < len(path.Tables); i++ {
		table := path.Tables[i]
		tKey := snowflake.TableKey(table.Schema, table.Name)

		var conditions []string
		var fkPairs []FKPair
		for _, edge := range path.Edges {
			fromKey := snowflake.TableKey(edge.From.Schema, edge.From.Table)
			toKey := snowflake.TableKey(edge.To.Schema, edge.To.Table)

			if (fromKey == tKey && visited[toKey]) || (toKey == tKey && visited[fromKey]) {
				conditions = append(conditions, fmt.Sprintf(
					"%s.%s.%s = %s.%s.%s",
					edge.From.Schema, edge.From.Table, edge.From.Col,
					edge.To.Schema, edge.To.Table, edge.To.Col,
				))
				fkPairs = append(fkPairs, FKPair{
					From: FKColRef{Schema: edge.From.Schema, Table: edge.From.Table, Col: edge.From.Col},
					To:   FKColRef{Schema: edge.To.Schema, Table: edge.To.Table, Col: edge.To.Col},
				})
			}
		}

		visited[tKey] = true

		onCondition := strings.Join(conditions, " AND ")

		joins = append(joins, JoinEntry{
			Table:          TableRef{Schema: table.Schema, Name: table.Name},
			JoinType:       "INNER",
			OnCondition:    onCondition,
			FKPairs:        fkPairs,
			IsIntermediate: !selectedSet[tKey],
		})
	}

	return JoinQueryState{
		Database:        database,
		BaseTable:       TableRef{Schema: baseTable.Schema, Name: baseTable.Name},
		Joins:           joins,
		SelectedColumns: map[string][]string{},
	}
}
