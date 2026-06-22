# components/contact

> Modals for creating and managing Snowflake CONTACT (notification-target) objects.

## Components

| File | Purpose |
|---|---|
| `CreateContactModal.tsx` | Create form with a live `CREATE CONTACT` SQL preview. Fields: name, OR REPLACE / IF NOT EXISTS (mutually exclusive — selecting one clears the other), a **contact-method** radio (Email distribution list / Snowflake users / URL — a contact has exactly one), the method-specific input (a multi-select **users dropdown** populated from `ListUsers`, or an email / URL text field), and an optional comment. |
| `ContactPropertiesModal.tsx` | `SHOW CONTACTS` metadata plus an **editable** contact method (radio + method-specific editor; saving runs `ALTER CONTACT … SET {USERS|EMAIL_DISTRIBUTION_LIST|URL}`) and an editable comment (`SET COMMENT`). The current method is derived from whichever of `email_distribution_list` / `url` / `users` is populated. |

## Integration

- Create delegates to IPC `BuildCreateContactSql` / `ExecDDL`; properties delegate
  to `GetObjectProperties` (`SHOW CONTACTS`) and `AlterContact` (free-form
  `ALTER CONTACT … SET …`). The USERS value list is built via `FormatContactUsers`
  and the `users` cell is tokenized with `ParseSqlList`.
- A contact names a notification target — a set of Snowflake users, an email
  distribution list, or a URL — used by alerts and other notification-based
  features. The three methods are **mutually exclusive**; the modal renders a
  method picker, not three independent fields.
- The Snowflake-user dropdown ("use drop down to add snowflake users") is
  best-effort: a `SHOW USERS` failure (insufficient privileges) leaves the list
  empty but the Select stays free-typeable.
- **`RENAME` is supported** (`ALTER CONTACT … RENAME TO`) and is reached from the
  sidebar context-menu **Rename…** item (the default rename case). `GET_DDL`
  supports contacts, so **View Definition** / comparison also work.
- `ALTER CONTACT` has **no `UNSET`** — changing the method is a `SET` of the new
  one; there is no way to clear a method other than replacing it.
