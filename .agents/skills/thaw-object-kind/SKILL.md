---
name: thaw-object-kind
description: >-
  Use this skill when adding, renaming, reordering, or modifying a Snowflake schema-scoped
  object kind in Thaw's object-kind registry, sidebar tree, search, or DDL export.
---

# Thaw Snowflake Object-Kind Registry Workflow

All schema-scoped Snowflake object kinds derive from a single canonical registry in `internal/objectkind/kinds.go`. Never introduce a parallel per-kind `switch` or hand-edit generated TypeScript.

> **Scope note:** Non-schema-scoped kinds (`DATABASE`, `SCHEMA`, `WAREHOUSE`, `ROLE`, `USER`) are deliberately outside `internal/objectkind` and stay explicit in their few consumers.

## 1. Update the Canonical Registry
1. Read `internal/objectkind/README.md` and `internal/objectkind/kinds.go`.
2. Add or update the entry in `objectkind.Kinds` positioned at its desired sidebar tree order, setting:
   - Canonical `KIND` string
   - `SHOW` plural keyword
   - Display label
   - `Basic` (`true` if already returned by `SHOW OBJECTS`, `false` if fetched by `ListExtendedObjects`)
   - `GET_DDL` object type (`""` if Snowflake `GET_DDL` does not support this kind)

## 2. Regenerate the Frontend Artifact
Run from the repository root:
```bash
go generate ./internal/objectkind/
```
This regenerates `frontend/src/generated/objectKinds.ts` (`OBJECT_KIND_ORDER`, `OBJECT_KIND_LABEL`, `DDL_UNSUPPORTED_KINDS`, type-filter options). Never hand-edit that file.

## 3. Wire Sidebar Icons & Theme Colours
1. Add the icon component and colour mapping for the kind in `frontend/src/components/sidebar/objectIcons.tsx`.
2. Declare the corresponding `--icon-*` CSS custom property in **both** the light and dark theme blocks of `frontend/src/styles/global.css`.

## 4. Wire Backend & UI Consumers (If Creating/Editing Properties)
1. Check `internal/objects/` (`BuildObjectPropertiesQuery`) and `internal/snowflake/` (`ListExtendedObjects`, search, DDL disposition) — most consumers pick up the new registry entry automatically, while custom create/properties modals follow the `thaw-ipc-feature` skill.
2. If you added a new domain package or frontend modal files, annotate with `// thaw:domain: Object Browser & Administration` / `// @thaw-domain: Object Browser & Administration` and run `go generate ./internal/architecture/`.

## 5. Run Coverage Guard Tests
Every step above is enforced by automated tests — run all of them:
```bash
go test ./internal/objectkind/... ./internal/snowflake/... ./internal/objects/... ./internal/architecture/...
cd frontend && npx tsc --noEmit && npx vitest run objectIcons
```
Specifically verify:
- `TestGeneratedObjectKindsInSync` (`internal/objectkind`)
- `TestEveryKindIsSearchable`, `TestEveryKindIsListable`, `TestEveryKindHasDDLDisposition` (`internal/snowflake`)
- `TestEveryKindHasPropertiesQuery` (`internal/objects`)
- `objectIcons.test.ts` (icon, colour, and `--icon-*` CSS variable in `global.css`)

## 6. Update Documentation
Update `FEATURES.md`, affected module `README.md` files, and `docs/concepts/` if applicable in the same PR.
