# components/sessionpolicy

Object-browser UI for Snowflake **SESSION POLICY** objects.

## Components

- **`CreateSessionPolicyModal.tsx`** — the create flow. Name + casing +
  `OR REPLACE` / `IF NOT EXISTS` (via the shared `NameWithReplaceOptions`), then
  the four timeout parameters grouped into **Idle timeout** and **Maximum
  lifespan** sections, plus a **Secondary roles** section (allowed/blocked
  `Select mode="tags"` — type `ALL` or role names; `reconcileAll` enforces the
  grammar's `ALL`-vs-role-list exclusivity as you type so the preview can't
  produce the invalid `('ALL', R1)`). Each timeout is an
  `InputNumber` bounded to its documented range; leaving it empty inherits
  Snowflake's default (shown as the placeholder) and omits it from the generated
  SQL. A live `SqlPreview` reflects `App.BuildCreateSessionPolicySql`; submit runs
  it via `ExecDDL`.
- **`SessionPolicyPropertiesModal.tsx`** — the properties panel. Loads
  `GetObjectProperties("SESSION POLICY")` (SHOW-level metadata) and
  `DescribeSessionPolicy` (a single row whose columns carry the current parameter
  values) together. The **Parameters** section renders every timeout with inline
  edit — *Save* issues `ALTER SESSION POLICY … SET <param> = N`, *Unset* issues
  `UNSET <param>` to restore the default. **Secondary roles** edits the
  allowed/blocked lists (`SET … = ('ALL')` / `(R1, …)` or `UNSET`; the tag editor
  also applies `reconcileAll`). Snowflake's `DESCRIBE SESSION POLICY` documents
  only `allowed_secondary_roles`; if the `blocked_secondary_roles` column is
  absent from the result the **Blocked** row shows `(unknown)` with a caveat that
  editing sets it blind, rather than misleadingly rendering `(default)`. The
  parse/serialize helpers (`secondaryRoles.ts`, unit-tested in
  `secondaryRoles.test.ts`) handle both the SQL-tuple and JSON-array shapes
  `DESCRIBE` may return, and emit role names bare unless they need quoting. The
  quoting decision is the shared `needsQuoting` (from `shared/ObjectNameCaseControl`,
  injected so `secondaryRoles.ts` stays runtime-free for tests), which — like the
  Go `snowflake.NeedsQuoting` the CREATE builder uses — double-quotes reserved
  keywords too, keeping the ALTER and CREATE paths in sync.
  **Settings** edits the comment. **References** lazily loads
  `GetSessionPolicyReferences` (the users/account the policy is attached to, from
  `ACCOUNT_USAGE.POLICY_REFERENCES`).

Both modals are wired into the sidebar context menu in
`components/layout/Sidebar.tsx`: the **Create Object → Security & Governance**
submenu, a per-type "Create Session Policy…" item, and the per-object
"Properties…", Rename, View Definition, Compare, and Drop items. The tree icon
(`FieldTimeOutlined`, `--icon-sessionpolicy`) lives in
`components/sidebar/objectIcons.tsx`.
