# frontend/src/schemas

> Static JSON Schema files used to provide Monaco editor validation and autocompletion for dbt YAML files.

## Responsibility

Holds vendored JSON Schema (draft-07) definitions for dbt project configuration files. These schemas are imported at build time and registered with Monaco's JSON language service so that dbt YAML files opened in Thaw's editor receive inline validation and IntelliSense.

## Files

```
schemas/
└── dbt/
    ├── dbt_project-latest.json   # Schema for dbt_project.yml (required field: name)
    ├── packages-latest.json      # Schema for packages.yml and dependencies.yml
    ├── selectors-latest.json     # Schema for selectors.yml
    └── dbt_yml_files-latest.json # Schema for all other *.yml files (models, tests, sources, etc.)
```

| File | Covers |
|------|--------|
| `dbt/dbt_project-latest.json` | Top-level project configuration (`name`, `version`, `models`, `seeds`, `sources`, path lists, etc.) |
| `dbt/packages-latest.json` | Package dependency declarations (`packages` array with `package`+`version` or `git`+`revision`) |
| `dbt/selectors-latest.json` | Named graph selectors (`selectors` array with `name`, `definition`) |
| `dbt/dbt_yml_files-latest.json` | Generic dbt YAML: analyses, exposures, groups, macros, metrics, models, seeds, snapshots, sources, tests |

## Patterns & integration

The schemas are consumed exclusively by `frontend/src/components/editor/monacoSetup.ts`, which imports all four files and registers them with Monaco's JSON language worker via `monaco.languages.json.jsonDefaults.setDiagnosticsOptions`. File-to-schema matching is done by filename glob patterns:

| Filename pattern | Schema applied |
|-----------------|----------------|
| `dbt_project.yml` | `dbt_project-latest.json` |
| `packages.yml`, `dependencies.yml` | `packages-latest.json` |
| `selectors.yml` | `selectors-latest.json` |
| all other `*.yml` | `dbt_yml_files-latest.json` |

The `uri` field in each registration uses a custom `dbt-jsonschema://` scheme so the schemas do not conflict with other JSON schemas registered in the same Monaco instance.

## Gotchas

- These are **vendored snapshots**; they are not fetched at runtime from the dbt JSON Schema Store. Update them manually when dbt releases breaking changes to its schema.
- All four files use JSON Schema draft-07 (`$schema: http://json-schema.org/draft-07/schema#`). Monaco's JSON worker supports draft-07 natively; do not upgrade to draft-2019-09 or later without verifying Monaco compatibility.
- The schemas apply only when Monaco is running in YAML mode with `modelUri` set to a matching filename. Files opened without a URI or with a non-matching name receive no schema validation.
