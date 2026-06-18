# components/sessionpolicy

Object-browser UI for Snowflake **SESSION POLICY** objects.

## Components

- **`CreateSessionPolicyModal.tsx`** — the create flow. Name + casing +
  `OR REPLACE` / `IF NOT EXISTS` (via the shared `NameWithReplaceOptions`), then
  the four timeout parameters grouped into **Idle timeout** and **Maximum
  lifespan** sections, plus a **Secondary roles** section (allowed/blocked
  `Select mode="tags"` — type `ALL` or role names). Each timeout is an
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
  allowed/blocked lists (`SET … = ('ALL')` / `(R1, …)` or `UNSET`). The
  parse/serialize helpers (`secondaryRoles.ts`, unit-tested in
  `secondaryRoles.test.ts`) handle both the SQL-tuple and JSON-array shapes
  `DESCRIBE` may return, and emit role names bare unless they need quoting.
  **Settings** edits the comment. **References** lazily loads
  `GetSessionPolicyReferences` (the users/account the policy is attached to, from
  `ACCOUNT_USAGE.POLICY_REFERENCES`).

Both modals are wired into the sidebar context menu in
`components/layout/Sidebar.tsx`: the **Create Object → Security & Governance**
submenu, a per-type "Create Session Policy…" item, and the per-object
"Properties…", Rename, View Definition, Compare, and Drop items. The tree icon
(`FieldTimeOutlined`, `--icon-sessionpolicy`) lives in
`components/sidebar/objectIcons.tsx`.
