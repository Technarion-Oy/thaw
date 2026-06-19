# components/projectionpolicy

Object-browser UI for Snowflake **PROJECTION POLICY** objects.

A projection policy controls whether a protected **column** can appear in query
output — i.e. whether it can be **projected** via `SELECT`. Unlike a masking
policy, which transforms values, a projection policy prevents the column from
being selected at all. It takes no arguments — the signature is always `()` and
the return type is always `PROJECTION_CONSTRAINT` — so the only authored part is
the body expression (plus an optional comment).

## Components

- **`CreateProjectionPolicyModal.tsx`** — the create flow. Name (with
  identifier-casing + `OR REPLACE` / `IF NOT EXISTS`), a Monaco body editor
  (seeded with `PROJECTION_CONSTRAINT(ALLOW => true)`), an optional comment, and
  a live SQL preview. The body returns `PROJECTION_CONSTRAINT(ALLOW => true)` or
  `PROJECTION_CONSTRAINT(ALLOW => false)`, optionally wrapped in conditional
  logic (e.g. a `CASE` on `CURRENT_ROLE()`). Submits via
  `BuildCreateProjectionPolicySql` + `ExecDDL`.
- **`ProjectionPolicyPropertiesModal.tsx`** — the properties panel. Reads the
  current body via `GetObjectProperties` (which merges the `DESCRIBE PROJECTION
  POLICY` body), with inline editing (`SET BODY -> <expr>`), an editable comment
  (`SET`/`UNSET COMMENT`), the generic SHOW properties, and on-demand references
  (columns the policy is attached to) via `GetProjectionPolicyReferences`. All
  ALTER operations go through the free-form `App.AlterProjectionPolicy`.

## Wiring

Both modals are rendered and triggered from `components/layout/Sidebar.tsx`
(Create-Object → **Security & Governance** submenu, type-node "Create Projection
Policy…", object-node "Properties…", and DROP / RENAME via the shared paths).
The tree icon (`ColumnWidthOutlined`, orange `--icon-projectionpolicy`) lives in
`components/sidebar/objectIcons.tsx`.
