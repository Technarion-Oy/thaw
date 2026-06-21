# internal/sequence

> SQL builder for Snowflake SEQUENCE objects.

## Responsibility

Builds the `CREATE SEQUENCE` DDL from a structured config. The edit operations
(`SET INCREMENT`, `SET`/`UNSET COMMENT`, `RENAME TO`) are simple enough that they
are issued as free-form `ALTER SEQUENCE <fqn> <clause>` statements directly from
`internal/app/sequence.go` (`App.AlterSequence`) without a dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `SequenceConfig`, `BuildCreateSequenceSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `SequenceConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Start` (START WITH), `Increment` (INCREMENT BY), `Ordered` (`""` / `"ORDER"` / `"NOORDER"`), comment |
| `BuildCreateSequenceSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] SEQUENCE [IF NOT EXISTS] <fqn> START WITH <n> INCREMENT BY <n> [ORDER\|NOORDER] [COMMENT='…'];` |

## Gotchas

- `Start` and `Increment` are emitted verbatim — the caller (Create modal)
  defaults both to `1`.
- `ORDER` / `NOORDER` is emitted only when `Ordered` is set to that value;
  Snowflake's default is `NOORDER`.
