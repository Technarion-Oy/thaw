# internal/contact

> SQL builder for Snowflake CONTACT objects.

## Responsibility

Builds the `CREATE CONTACT` DDL from a structured config. A contact is a
schema-level object that names a notification target used by alerts and other
notification-based features. It carries exactly **one** contact method — a set
of users, an email distribution list, or a URL — plus an optional comment:

```
CREATE [OR REPLACE] CONTACT [IF NOT EXISTS] <fqn>
  [ { USERS = ('u1' [, 'u2' ...])
    | EMAIL_DISTRIBUTION_LIST = '<email>'
    | URL = '<url>' } ]
  [ COMMENT = '<string>' ]
```

The three methods are mutually exclusive (a contact has a single "type"). The
contact name and a free-form `clause` drive `ALTER CONTACT` for RENAME / SET
operations, handled by the generic `App.AlterContact` delegator rather than a
dedicated builder here.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `ContactConfig`, method consts, `FormatContactUsers`, `BuildCreateContactSql` |
| `sql_test.go` | Unit tests for the SQL builder + the user-list formatter |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `ContactConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Method`, `Users`, `Email`, `URL`, `Comment` |
| `MethodUsers` / `MethodEmail` / `MethodURL` | Method identifiers (`"users"` / `"email"` / `"url"`) selecting which contact method is emitted |
| `FormatContactUsers(users)` | Renders the parenthesised, single-quoted `('u1', 'u2')` user list (also used by the properties panel to build `SET USERS`) |
| `BuildCreateContactSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] CONTACT [IF NOT EXISTS] <fqn> [<method>] [COMMENT='…'];` |

## Patterns & integration

- A blank name emits the placeholder `contact_name`, and a method selected with
  no value yet emits no method clause, so the live SQL preview reads as a
  completable template while the user is still typing.
- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive in Snowflake; the
  builder drops `IF NOT EXISTS` when `OrReplace` is also set (and the create
  modal prevents selecting both).
- `App.BuildCreateContactSql` (in `internal/app/builders.go`) is the thin IPC
  delegator for the create preview; `App.AlterContact` (in
  `internal/app/contact.go`) runs free-form `ALTER CONTACT … <clause>` for the
  properties panel (SET method / SET COMMENT) and the sidebar Rename action.
- The create + properties panels populate the **USERS** method from a dropdown
  of Snowflake users via the existing `App.ListUsers` IPC (no new backend).
- Discovery: `Client.ListExtendedObjects` runs `SHOW CONTACTS IN SCHEMA` with the
  fixed kind `"CONTACT"`. Contacts are not surfaced by `SHOW OBJECTS`, so — like
  tags, policies, and services — no dedupe pass is needed.
- Properties panel: `internal/objects` runs `SHOW CONTACTS LIKE …` for the
  `CONTACT` kind; SHOW already returns every column the panel needs
  (`email_distribution_list`, `url`, `users`, `entries_in_users`, `comment`), so
  there is no `DESCRIBE`-enrichment block.

## Gotchas

- **The three contact methods are mutually exclusive** — the grammar is
  `{ USERS | EMAIL_DISTRIBUTION_LIST | URL }`, so a contact has a single method.
  `Method` selects which one the builder emits; the modal renders a method
  picker rather than three independent fields.
- **`ALTER CONTACT` has no `UNSET`** — only `RENAME TO` and `SET` of the same
  method/comment options. Changing the method is a `SET` of the new one.
- **`GET_DDL` *is* supported** for contacts (object_type `'CONTACT'`, no space),
  and the SHOW kind already equals that object_type, so the default
  `ddlKind := kind` path works — there is **no** `buildGetDDLQuery` case and
  contacts are **not** excluded from "View Definition" / comparison.
- **`RENAME` is supported** (`ALTER CONTACT … RENAME TO`), so contacts are **not**
  added to the sidebar Rename-exclusion; the default rename case handles them.
- **`SHOW CONTACTS` has no verification-status column** — it reports the contact
  methods (`users` / `email_distribution_list` / `url`) and metadata only.
