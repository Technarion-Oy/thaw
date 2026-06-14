# components/maskingpolicy

> Modals for creating and managing Snowflake MASKING POLICY objects.

## Components

| File | Purpose |
|---|---|
| `CreateMaskingPolicyModal.tsx` | Create form with a live `CREATE MASKING POLICY` SQL preview. Fields: name, OR REPLACE / IF NOT EXISTS, a **Signature** editor (rows of `name` + data-type `AutoComplete`; the first row is the masked column and pins the **Returns** type), a Monaco **Body** editor (the masking expression), comment, and a collapsible **Advanced options** section with `EXEMPT_OTHER_POLICIES`. |
| `MaskingPolicyPropertiesModal.tsx` | `SHOW`/`DESCRIBE MASKING POLICY` metadata: a read-only **Definition** (signature + return type), an editable **Body** (Monaco; saved via `SET BODY -> …`), an inline-editable **Comment**, and a **References** section that lists the columns the policy is applied to. |

## Integration

- Both delegate to IPC: `BuildCreateMaskingPolicySql` / `ExecDDL` (create) and
  `GetObjectProperties` / `AlterMaskingPolicy` / `GetMaskingPolicyReferences`
  (properties + edits).
- `AlterMaskingPolicy(db, schema, name, clause)` runs free-form
  `ALTER MASKING POLICY … <clause>` for `RENAME TO`, `SET BODY -> …`, and
  `SET`/`UNSET COMMENT`.
- `GetMaskingPolicyReferences(db, schema, name)` queries
  `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES` (filtered to
  `POLICY_KIND = 'MASKING_POLICY'`) for the live applications of the policy; it is
  loaded on demand (the view is slow and requires governance privileges), and its
  tabular `QueryResult` is rendered in an Ant Design `Table`.
- Wired into the object tree from `components/layout/Sidebar.tsx` under the
  **Masking Policies** group (kind `"MASKING POLICY"`). Masking policies are not
  queryable tables, so there is no **Select Top 1000 Rows**;
  `ALTER MASKING POLICY` supports `RENAME TO`, so **Rename** is offered.
- The form shape mirrors the Wails-generated `maskingpolicy.MaskingPolicyConfig`
  (with a nested `args` array); a plain object literal is cast `as any` only at
  the IPC boundary.

## Gotchas

- **The return type must match the first column's type** — the create modal pins
  **Returns** to the first signature row's type automatically so the two can't
  drift; the default body references the first column (`val`), so renaming it
  means updating the body too.
- The properties panel pulls the **signature**, **return type**, and **body** from
  `DESCRIBE MASKING POLICY` (appended by `internal/objects` `GetObjectProperties`)
  because `SHOW MASKING POLICIES` reports none of them.
