# components/sessionpolicy

Object-browser UI for Snowflake **SESSION POLICY** objects.

## Components

- **`CreateSessionPolicyModal.tsx`** — the create flow. Name + casing +
  `OR REPLACE` / `IF NOT EXISTS` (via the shared `NameWithReplaceOptions`), then
  the four timeout parameters grouped into **Idle timeout** and **Maximum
  lifespan** sections, plus a **Secondary roles** section (allowed/blocked
  `Select mode="tags"`). `ALL` is offered only on the **Allowed** list — the
  grammar accepts `'ALL'` only for `ALLOWED_SECONDARY_ROLES`, not for blocked — and
  there `App.ReconcileSecondaryRoles` enforces the `ALL`-vs-role-list exclusivity as
  you type so the preview can't produce the invalid `('ALL', R1)`; the **Blocked**
  list takes role names only and drops a typed `ALL` (invalid there). Each timeout
  is an `InputNumber` constrained to integers (`precision={0}`) within its
  documented range; leaving it empty inherits Snowflake's default (shown as the
  placeholder) and omits it from the generated SQL. A live `SqlPreview` reflects
  `App.BuildCreateSessionPolicySql`; submit runs it via `ExecDDL`.
- **`SessionPolicyPropertiesModal.tsx`** — the properties panel. Loads
  `GetObjectProperties("SESSION POLICY")` (SHOW-level metadata) and
  `DescribeSessionPolicy` (a single row whose columns carry the current parameter
  values) together. The **Parameters** section renders every timeout with inline
  edit (integer-only, `precision={0}`) — *Save* issues
  `ALTER SESSION POLICY … SET <param> = N`, *Unset* issues `UNSET <param>` to
  restore the default. **Secondary roles** edits the allowed/blocked lists
  (`SET … = ('ALL')` / `(R1, …)` or `UNSET`; the **Allowed** tag editor offers
  `ALL` and applies `App.ReconcileSecondaryRoles`, the **Blocked** one takes role
  names only and drops a typed `ALL`). Snowflake's `DESCRIBE SESSION POLICY` documents only
  `allowed_secondary_roles`; if the `blocked_secondary_roles` column is absent from
  the result the **Blocked** row shows `(unknown)` with a caveat that editing sets it
  blind, rather than misleadingly rendering `(default)` — and if DESCRIBE fails
  outright both role rows read `(unknown)`, matching the timeout rows. All
  secondary-role logic lives in Go and is reached over IPC, so parse, serialize, and
  ALL-reconciliation share one implementation: `App.ParseSecondaryRoles`
  (DESCRIBE cell → tokens, on load), `App.FormatSecondaryRoles` (tokens → the
  `( 'ALL' | <role>, … )` SQL list — used both to render the current value and to
  build the `ALTER … SET` clause, emitting role names bare unless they need quoting
  via the Go `snowflake.NeedsQuoting`), and `App.ReconcileSecondaryRoles`. Because
  formatting is async, the row display reads a pre-rendered string computed on load.
  This keeps the ALTER and CREATE paths in sync with no TypeScript role helpers
  (the former `secondaryRoles.ts` is gone; coverage is the Go tests in
  `internal/snowflake/identifiers_test.go`).
  **Settings** edits the comment. **References** lazily loads
  `GetSessionPolicyReferences` (the users/account the policy is attached to, from
  `ACCOUNT_USAGE.POLICY_REFERENCES`).

Both modals are wired into the sidebar context menu in
`components/layout/Sidebar.tsx`: the **Create Object → Security & Governance**
submenu, a per-type "Create Session Policy…" item, and the per-object
"Properties…", Rename, View Definition, Compare, and Drop items. The tree icon
(`FieldTimeOutlined`, `--icon-sessionpolicy`) lives in
`components/sidebar/objectIcons.tsx`.
