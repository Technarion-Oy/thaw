# components/privacypolicy

> UI for creating and inspecting Snowflake PRIVACY POLICY objects.

## Components

| File | Purpose |
|---|---|
| `CreatePrivacyPolicyModal.tsx` | Create dialog: name + replace/if-not-exists options, case control, a fixed (disabled) signature, an "Enforce privacy budget" switch that rewrites the body between `PRIVACY_BUDGET(…)` and `NO_PRIVACY_POLICY()`, a Monaco body editor, an optional comment, and a live SQL preview. The signature is fixed to `() RETURNS PRIVACY_BUDGET`. |
| `PrivacyPolicyPropertiesModal.tsx` | Properties dialog: Definition (signature / returns), an editable Body (`SET BODY -> …`), an editable Comment, a Tags section (`SET`/`UNSET TAG` with removable chips), the generic SHOW properties, and an on-demand References table (tables/views the policy is applied to). |

## Integration

- Both modals are rendered from `components/layout/Sidebar.tsx`, opened via the
  schema "Create Object → Security & Governance" submenu / the `PRIVACY POLICY`
  type node and the object node's "Properties…" entry. `RENAME TO` is reached via
  the context-menu "Rename…" item (default rename path).
- IPC: `BuildCreatePrivacyPolicySql` + `ExecDDL` (create), `GetObjectProperties`
  (load), `AlterPrivacyPolicy` (edit body / comment / tags / rename),
  `GetPrivacyPolicyTags` (tag chips), and `GetPrivacyPolicyReferences`
  (references).
- `PrivacyPolicyConfig` has no nested arrays, but the plain form-state object is
  still cast to `any` at the IPC boundary for consistency with the other create
  modals.

## Notes

- A privacy policy has a fixed, argument-less signature returning
  `PRIVACY_BUDGET`; the body is a `PRIVACY_BUDGET(BUDGET_NAME => '…', …)` or
  `NO_PRIVACY_POLICY()` expression. Budget parameters are `BUDGET_NAME`
  (required), `BUDGET_LIMIT`, `MAX_BUDGET_PER_AGGREGATE`, and `BUDGET_WINDOW`.
- After creation, attach the policy to a table or view with
  `ALTER TABLE … ADD PRIVACY POLICY … ENTITY KEY (col, …)`.
- The Tags chips read `INFORMATION_SCHEMA.TAG_REFERENCES` (immediate
  consistency); the listing is best-effort, so `SET`/`UNSET TAG` still work if the
  read fails.
- The References table reads `ACCOUNT_USAGE.POLICY_REFERENCES`, which needs
  governance privileges and has propagation latency, so it is loaded on demand.
- Privacy policies require Enterprise Edition (or higher).
