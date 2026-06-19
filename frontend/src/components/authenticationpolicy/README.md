# components/authenticationpolicy

Object-browser UI for Snowflake **AUTHENTICATION POLICY** objects.

## Components

- **`CreateAuthenticationPolicyModal.tsx`** — the create flow. Name + casing +
  `OR REPLACE` / `IF NOT EXISTS` (via the shared `NameWithReplaceOptions`), then
  the policy parameters: **Authentication methods** and **Client types** as
  fixed-option multi-selects, **Security integrations** as a free-form
  `Select mode="tags"` (offers the `ALL` token, accepts integration names), and
  **MFA enrollment** as a single-choice `Select` (`REQUIRED` /
  `REQUIRED_PASSWORD_ONLY` / `OPTIONAL`), plus a comment. Leaving a parameter
  empty inherits Snowflake's default (the lists default to `ALL`, MFA enrollment
  to `OPTIONAL`) and omits it from the generated SQL. The three list selects
  reconcile `ALL`-vs-specific exclusivity through `App.ReconcileAllExclusiveList`
  (same as the Properties modal) so an invalid `('ALL', X)` list can't be built.
  A live `SqlPreview` reflects `App.BuildCreateAuthenticationPolicySql`; submit
  runs it via `ExecDDL`.
- **`AuthenticationPolicyPropertiesModal.tsx`** — the properties panel. Loads
  `GetObjectProperties("AUTHENTICATION POLICY")` (SHOW-level metadata) and
  `DescribeAuthenticationPolicy` (one row per property: `property`/`value`)
  together. The **Parameters** section renders each list parameter
  (`AUTHENTICATION_METHODS`, `CLIENT_TYPES`, `SECURITY_INTEGRATIONS`) with an
  inline tag editor — the parameter descriptors (keyword/label/allowed values/
  free-form flag) and the MFA-enrollment options come from
  `App.AuthenticationPolicyListParams` / `App.AuthenticationPolicyMFAEnrollmentOptions`,
  so the allowed values are not duplicated in TypeScript. *Save* issues
  `ALTER AUTHENTICATION POLICY … SET <param> = (…)` (the list is serialized by
  `App.FormatAuthPolicyList`, the same `('A', 'B')` serializer the CREATE builder
  uses), *Unset* issues `UNSET <param>` to restore the `ALL` default — plus an
  **MFA enrollment** single-choice row (`SET MFA_ENROLLMENT = <kw>` / `UNSET`).
  The DESCRIBE list cells (e.g. `[PASSWORD, SAML]`) are parsed back into tokens
  via `App.ParseSqlList` and the scalar normalized via `App.NormalizeSqlScalar`
  (the comment is quoted through `App.QuoteSqlText`), so the modal carries no SQL
  quoting/parsing logic; if DESCRIBE fails, a caveat notes that editing sets
  values blind. The multi-selects reconcile `ALL`-vs-specific exclusivity through
  `App.ReconcileAllExclusiveList` (also used by the bag `ALLOWED_METHODS` /
  `ALLOWED_PROVIDERS` selects), so a redundant `('ALL', X)` list can't be
  submitted. An **Advanced policies** section (`PolicyBagRows.tsx`) provides
  structured editors for the four nested property bags — `MFA_POLICY`,
  `PAT_POLICY`, `WORKLOAD_IDENTITY_POLICY`, `CLIENT_POLICY` — as selects /
  numbers / toggles / per-driver version rows. These editors hold only widget
  state: pre-fill goes through `App.Parse<Bag>` and Save through
  `App.Build<Bag>Value` (which returns the `( … )` clause), so no SQL grammar
  lives in TypeScript; the row then issues `ALTER … SET <BAG> = <value>` /
  `UNSET <BAG>`. A **Settings** section edits the comment and exposes
  **Detach from DCM project** (`UNSET DCM PROJECT`), and **References** loads the
  users/account the policy is attached to on demand via
  `App.GetAuthenticationPolicyReferences` (`POLICY_REFERENCES` —
  `POLICY_KIND = 'AUTHENTICATION_POLICY'`, governance-gated and latency-prone).
