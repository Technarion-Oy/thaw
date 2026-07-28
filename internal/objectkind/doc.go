// SPDX-License-Identifier: GPL-3.0-or-later

// Package objectkind is the single canonical registry of Snowflake object kinds
// the object browser knows about.
//
// thaw:domain: Object Browser & Administration
//
// Before this package the same kind metadata — the canonical KIND string, its
// SHOW plural noun, the display label, the tree ordering, the GET_DDL object
// type — was spelled out independently in half a dozen places (the extended SHOW
// command list, the Properties query switch, the GET_DDL normalization switch,
// its reject-list, and the frontend's KIND_LABEL / KIND_ORDER maps). Adding a
// kind meant editing all of them and nothing failed when one was missed.
//
// Everything now derives from [Kinds]:
//
//   - internal/snowflake: the per-schema ListExtendedObjects command list, the
//     account-wide search plan, buildGetDDLQuery's object-type normalization and
//     DDLUnsupportedKinds.
//   - internal/objects: BuildObjectPropertiesQuery's SHOW plural.
//   - frontend/src/generated/objectKinds.ts: generated from this registry by
//     scripts/gen_object_kinds.go, and consumed by the sidebar tree grouping,
//     the search type filter, and the DDL-capability guard.
//
// Icons stay a hand-maintained frontend map (they are React components), guarded
// by a coverage test in frontend/src/components/sidebar/objectIcons.test.ts.
//
// The package is a leaf: it imports nothing from the rest of Thaw, so any
// package — including internal/snowflake — can depend on it without a cycle.
package objectkind
