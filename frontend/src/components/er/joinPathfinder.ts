// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import type { JoinPath, JoinQueryState, JoinEntry } from "./erTypes";

/** FK edge in the adjacency graph. */
interface FKEdge {
  fromSchema: string;
  fromTable: string;
  fromCol: string;
  toSchema: string;
  toTable: string;
  toCol: string;
}

/** Canonical key for a table node: "SCHEMA.TABLE" (uppercase). */
function tableKey(schema: string, name: string): string {
  return `${schema.toUpperCase()}.${name.trim().toUpperCase()}`;
}

/** Parse a table key back into schema + name. */
function parseKey(key: string): { schema: string; name: string } {
  const dot = key.indexOf(".");
  return { schema: key.slice(0, dot), name: key.slice(dot + 1) };
}

/** An edge in the adjacency list, carrying FK column info and direction. */
interface AdjEdge {
  neighbor: string; // tableKey of the other table
  fromSchema: string;
  fromTable: string;
  fromCol: string;
  toSchema: string;
  toTable: string;
  toCol: string;
}

/**
 * Build a bidirectional adjacency list from FK edges.
 * Both directions are stored so BFS can traverse parent->child and child->parent.
 */
function buildAdjacency(fks: FKEdge[]): Map<string, AdjEdge[]> {
  const adj = new Map<string, AdjEdge[]>();

  const ensure = (key: string) => {
    if (!adj.has(key)) adj.set(key, []);
  };

  for (const fk of fks) {
    const fromKey = tableKey(fk.fromSchema, fk.fromTable);
    const toKey = tableKey(fk.toSchema, fk.toTable);
    ensure(fromKey);
    ensure(toKey);

    // Forward: from -> to
    adj.get(fromKey)!.push({
      neighbor: toKey,
      fromSchema: fk.fromSchema,
      fromTable: fk.fromTable,
      fromCol: fk.fromCol,
      toSchema: fk.toSchema,
      toTable: fk.toTable,
      toCol: fk.toCol,
    });

    // Reverse: to -> from (same FK info, just traversed backwards)
    adj.get(toKey)!.push({
      neighbor: fromKey,
      fromSchema: fk.fromSchema,
      fromTable: fk.fromTable,
      fromCol: fk.fromCol,
      toSchema: fk.toSchema,
      toTable: fk.toTable,
      toCol: fk.toCol,
    });
  }

  return adj;
}

interface BFSResult {
  path: string[];       // table keys from start to end
  edges: AdjEdge[];     // one edge per hop
}

/**
 * BFS from `start` to `end`, returning ALL shortest paths of equal length.
 * Each path is a sequence of table keys + the edges traversed.
 */
function bfsAllShortest(
  adj: Map<string, AdjEdge[]>,
  start: string,
  end: string,
): BFSResult[] {
  if (start === end) return [{ path: [start], edges: [] }];

  // BFS tracking: for each node, store all parent entries (multiple = multiple shortest paths)
  const parents = new Map<string, { parent: string; edge: AdjEdge }[]>();
  const dist = new Map<string, number>();
  dist.set(start, 0);
  parents.set(start, []);

  const queue: string[] = [start];
  let found = false;
  let targetDist = Infinity;

  while (queue.length > 0) {
    const current = queue.shift()!;
    const currentDist = dist.get(current)!;

    if (found && currentDist >= targetDist) break;

    for (const edge of adj.get(current) ?? []) {
      const next = edge.neighbor;
      const nextDist = currentDist + 1;

      if (!dist.has(next)) {
        dist.set(next, nextDist);
        parents.set(next, [{ parent: current, edge }]);
        queue.push(next);

        if (next === end) {
          found = true;
          targetDist = nextDist;
        }
      } else if (dist.get(next) === nextDist) {
        // Same distance — add another parent (alternative shortest path)
        parents.get(next)!.push({ parent: current, edge });
      }
    }
  }

  if (!found) return [];

  // Reconstruct all shortest paths via backtracking
  const results: BFSResult[] = [];

  function backtrack(node: string, path: string[], edges: AdjEdge[]) {
    if (node === start) {
      results.push({ path: [...path].reverse(), edges: [...edges].reverse() });
      return;
    }
    for (const { parent, edge } of parents.get(node) ?? []) {
      path.push(parent);
      edges.push(edge);
      backtrack(parent, path, edges);
      path.pop();
      edges.pop();
    }
  }

  backtrack(end, [end], []);
  return results;
}

/**
 * Find join paths connecting the selected tables via FK relationships.
 *
 * For 2 tables: BFS from A to B, returns all shortest paths.
 * For 3+ tables: Steiner tree approximation — iteratively connects each
 * unconnected table to the nearest already-connected table via BFS.
 *
 * Returns empty array if tables cannot be connected.
 */
