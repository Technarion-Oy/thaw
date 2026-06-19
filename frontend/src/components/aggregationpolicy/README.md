# components/aggregationpolicy

Object-browser UI for Snowflake **AGGREGATION POLICY** objects.

An aggregation policy enforces queries on a protected table or view to aggregate
their results to a minimum group size, so individual records cannot be
identified. It takes no arguments — the signature is always `()` and the return
type is always `AGGREGATION_CONSTRAINT` — so the only authored part is the body
expression (plus an optional comment).

## Components

- **`CreateAggregationPolicyModal.tsx`** — the create flow. Name (with
  identifier-casing + `OR REPLACE` / `IF NOT EXISTS`), a Monaco body editor
  (seeded with `AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)`), an optional
  comment, and a live SQL preview. The body returns
  `AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => n)` or
  `NO_AGGREGATION_CONSTRAINT()`, optionally wrapped in conditional logic (e.g. a
  `CASE` on `CURRENT_ROLE()`). Submits via `BuildCreateAggregationPolicySql` +
  `ExecDDL`.
- **`AggregationPolicyPropertiesModal.tsx`** — the properties panel. Reads the
  current body via `GetObjectProperties` (which merges the `DESCRIBE AGGREGATION
  POLICY` body), with inline editing (`SET BODY -> <expr>`), an editable comment
  (`SET`/`UNSET COMMENT`), the generic SHOW properties, and on-demand references
  (tables/views the policy is attached to) via
  `GetAggregationPolicyReferences`. All ALTER operations go through the free-form
  `App.AlterAggregationPolicy`.

## Wiring

Both modals are rendered and triggered from `components/layout/Sidebar.tsx`
(Create-Object → **Security & Governance** submenu, type-node "Create Aggregation
Policy…", object-node "Properties…", and DROP / RENAME via the shared paths).
The tree icon (`GroupOutlined`, emerald `--icon-aggregationpolicy`) lives in
`components/sidebar/objectIcons.tsx`.
