# components/passwordpolicy

Object-browser UI for Snowflake **PASSWORD POLICY** objects.

## Components

- **`CreatePasswordPolicyModal.tsx`** — the create flow. Name + casing +
  `OR REPLACE` / `IF NOT EXISTS` (via the shared `NameWithReplaceOptions`), then
  the 11 password parameters grouped into **Complexity**, **Age & history**, and
  **Retry & lockout** sections. Each parameter is an `InputNumber` bounded to its
  documented range; leaving it empty inherits Snowflake's default (shown as the
  placeholder) and omits it from the generated SQL. A live `SqlPreview` reflects
  `App.BuildCreatePasswordPolicySql`; submit runs it via `ExecDDL`.
- **`PasswordPolicyPropertiesModal.tsx`** — the properties panel. Loads
  `GetObjectProperties("PASSWORD POLICY")` (SHOW-level metadata) and
  `DescribePasswordPolicy` (per-parameter `value` + `default`) together. The
  **Parameters** section renders every parameter with inline edit — *Save*
  issues `ALTER PASSWORD POLICY … SET <param> = N`, *Unset* issues `UNSET <param>`
  to restore the default. **Settings** edits the comment. **References** lazily
  loads `GetPasswordPolicyReferences` (the users/account the policy is attached
  to, from `ACCOUNT_USAGE.POLICY_REFERENCES`).

Both modals are wired into the sidebar context menu in
`components/layout/Sidebar.tsx`: the **Create Object → Security & Governance**
submenu, a per-type "Create Password Policy…" item, and the per-object
"Properties…", Rename, View Definition, Compare, and Drop items. The tree icon
(`SafetyCertificateOutlined`, `--icon-passwordpolicy`) lives in
`components/sidebar/objectIcons.tsx`.
