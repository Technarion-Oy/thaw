# stage/

Frontend components for internal named stages.

| File | What it does |
|------|--------------|
| `UploadToStageModal.tsx` | Dialog for uploading a local file to a stage via `PUT`. Picks any file type through `PickAnyFile` (no SQL/dbt filter), lets the user enter an arbitrary destination path inside the stage (prefilled from the right-clicked directory, empty = stage root), and exposes the Overwrite / Auto-compress `PUT` options. Opened from the Sidebar stage and stage-directory context menus; `onSuccess` refreshes the originating tree node. |
