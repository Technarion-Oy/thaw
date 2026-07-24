// Pure matching logic for the object-browser advanced search (issue #855).
// Kept free of React/antd runtime so it can be unit-tested in the node test env
// (`DataNode` is a type-only import — erased at runtime). Sidebar.tsx builds a
// predicate once per query (memoized) and threads it through filterTree.

import type { DataNode } from "antd/es/tree";

// @thaw-domain: Object Browser & Administration

// A prebuilt matching predicate for the object search. Compiled once per query
// (memoized) rather than per node: the RegExp is built a single time, invalid
// patterns fall back to literal substring matching (with `regexError` set so the
// input can show an error state) and never throw, and the kind filter is a Set
// lookup.
//
// The backend account search (SearchAccountObjects) already scopes by kind and
// pushes a coarse LIKE for substring queries, so this predicate's job is the
// precise client-side pass: exact case handling and regex, which SQL LIKE can't
// express.
export interface SearchPredicate {
  // True when the search is engaged (a query, a kind filter, or both).
  active: boolean;
  // True when regex mode is on but the pattern failed to compile.
  regexError: boolean;
  matches: (name: string, kind: string) => boolean;
}

export function buildSearchPredicate(
  query: string,
  regexMode: boolean,
  caseSensitive: boolean,
  kinds: string[],
): SearchPredicate {
  const q = query.trim();
  const kindSet = kinds.length > 0 ? new Set(kinds) : null;

  let regexError = false;
  let nameTest: (name: string) => boolean;
  if (!q) {
    nameTest = () => true;
  } else if (regexMode) {
    try {
      const re = new RegExp(q, caseSensitive ? "" : "i");
      nameTest = (name) => re.test(name);
    } catch {
      // Invalid regex: never throw. Surface an error state and degrade to a
      // literal substring match so the user still gets useful results.
      regexError = true;
      const needle = caseSensitive ? q : q.toLowerCase();
      nameTest = (name) => (caseSensitive ? name : name.toLowerCase()).includes(needle);
    }
  } else {
    const needle = caseSensitive ? q : q.toLowerCase();
    nameTest = (name) => (caseSensitive ? name : name.toLowerCase()).includes(needle);
  }

  return {
    active: !!q || !!kindSet,
    regexError,
    matches: (name, kind) => (!kindSet || kindSet.has(kind)) && nameTest(name),
  };
}

// Filter the tree to obj: nodes whose (name, kind) satisfy `matches`, pruning
// empty structural parents (db/schema/type) but preserving the full loaded
// subtree of a matched object — its columns, stage/git/dbt files, or task
// subtree — so expanding a search hit shows real content instead of an empty
// node (finding 2). Parent task nodes match on their own name too; a matched
// node whose loaded children are empty is emitted as a leaf so it doesn't
// render a switcher that expands to nothing (finding 7).
//
// Name and kind come from the obj: key (`obj:<db>:<schema>:<KIND>:<name>`) rather
// than the node title, which may be a React element in some render paths.
export function filterTree(nodes: DataNode[], matches: SearchPredicate["matches"]): DataNode[] {
  return nodes.reduce<DataNode[]>((acc, node) => {
    const key      = String(node.key);
    const isObj    = key.startsWith("obj:");
    const children = (node as DataNode & { children?: DataNode[] }).children;
    let selfMatch = false;
    if (isObj) {
      const p = key.split(":");
      selfMatch = matches(p.slice(4).join(":"), p[3]);
    }
    if (children !== undefined) {
      if (selfMatch) {
        if (children.length === 0) {
          const { children: _drop, ...rest } = node as DataNode & { children?: DataNode[] };
          acc.push(rest as DataNode);
        } else {
          // Keep the matched object's own subtree intact (unfiltered) so its
          // columns / files / subtasks are all visible on expand.
          acc.push({ ...node, children });
        }
      } else {
        const filtered = filterTree(children, matches);
        if (filtered.length > 0) acc.push({ ...node, children: filtered });
      }
    } else if (isObj && selfMatch) {
      acc.push(node);
    }
    return acc;
  }, []);
}