export function findJoinPaths(
  selectedTables: { schema: string; name: string }[],
  fks: FKEdge[],
): JoinPath[] {
  if (selectedTables.length < 2) return [];

  const adj = buildAdjacency(fks);
  const selectedKeys = selectedTables.map((t) => tableKey(t.schema, t.name));

  // Check all selected tables exist in the graph
  for (const key of selectedKeys) {
    if (!adj.has(key)) return [];
  }

  if (selectedKeys.length === 2) {
    // Simple case: BFS between two tables, return all shortest paths
    const results = bfsAllShortest(adj, selectedKeys[0], selectedKeys[1]);
    return results.map((r) => bfsResultToJoinPath(r));
  }

  // 3+ tables: Steiner tree approximation
  // Start from first selected table, iteratively BFS to nearest unconnected
  const connected = new Set<string>([selectedKeys[0]]);
  const treeKeys: string[] = [selectedKeys[0]];
  const treeEdges: AdjEdge[] = [];
  const remaining = new Set(selectedKeys.slice(1));

  while (remaining.size > 0) {
    let bestPath: BFSResult | null = null;
    let bestTarget: string | null = null;

    for (const target of remaining) {
      // BFS from each connected node to the target, keep shortest
      for (const source of connected) {
        const paths = bfsAllShortest(adj, source, target);
        if (paths.length === 0) continue;
        // Take first shortest path
        if (!bestPath || paths[0].path.length < bestPath.path.length) {
          bestPath = paths[0];
          bestTarget = target;
        }
      }
    }

    if (!bestPath || !bestTarget) return []; // Disconnected graph

    // Add all nodes and edges from this path to the tree
    for (const key of bestPath.path) {
      connected.add(key);
      if (!treeKeys.includes(key)) treeKeys.push(key);
    }
    treeEdges.push(...bestPath.edges);
    remaining.delete(bestTarget);
  }

  // Build a single JoinPath from the Steiner tree
  const joinPath: JoinPath = {
    tables: treeKeys.map(parseKey),
    edges: treeEdges.map((e) => ({
      from: { schema: e.fromSchema, table: e.fromTable, col: e.fromCol },
      to: { schema: e.toSchema, table: e.toTable, col: e.toCol },
    })),
  };

  return [joinPath];
}

function bfsResultToJoinPath(result: BFSResult): JoinPath {
  return {
    tables: result.path.map(parseKey),
    edges: result.edges.map((e) => ({
      from: { schema: e.fromSchema, table: e.fromTable, col: e.fromCol },
      to: { schema: e.toSchema, table: e.toTable, col: e.toCol },
    })),
  };
}

/**
 * Convert a JoinPath into a JoinQueryState ready for SQL generation.
 * The first table in the path becomes the base table.
 * Tables not in the user's original selection are marked as intermediate.
 */
export function buildJoinState(
  path: JoinPath,
  selectedTables: { schema: string; name: string }[],
  _fks: FKEdge[],
): JoinQueryState {
  const selectedSet = new Set(
    selectedTables.map((t) => tableKey(t.schema, t.name)),
  );

  const baseTable = path.tables[0];
  const joins: JoinEntry[] = [];
  const visited = new Set<string>([tableKey(baseTable.schema, baseTable.name)]);

  // Build ON conditions from edges, grouping by table pair for composite FKs
  // Walk the path tables in order, finding the edges that connect each to the tree
  for (let i = 1; i < path.tables.length; i++) {
    const table = path.tables[i];
    const tKey = tableKey(table.schema, table.name);

    // Find all edges connecting this table to any already-visited table
    const conditions: string[] = [];
    for (const edge of path.edges) {
      const fromKey = tableKey(edge.from.schema, edge.from.table);
      const toKey = tableKey(edge.to.schema, edge.to.table);

      if (
        (fromKey === tKey && visited.has(toKey)) ||
        (toKey === tKey && visited.has(fromKey))
      ) {
        conditions.push(
          `${edge.from.schema}.${edge.from.table}.${edge.from.col} = ${edge.to.schema}.${edge.to.table}.${edge.to.col}`,
        );
      }
    }

    visited.add(tKey);

    joins.push({
      table: { schema: table.schema, name: table.name },
      joinType: "INNER",
      onCondition: conditions.join(" AND "),
      isIntermediate: !selectedSet.has(tKey),
    });
  }

  return {
    baseTable: { schema: baseTable.schema, name: baseTable.name },
    joins,
    selectedColumns: new Map(),
  };
}
