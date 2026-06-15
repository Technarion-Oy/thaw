# components/imagerepository

> Modals for creating and managing Snowflake IMAGE REPOSITORY objects.

## Components

| File | Purpose |
|---|---|
| `CreateImageRepositoryModal.tsx` | Create form with a live `CREATE IMAGE REPOSITORY` SQL preview. Fields: name, OR REPLACE / IF NOT EXISTS (mutually exclusive — selecting one clears the other), and a comment. |
| `ImageRepositoryPropertiesModal.tsx` | `SHOW IMAGE REPOSITORIES` metadata: a highlighted **Repository URL** (with copy button), an inline-editable **Comment**, a lazily-loaded **Images** table (`SHOW IMAGES IN IMAGE REPOSITORY`), and the generic property rows. |

## Integration

- Both delegate to IPC: `BuildCreateImageRepositorySql` / `ExecDDL` (create) and
  `GetObjectProperties` / `AlterImageRepository` / `ListImagesInRepository`
  (properties + edits + image listing).
- `AlterImageRepository(db, schema, name, clause)` runs free-form
  `ALTER IMAGE REPOSITORY … <clause>` for `SET`/`UNSET COMMENT` (the only mutable
  property).
- `ListImagesInRepository(db, schema, name)` returns the raw
  `snowflake.QueryResult` from `SHOW IMAGES IN IMAGE REPOSITORY`; the modal builds
  an antd `Table` directly from `columns`/`rows`, so it adapts to whatever columns
  the Snowflake edition reports (typically created_on, image_name, tags, digest,
  image_path). It is loaded on demand (not on open) to avoid an extra round-trip.
- Wired into the object tree from `components/layout/Sidebar.tsx` under the
  **Image Repositories** group (kind `"IMAGE REPOSITORY"`). Image repositories are
  not queryable tables, so there is no **Select Top 1000 Rows**; `ALTER IMAGE
  REPOSITORY` has no `RENAME TO`, so **Rename** is not offered.
- The form shape mirrors the Wails-generated
  `imagerepository.ImageRepositoryConfig`; a plain object literal is cast `as any`
  only at the IPC boundary.

## Gotchas

- **No `RENAME` and no `GET_DDL`** — image repositories can't be renamed and
  `GET_DDL` doesn't support the kind, so there's no DDL/View-Definition path; the
  properties panel relies entirely on `SHOW IMAGE REPOSITORIES`.
- **Repository URL is server-assigned** and read-only — only `COMMENT` can be
  changed after creation.
