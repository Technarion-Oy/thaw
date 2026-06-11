# Thaw — Feature Overview

Thaw is a native desktop application for Snowflake — built for analysts, engineers, and administrators who need a fast, capable SQL environment beyond the Snowflake web UI.

---

## SQL Editor

- **Monaco-based editor** with full SQL syntax highlighting and rich keyboard shortcuts
- **Multi-tab editing** — open multiple files simultaneously; each tab remembers its SQL, results, and scroll position
- **Run selected text** — highlight any portion of a query and run only that part (`⌘ Enter` / `Ctrl+Enter`)
- **Multi-statement scripts** — separate statements with `;`; all statements execute sequentially on a dedicated Snowflake session so `LAST_QUERY_ID(-1)` and `RESULT_SCAN` work correctly across statements, matching Snowsight behaviour; the spinner shows **statement N of M** and the Snowflake query ID for the active statement while the script runs; the currently-executing statement is highlighted in the editor with an amber background and a gutter indicator so you always know exactly where execution is — works whether running the full buffer or a painted selection of multiple statements
- **Cancel queries** — cancel a running query at any time; Thaw issues `SYSTEM$CANCEL_QUERY` so it also stops consuming Snowflake credits
- **Query ID** — the Snowflake Query ID is shown in the spinner while running (per-statement for multi-statement scripts) and in the results status bar after completion; click the copy icon to copy it to the clipboard
- **Multi-cursor editing** — `⌘⌥↑` / `Ctrl+Alt+↑` adds a cursor on the line above; `⌘⌥↓` / `Ctrl+Alt+↓` adds one below; works in the SQL editor, YAML editor, and all notebook cell editors; matches VS Code behaviour
- **Cross-tab search & replace** — press `⌘⇧H` / `Ctrl+Shift+H` to open a search/replace panel above the editor that searches across all open tabs (SQL, YAML, Python, and notebook cells); navigate between matches with Enter/Shift+Enter (automatically switches tabs); supports case-sensitive matching and regular expressions (with capture-group back-references like `$1`, `$2` in replace); replace single or all occurrences in one action; replace on the active tab integrates with Monaco's undo stack (Ctrl+Z); toggleable via **View → Enabled Features → Cross-Tab Search & Replace**
- **Selection highlight** — selecting text highlights every other occurrence in the document; overview-ruler markers show occurrences in long files
- **Toggle line comment** — `⌘/` / `Ctrl+/` (or right-click → **Toggle Line Comment**) adds or removes `--` on the current line or on every line in the selection
- **Font size zoom** — `⌘+` / `Ctrl++` increases the editor font size, `⌘-` / `Ctrl+-` decreases it, `⌘0` / `Ctrl+0` resets to the default
- **Code folding** — fold arrows are always visible in the editor gutter; click to collapse or expand any SQL block — CTEs, `BEGIN…END` blocks, subqueries, and multi-line expressions
- **Hover definitions** — move the cursor over any table or view name — including fully-qualified three-part identifiers (`DB.SCHEMA.TABLE`) and double-quoted identifiers (`"MY_TABLE"`, `"DB"."SCHEMA"."TABLE"`) — to see its DDL in a scrollable overlay tooltip; the tooltip fires as the cursor enters the token (not just when stationary at the end), stays open when the cursor moves into it, and auto-loads object metadata for schemas not yet expanded in the sidebar:
  - **Copy button** — copies the full DDL to the clipboard
  - **Text selection** — paint any portion of the DDL and copy with `⌘C` / `Ctrl+C`
  - **Right-click → Copy** — right-click inside the tooltip to copy the selected text via a context menu
  - Definitions are cached per session and refreshed automatically after 60 seconds
  - **Function tooltips** — hovering over a bare function name (e.g. `DATEADD`, `FLATTEN`, or a UDF) shows all overloads with their full signatures and descriptions in the same overlay; backed by an embedded catalogue of ~320 built-in functions that is always available offline, and refreshed with live metadata after each Snowflake connection
- **Function call highlighting** — every function call in the editor is syntax-coloured by kind: built-in Snowflake functions appear in **gold** and user-defined functions appear in **teal**, making it easy to distinguish system functions from custom logic at a glance; highlighting updates as you type (200 ms debounce) and is seeded from a local SQLite cache on editor mount so it works without a live connection
- **Live SQL diagnostics** — squiggly-line markers appear 400 ms after each edit and clear automatically once fixed; no false positives on well-formed Snowflake SQL:
  - **Syntax errors** (red squiggly) — unclosed string literals, unclosed quoted identifiers, unclosed dollar-quoted strings, unclosed block comments, unmatched parentheses/brackets, and tokens that appear after a semicolon but are not a recognised SQL statement keyword; inside `$$` scripting blocks, placeholder text (`<wrong_text>`, `{placeholder}`), bare unrecognised identifiers, and other tokens that cannot open a valid scripting statement are all flagged
  - **Grammar warnings** (yellow squiggly) — Snowflake-dialect PEG parser checks full grammatical structure of SELECT, INSERT, UPDATE, CREATE, DROP, ALTER, and related statements; Snowflake-specific constructs unsupported by the parser (such as CREATE STREAM, CREATE PIPE, CREATE ALERT, CREATE PROCEDURE, CREATE FUNCTION, CREATE HYBRID TABLE, CREATE NETWORK POLICY, CREATE ROW ACCESS POLICY, CREATE SESSION POLICY, CREATE PASSWORD POLICY, CREATE FILE FORMAT, CREATE SHARE, ALTER SHARE, CREATE DATASHARE, ALTER DATASHARE, DROP DATASHARE, CREATE SERVICE, EXECUTE SERVICE, ALTER SERVICE, DROP SERVICE, CREATE IMAGE REPOSITORY, DROP IMAGE REPOSITORY, ALTER IMAGE REPOSITORY, CREATE NOTEBOOK, ALTER NOTEBOOK, DROP NOTEBOOK, CREATE DYNAMIC TABLE, ALTER DYNAMIC TABLE, GRANT, or REVOKE) are validated via Go-side regex patterns to catch structural errors and avoid false positives; GRANT and REVOKE validation covers privilege-to-object-type compatibility (TABLE, VIEW, STAGE, WAREHOUSE, DATABASE, SCHEMA, ROLE, INTEGRATION, TASK, STREAM, USER, ACCOUNT), missing grantee/FROM clauses, WITH GRANT OPTION misuse on role grants, ON ALL/FUTURE without IN SCHEMA/DATABASE, and mutually exclusive CASCADE/RESTRICT modifiers; CREATE SHARE validation covers account-level prefix enforcement and COMMENT-only properties; ALTER SHARE validation covers RESTRICT only with ADD ACCOUNTS and ADD ACCOUNTS requiring at least one account identifier; CREATE DATASHARE validation covers account-level prefix enforcement, COMMENT and SHARE_RESTRICTIONS properties, and OR REPLACE / IF NOT EXISTS conflict; ALTER DATASHARE validation covers ADD/REMOVE ACCOUNTS, ADD/REMOVE DATABASES, SET/UNSET COMMENT sub-commands with missing-identifier and unknown-sub-command warnings; DROP DATASHARE validation requires a name; CREATE DATABASE ... FROM SHARE validates the mandatory two-part provider_account.share_name; CREATE SERVICE validation covers mandatory IN COMPUTE POOL and FROM SPECIFICATION/SPECIFICATION_FILE clauses, OR REPLACE / IF NOT EXISTS conflict, MIN_INSTANCES/MAX_INSTANCES range and cross-checks, AUTO_RESUME boolean validation, and property allow-list enforcement; EXECUTE SERVICE (job service) validation covers the same mandatory clauses as CREATE SERVICE and flags MIN_INSTANCES/MAX_INSTANCES as unsupported; ALTER SERVICE validation covers SUSPEND, RESUME, SET/UNSET properties, and FROM SPECIFICATION sub-commands with MIN_INSTANCES/MAX_INSTANCES range checks; DROP SERVICE validation requires a name; CREATE IMAGE REPOSITORY validation covers OR REPLACE / IF NOT EXISTS conflict, mandatory name, and COMMENT-only property allow-list; DROP IMAGE REPOSITORY validation requires a name; ALTER IMAGE REPOSITORY warns that the operation is unsupported; SNOWFLAKE.CORTEX.* AI function calls are recognised as a built-in system namespace — known Cortex function names (COMPLETE, EXTRACT_ANSWER, SENTIMENT, SUMMARIZE, TRANSLATE, CLASSIFY_TEXT, EMBED_TEXT_768, EMBED_TEXT_1024, FINETUNE, SEARCH_PREVIEW, TRY_COMPLETE) produce no false positives while unknown Cortex function names are flagged with a warning; CREATE NOTEBOOK validation covers mandatory name, OR REPLACE / IF NOT EXISTS conflict, and MAIN_FILE requirement when FROM is specified; ALTER NOTEBOOK validation covers mandatory name, known sub-commands (SET, UNSET, RENAME TO, ADD LIVE VERSION FROM LAST), and RENAME TO requiring a target name; DROP NOTEBOOK validation covers mandatory name and CASCADE / RESTRICT rejection; PIVOT clause validation covers valid aggregate functions (SUM, AVG, COUNT, MAX, MIN, ANY_VALUE, LISTAGG, MEDIAN, STDDEV, VARIANCE), mandatory FOR … IN syntax, and non-empty IN value list; UNPIVOT clause validation covers mandatory FOR … IN syntax and non-empty IN column list; both PIVOT and UNPIVOT suppress false-positive bare-column-reference warnings for dynamically generated virtual columns; MATCH_RECOGNIZE clause validation covers mandatory PATTERN clause (must contain at least one pattern variable), mandatory DEFINE clause, ONE ROW PER MATCH / ALL ROWS PER MATCH mutual exclusion, and AFTER MATCH SKIP target validation (TO NEXT ROW, PAST LAST ROW, TO FIRST <variable>, TO LAST <variable>); MATCH_RECOGNIZE suppresses false-positive bare-column-reference and table-existence warnings for pattern variable aliases defined in DEFINE clauses; ASOF JOIN clause validation covers mandatory MATCH_CONDITION clause (or USING FUNCTION alternative), valid comparison operators (>=, >, <=, < only — =, <>, != are rejected), and rejection of ON/USING clauses which are not supported with ASOF JOIN; multi-table INSERT validation covers INSERT ALL (unconditional and conditional with WHEN/THEN INTO/ELSE), INSERT FIRST (requires at least one WHEN branch), mandatory trailing SELECT, WHEN branch must contain INTO clause, and INSERT OVERWRITE INTO structural checks (mandatory INTO keyword, mandatory source SELECT or VALUES clause); ALTER TABLE ADD/DROP SEARCH OPTIMIZATION validation covers bare form acceptance (no ON clause), ON clause expression type validation (EQUALITY, SUBSTRING, GEO, FULL_TEXT are valid — unknown types are flagged), and empty ON clause detection; ALTER TABLE SWAP WITH validation covers missing target table name, same-table no-op detection, and trailing clause rejection; ALTER DYNAMIC TABLE lifecycle command validation covers REFRESH, SUSPEND, RESUME, SET, UNSET, SWAP WITH, and RENAME TO sub-commands — mandatory table name, unknown sub-command detection, SWAP WITH / RENAME TO missing-target-name warnings, and SET TARGET_LAG value validation (quoted duration like '1 minute' or DOWNSTREAM — invalid values are flagged); CREATE EXTERNAL VOLUME validation:
    - STORAGE_LOCATIONS mandatory
    - valid STORAGE_PROVIDER values: S3, S3GOV, S3CHINA, S3COMPAT, GCS, AZURE
    - STORAGE_BASE_URL required in each location
    - STORAGE_AWS_ROLE_ARN required for S3, S3GOV, S3CHINA, and S3COMPAT providers
    - AZURE_TENANT_ID required for AZURE provider
    - STORAGE_AWS_EXTERNAL_ID restricted to S3-family providers
    - ENCRYPTION TYPE must be NONE, AWS_SSE_S3, AWS_SSE_KMS, or GCS_SSE_KMS, matched to the location provider
    - ALLOW_WRITES must be TRUE or FALSE
    - account-level prefix (database or schema) not allowed
  - **Column existence warnings** (yellow squiggly) — bare unquoted and double-quoted column names in SELECT lists are validated against the table's column list; `alias.column` two-part references are also checked; column metadata is fetched automatically on first use and cached; silent while cache is cold
  - **Hover tooltip** — hovering any marker shows a compact `ERROR — …` or `WARNING — …` tooltip with the problem description; works on both identifier tokens and non-identifier characters (e.g. an unmatched parenthesis or an opening quote)
- **Explain SQL** — right-click any query and choose **Explain SQL** to see the Snowflake execution plan before running it; detects and highlights performance anti-patterns directly in the editor:
  - **Full Table Scans** (Yellow/Red) — warns when a large table is scanned without selective filters; the tooltip shows exactly how many partitions are being scanned
  - **Cartesian Joins** (Red) — flags joins that produce a cross-product of rows, preventing catastrophic query runs
  - **Row Explosion** (Yellow/Red) — warns when an equi-join is estimated to produce more than 10M rows (Warning) or 1B rows (Error)
  - Hovering over a performance marker shows a detailed tooltip with the specific operation (e.g. `TableScan`, `Join`), object name, and estimated row/partition counts
- **SQL autocomplete** — context-aware completions:
  - `db.` → schemas in that database
  - `db.schema.` → tables, views, functions, and other objects in that schema
  - `db.schema.table.` → columns of that table or view
  - `Ctrl+Space` inside a query → columns from all tables referenced in the current `FROM`/`JOIN` clauses
  - **CTE column projection** — `WITH cte AS (SELECT id, name FROM t) SELECT cte.` → suggests `id`, `name` from the CTE's projected columns; works with multiple CTEs and nested references
  - **USING clause completion** — after `USING (` in a JOIN clause → suggests shared column names between the two joined tables; filters out already-listed columns in partial USING expressions
  - **Quick-fix table qualification** — when a table name cannot be resolved, a lightbulb quick-fix offers to replace it with the fully-qualified `DB.SCHEMA.TABLE` path if the same table name exists in other schemas
  - After `ON` in a `JOIN` clause → join conditions in three tiers: **(1)** FK relationships — composite multi-column constraints produce a single `col1 = ref.col1 AND col2 = ref.col2` expression (sourced from `SHOW IMPORTED KEYS`); **(2)** PK-naming-convention heuristic (`orders.CUSTOMER_ID = customers.ID`) when no FK constraint exists; **(3)** type-compatible same-name columns with both `a.col = b.col` equality and `USING (col)` alternatives; works with quoted/unquoted identifiers, full three-part names, and optional table aliases
  - **Ghost text before ON** — after `JOIN table ` (before typing `ON`), an inline ghost-text suggestion `ON <condition>` appears and can be accepted with `Tab` (FK-cache-backed, instant)
  - **Ctrl+Space before ON** — pressing `Ctrl+Space` after a JOIN table reference but before typing `ON` opens a full dropdown of `ON <condition>` suggestions covering all three tiers
  - **Function completions** — typing two or more characters outside a dotted context also suggests matching Snowflake built-in and user-defined functions from the local cache; UDFs sort above built-ins so custom functions surface first; instant and available offline
- **AI inline completions** — ghost-text SQL suggestions powered by OpenAI or Google AI Studios (Gemini); press `Tab` to accept
- **SQL formatter** — right-click anywhere in the editor and choose **Format SQL** (`⇧⌥F` / `Shift+Alt+F`) to format the selection or the full document; open **View → Editor Preferences…** to customise:
  - **Keyword casing** — `UPPER`, `lower`, `Title`, or `Preserve` — reserved words (`SELECT`, `FROM`, `WINDOW`, `QUALIFY`, …)
  - **Identifier casing** — `Preserve`, `UPPER`, or `lower` — unquoted table/column names only; double-quoted identifiers are never modified
  - **Function casing** — `UPPER` or `lower` — all function calls including UDFs
  - **Indent style** — Spaces or Tabs; size 2 (recommended for Snowflake) or 4
  - **Comma position** — Trailing or Leading
  - **AND / OR position** — Before or After the line break
  - **Snowflake-specific rules** always applied: `::` and `:` operators kept whitespace-free; `WITH` on its own line; LATERAL FLATTEN treated as a unit
  - **Live preview** panel in the preferences dialog shows a Snowflake sample query updating in real time
- **Code Snippets** — open **Tools → Code Snippets…** in the menu bar to browse 24 curated `CREATE OR REPLACE` templates across six categories:
  - **Data Objects** — Table, View, Materialized View, Dynamic Table, Sequence
  - **Code** — Stored Procedure (Snowflake Scripting), Stored Procedure (Python), UDF (SQL), UDF (JavaScript), UDF (Python)
  - **Automation** — Task, Stream on Table, Pipe, Alert
  - **Storage** — Stage (Internal), Stage (External S3), File Format (CSV), File Format (Parquet)
  - **Governance** — Network Policy, Resource Monitor
  - **Infrastructure** — Database, Schema, Warehouse
  - Live search filters by snippet name across all categories; the first match is auto-selected; clicking **Open in New Tab** loads the SQL into a new scratch tab for review and customisation — not auto-executed
- **Unsaved-change indicator** — a `•` dot in the tab title shows unsaved work at a glance
- **Close confirmation** — closing a tab with unsaved changes (via the `×` button or `⌘W` / `Ctrl+W`) shows a dialog with three choices: **Save**, **Close without Saving**, or **Cancel**; for new scratch tabs or files not yet saved to disk, **Save** opens a native Save As dialog first; applies to SQL files, notebooks, and any scratch tab that has been edited
- **Tab reordering** — drag any tab left or right to rearrange the tab strip; a vertical accent line shows the insertion point
- **Split view** — right-click any tab and choose **Split with: [tab name]** to view two editors side by side; a draggable vertical divider separates them and the ratio is persisted across sessions; each editor is fully independent with its own completions, hover definitions, and editing history; close the split with the × button in the secondary editor header, via **Close split view** in the right-click menu, or by closing either of the two tabs
- **Snowflake Scripting Support** — advanced support for Snowflake Scripting (used in Stored Procedures and UDFs):
  - **Syntax Highlighting** — distinct coloring for scripting keywords (`DECLARE`, `BEGIN`, `EXCEPTION`, `END`), control flow (`IF`, `LOOP`, `WHILE`), and async operations (`ASYNC`, `AWAIT`)
  - **Right-click Code Snippets** — right-click anywhere in the SQL editor and hover over **Code Snippets →** to open a cascading submenu of Snowflake Scripting templates grouped into seven categories:
    - **Block Structure** — `block` (DECLARE / BEGIN / EXCEPTION / END with correct `WHEN exception THEN` / `WHEN OTHER THEN` handlers), `declare` (block without EXCEPTION)
    - **DECLARE Variables** — `declare var` (`variable_name type DEFAULT expression;`), `declare var (type only)` (`variable_name type;` — NULL until assigned)
    - **LET Variables** — `let (typed)` (`LET variable_name type DEFAULT|:= expression;`), `let` (`LET variable_name DEFAULT|:= expression;` — type inferred)
    - **Conditionals** — `if` (IF / ELSEIF / ELSE / END IF), `case` (CASE / WHEN / ELSE / END CASE)
    - **Loops** — `for`, `for_reverse`, `while`, `repeat`, `loop`
    - **Cursors & Resultsets** — `cursor_lifecycle` (OPEN / FETCH / CLOSE), `resultset`, `execute_immediate` (dollar-quoted `EXECUTE IMMEDIATE $$ … $$;`), `execute_immediate_using` (with USING bind variables)
    - **Async Jobs** — `async_job`, `await_job`, `cancel_job`
    - The submenu opens on hover (auto-flips left if there is insufficient space to the right); clicking any item inserts the snippet at the cursor with **keyword casing and indentation** automatically applied from **View → Editor Preferences…** — changing preferences takes effect on the next insertion with no restart required
  - **Autocomplete Snippets** — the same templates are also available as autocomplete suggestions; type the snippet label (e.g. `block`, `if`, `for`) and press Enter or Tab to expand
  - **Transparent Dollar Quoting** — code inside `$$...$$` or `$tag$...$tag$` is treated as normal SQL for highlighting, diagnostics, and hover tooltips, perfect for Snowflake Scripting development

---

## Object Browser

- Browse all databases → schemas → tables, views, functions, procedures, sequences, stages, streams, tasks, file formats, pipes, notebooks, secrets, and git repositories
- **Multi-selection** — hold `⌘` (macOS) or `Ctrl` (Windows/Linux) and click anywhere on an object row to select it; selected objects are highlighted across the full width of the sidebar; click any non-modifier area to clear the selection
- **Batch deletion** — when multiple objects are selected, right-click any of them and choose **Delete N selected objects…** to drop all of them in one operation; a confirmation dialog lists all objects to be removed
- **Secret Management** — right-click any schema and choose **Create Object** → **Secret…** or right-click an existing secret and choose **Modify…** to open the secret management dialog:
  - **Dynamic Form** — fields update automatically based on the selected **TYPE** (`OAUTH2`, `CLOUD_PROVIDER_TOKEN`, `PASSWORD`, `GENERIC_STRING`, or `SYMMETRIC_KEY`)
  - **OAuth Support** — supports both **Client Credentials** (with `OAUTH_SCOPES`) and **Authorization Code Grant** (with `OAUTH_REFRESH_TOKEN` and expiry) flows
  - **Integration Picker** — `API_AUTHENTICATION` field is populated from a live list of security integrations
  - **Masked Inputs** — passwords, tokens, and secret strings are masked in the UI for security
  - **Modify Support** — generates correct `ALTER SECRET` statements with `SET` clauses; clearing the comment field generates an `UNSET COMMENT` statement
  - **Live SQL Preview** — see the full `CREATE SECRET` or `ALTER SECRET` statement update in real-time as you modify the form
  - **Execution** — runs `ExecDDL` and refreshes the schema tree automatically on success
- **Stage Management** — right-click any schema and choose **Create Object** → **Stage…** to open the comprehensive stage designer:
  - **Internal & External** — support for creating both internal (Snowflake-managed) and external (S3, GCS, Azure) stages
  - **External Configuration** — specify URL and Storage Integration (selected from a live account-wide list)
  - **Encryption** — configure encryption types (Snowflake Full, SSE, AWS SSE-KMS, etc.) and optional KMS key IDs
  - **Directory Settings** — toggle directory tables and configure auto-refresh for external stages
  - **File Format Builder Integration** — choose between **Named format** or **Inline** (manual configuration); inline mode uses the visual File Format Builder form with support for:
    - **Data Preview** — test your configuration against local files or Snowflake stage files before creating the stage
    - **✨ AI Suggest** — automatically infer format options from local file content
    - Fully gated by the **File Format Builder** feature flag
  - **Live SQL Preview** — the full `CREATE STAGE` statement updates in real-time as you modify the form
  - **Execution** — runs `ExecDDL` and refreshes the schema tree automatically on success
- **Stage Sidebar Tree** — expand any stage in the sidebar to browse its contents hierarchically (directories and files), with lazy-loading on expand; right-click `.sql` files for **Execute File** (`EXECUTE IMMEDIATE FROM @stage/path`), all files for **Download…** and **Delete…**; right-click directories for **Refresh** and **Upload File…**; gated by `getCommand`/`putCommand`/`removeCommand` feature flags as appropriate
- **Stage File Browser** — right-click any stage and choose **Manage Storage Files…** to open a virtualised TanStack Table grid view of the stage contents:
    - **LIST view** — displays name, size, MD5, and last modified timestamp for all files in the stage
    - **Regex filtering** — a search bar allows filtering files using the Snowflake `PATTERN` parameter
    - **Bulk operations** — select multiple files to **Download** to a local directory or **Delete** from the stage in one go
    - **Native dialogs** — uses native OS folder pickers for selecting download targets
    - **Context menu** — right-click any file row in the grid for quick access to Download and Delete actions
  - **Quick Upload** — right-click any stage and choose **Upload File to Stage…** to immediately pick a local file and execute a `PUT` command (internal stages only)
- **Snowpipe Management** — right-click any schema and choose **Create Object** → **Pipe…**, or right-click an existing pipe to access pipe-specific operations:
  - **Create** — dynamic form with name, OR REPLACE / IF NOT EXISTS options, Auto Ingest toggle, Error Integration, AWS SNS Topic, Integration, Comment, and an embedded Monaco editor for the `COPY INTO` statement; live SQL preview shows the full `CREATE PIPE` statement
  - **Properties** — right-click a pipe and choose **Properties…** to view `SHOW PIPES` metadata plus inline-editable **Pipe Execution Paused** (toggle), **Comment** (inline edit / UNSET), and **Tags** (add/remove key-value pairs); changes are applied immediately via `ALTER PIPE … SET / UNSET`
  - **Refresh** — right-click a pipe and choose **Refresh…** to open a dialog with optional Prefix and Modified After parameters; live SQL preview shows the generated `ALTER PIPE … REFRESH` command; click **Refresh** to execute
  - **Copy History** — right-click a pipe and choose **Copy History…** to open a TanStack Table grid view backed by `INFORMATION_SCHEMA.COPY_HISTORY`; defaults to the last 24 hours; filterable by Start Time, Status, and File Name substring; sortable by `LAST_LOAD_TIME DESC`
  - **Drop** — right-click a pipe and choose **Delete…** for a standard danger-confirmation dialog; executes `DROP PIPE IF EXISTS` and removes the node from the tree
- **Git Repository Management** — right-click any schema and choose **Create Object** → **Git Repository…**, or right-click an existing git repository and choose **Fetch** or **Modify…**:
  - **Create** — specify the origin URL, API integration (required), optional git credentials secret (selected from a live account-wide secret list), optional comment and tags; live SQL preview shows the full `CREATE GIT REPOSITORY` statement
  - **Fetch** — right-click a git repository and choose **Fetch** to run `ALTER GIT REPOSITORY … FETCH`; displays a loading toast during the operation and a success or error message on completion
  - **Modify** — pre-fills current API integration, git credentials, and comment from `DESCRIBE GIT REPOSITORY`; generates correct `ALTER GIT REPOSITORY … SET` and `UNSET` statements (credentials and comment can be cleared via UNSET; API_INTEGRATION can only be SET); live SQL preview; multi-statement execution
  - **Properties** — right-click and choose **Properties** to view `DESCRIBE GIT REPOSITORY` output in the properties panel
- **DBT Project Browser** — right-click any schema and choose **Create Object** → **DBT Project…**, or right-click an existing DBT PROJECT object for full lifecycle management:
  - **Sidebar Tree** — expand any DBT PROJECT in the sidebar to browse its versions; each version expands into a hierarchical file/directory tree with lazy-loading; right-click versions/directories for **Refresh**
  - **Create** — specify source location (required), optional dbt version, default target, external access integrations, and comment; live SQL preview shows the full `CREATE DBT PROJECT` statement; supports OR REPLACE, IF NOT EXISTS, and case-sensitive naming; **Source Location Picker** lets you browse available git repositories, internal stages, existing dbt projects, and workspaces visually — select a source type, pick a database and schema (or browse any schema in the account), choose an object, select a branch/tag or version, and browse directories in a tree; the assembled location string is generated automatically
  - **Execute** — choose between Direct and From Workspace execution modes; specify dbt CLI args, optional dbt version override, workspace name, and project root; live SQL preview; results stream to a new query tab
  - **Modify** — pre-fills current dbt version, default target, integrations, and comment from `DESCRIBE DBT PROJECT`; generates correct `ALTER DBT PROJECT … SET` and `UNSET` statements; live SQL preview; multi-statement execution
  - **Add Version** — add a version alias and source location via `ALTER DBT PROJECT … ADD VERSION`; live SQL preview; includes Source Location Picker in stage-only mode (git repositories and internal stages)
  - **Show Versions** — runs `SHOW VERSIONS IN DBT PROJECT` in a new tab
  - **Describe** — runs `DESCRIBE DBT PROJECT` in a new tab
  - **Properties** — right-click and choose **Properties** to view `SHOW DBT PROJECTS LIKE` output in the properties panel
- **Search** — filter objects by name across all databases and schemas in real time; for previously expanded schemas all object types are searched instantly (no network call); for schemas not yet expanded, a fast path returns tables, views, and sequences only — extended types (procedures, tasks, stages, etc.) appear after manually expanding the schema
- **Right-click procedures** to open a parameter dialog; clicking **Execute** generates the `CALL` statement, opens a new tab, and runs it immediately — no manual Run press needed
- **Right-click functions** (**Call Function…**) to open a parameter dialog; detects scalar vs. table functions and generates the correct SQL; clicking **Execute** opens a new tab and runs it immediately
- **View Dependencies…** (views, procedures, functions) — right-click any view, procedure, or function and choose **View Dependencies…** to open a recursive dependency tree built by parsing DDL:
  - Every referenced object (tables, views, procedures, functions) appears as a node with its kind icon, colour-coded type tag, and fully-qualified name
  - The tree is recursive — each SQL-language object's own dependencies are expanded as children, up to 8 levels deep
  - **Circular reference detection** — objects that have already appeared higher in the tree are marked with an "already shown" badge and shown as leaf nodes to prevent infinite expansion
  - **Hover for DDL** — hovering any node shows its DDL definition in a tooltip; content is fetched lazily on first hover and cached for 60 seconds
  - Tables and non-SQL objects (non-SQL procedures, external functions) are shown as leaf nodes
  - The tree is fully expanded on load; nodes can be collapsed and re-expanded manually
  - **Multi-CTE correctness** — all CTE aliases in a `WITH` clause (first and subsequent `cte_name AS (...)` entries) are correctly excluded from the dependency list; only real table/view references produce dependency nodes
- **Right-click tables and views** to:
  - Select the top 1,000 rows — opens a new tab and executes immediately
  - **Time Travel Query** — drag a timeline slider to query data at any past point within the retention window
  - **Export Data** — download table data as CSV, JSON, or Parquet via a temporary Snowflake stage
  - **Import Data** — upload one or more local files into Snowflake; supports CSV, JSON, AVRO, ORC, and Parquet; offers a **Format source** toggle to choose between **Named format** (using an existing Snowflake FILE FORMAT object) or **Inline** (manual configuration); inline mode uses the visual File Format Builder form with all Snowflake `FORMAT_TYPE_OPTIONS` pre-filled with sensible defaults; can create a new table automatically by inferring the schema; **file preview** for CSV and JSON shows the first 10 rows of up to 5 files — CSV offers **Parsed** (table respecting current delimiter and header settings) and **Raw** views; JSON offers **Parsed** (tabular, supports arrays-of-objects and NDJSON) and **Raw** views; multiple files shown in a tabbed layout; **✨ AI Suggest** button (CSV and JSON, requires AI configured) — clicking shows a confirmation dialog disclosing that up to 64 KB of file content will be sent to the configured AI provider and warning against use with sensitive data; on confirmation, format options (delimiter, header detection, quoting, encoding, compression, etc.) are auto-filled and a one-sentence AI explanation is shown; a ⓘ icon next to the button also surfaces the data-sharing notice on hover
  - **Insert Mapping** — select a table as an **Insert Target** and one or more tables or views as **Insert Sources** to open a column mapping dialog:
    - **One View / Side-by-Side** — all source tables are displayed side-by-side in a single table, allowing you to map multiple sources to a single target simultaneously
    - **Combine Modes** — choose between **UNION ALL** (each source provides its own set of rows) or **UNION** (duplicates are removed)
    - **Auto-mapping** — automatically matches columns by name (case-insensitive) across all sources
    - **Heuristic matches** — makes smart guesses based on table names and data types
    - **Type conversion** — detects data type mismatches and offers an **Add CAST** button to automatically wrap the source value in a `CAST` expression
    - **Nullability validation** — warns when a nullable source column is mapped to a `NOT NULL` target column; provides an **Add COALESCE** button to inject a `COALESCE` with a type-appropriate default value (e.g. `0`, `FALSE`, `''`, or `CURRENT_TIMESTAMP()`)
    - **Constant values** — map target columns to `NULL` or custom constant literals
    - **Formatted SQL output** — the generated `INSERT INTO … SELECT` statement places each column and each source expression on its own indented line for readability
    - **Quote identifiers toggle** — on by default; when switched off, double-quotes are omitted for identifiers that are structurally safe
    - **Generate SQL** — clicking **Generate SQL** opens a new tab with the complete statement for review and execution
  - **Insert Full Name** — insert the fully-qualified `"DB"."SCHEMA"."OBJECT"` identifier at the cursor
  - View DDL definition inline
  - **Rename** the object
  - **Drop** the object (with confirmation)
  - **Select for Comparison** / **Compare with** — side-by-side DDL diff (see [Text Comparison](#text-comparison))
- **Create Database** — click the **+** button in the Objects section header, or right-click any database node and choose **Create Database…**, to open a dialog covering the full `CREATE DATABASE` syntax:
  - **Name & case** — free-text name input with a **Case-insensitive / Case-sensitive** radio toggle; case-insensitive emits an unquoted identifier (Snowflake uppercases it), case-sensitive wraps the name in double quotes to preserve exact casing; the insensitive option is automatically forced off and greyed out when the name contains characters that require quoting (spaces, special characters, lowercase letters, or a leading digit)
  - **Create options** — `OR REPLACE`, `TRANSIENT`, and `IF NOT EXISTS` checkboxes; `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive
  - **Clone** — clone from any existing database; the AT / BEFORE time-travel slider is automatically bounded by the source database's live `DATA_RETENTION_TIME_IN_DAYS` so you can never select a timestamp beyond what Snowflake can serve; three time-travel modes: TIMESTAMP (interactive slider with start/end date marks), OFFSET (signed integer seconds), and STATEMENT (query ID); `IGNORE TABLES WITH INSUFFICIENT DATA RETENTION` and `IGNORE HYBRID TABLES` flags; a warning is shown when the source database has zero retention days (no time travel available)
  - **Data Retention** — `DATA_RETENTION_TIME_IN_DAYS` and `MAX_DATA_EXTENSION_TIME_IN_DAYS` with edition-dependent guidance
  - **Iceberg & External Storage** — `EXTERNAL_VOLUME` (dropdown from `SHOW EXTERNAL VOLUMES`), `CATALOG` (dropdown from catalog-type integrations), `ICEBERG_VERSION_DEFAULT`, `ENABLE_ICEBERG_MERGE_ON_READ`
  - **Storage Policy** — `REPLACE_INVALID_CHARACTERS`, `DEFAULT_DDL_COLLATION`, `STORAGE_SERIALIZATION_POLICY`, `ENABLE_DATA_COMPACTION`
  - **Catalog Sync** — `CATALOG_SYNC` integration picker, `CATALOG_SYNC_NAMESPACE_MODE` (NEST / FLATTEN), and a delimiter field (shown only when mode is FLATTEN)
  - **Tags** — dynamic `name = value` tag list; add or remove rows freely
  - **Visibility & Comment** — `OBJECT_VISIBILITY` (not set / `PRIVILEGED` / custom YAML block) and a free-text comment
  - **SQL preview** — live `CREATE DATABASE` statement updates with every field change; copy button sends the SQL to the clipboard
  - Submitting runs `ExecDDL` and the object browser refreshes automatically on success
- **Create Table** — right-click any schema and choose **Create Object** → **Table…** to open a comprehensive table designer:
  - **Name & Type** — specify the table name and choose from **Permanent**, **Transient**, **Temporary**, or **Volatile** table types
  - **Create Options** — toggle `OR REPLACE` and `IF NOT EXISTS` modifiers
  - **Column Editor** — dynamic list of columns:
    - Set name and choose from a searchable list of Snowflake data types
    - Toggle **Primary Key** and **Not Null** constraints per column
    - Set **Default Values** and **Comments** for each column
    - Add, remove, and reorder columns easily
  - **Table Options** — configure advanced Snowflake table properties:
    - **Cluster By** — define one or more clustering keys or expressions
    - **Data Retention** — set `DATA_RETENTION_TIME_IN_DAYS` (0–90)
    - **Max Data Extension** — set `MAX_DATA_EXTENSION_TIME_IN_DAYS`
    - **Change Tracking** and **Schema Evolution** toggles
    - Table-level comment
  - **Live SQL Preview** — the full `CREATE TABLE` statement updates in real-time as you modify the form
  - **Execution** — runs `ExecDDL` and refreshes the schema tree automatically on success
- **Visual File Format Builder & Previewer** — right-click any schema and choose **Create Object** → **File Format…**, or right-click the **FILE FORMATS** folder to open the designer:
  - **Dynamic Form** — configuration fields adapt to the selected format type (CSV, JSON, AVRO, ORC, PARQUET, XML)
  - **Comprehensive Options** — covers all Snowflake `FORMAT_TYPE_OPTIONS` including record/field delimiters, headers, skip blank lines, encoding, compression, and more
  - **Data Preview** — test your configuration against real data before creating the format:
    - **Local File Preview** — select a local CSV or JSON file and see how it parses with current settings (up to 50 rows)
    - **Stage File Preview** — enter a Snowflake stage path (e.g. `@MY_STAGE/data.csv`) to preview any supported file type using Snowflake's compute engine
  - **Live SQL Preview** — see the full `CREATE FILE FORMAT` statement update in real-time as you modify the form; only parameters that differ from Snowflake defaults are included, keeping the DDL concise
  - **Execution** — runs `ExecDDL` and refreshes the schema tree automatically on success
- **Right-click a database** to **Create Database…**, export its DDL, generate an ER Diagram, view dropped schemas recoverable via Time Travel, or open **Backup Sets…**
- **Right-click a schema** to view dropped tables, **Export Data…** or **Import Data…** without needing an existing table (schema-level launch opens the same modals with a table selector or name field), open **Backup Sets…**, or use the **Create Object** cascading submenu (opens left or right depending on available screen space); contains **Task…** to open the Create Task dialog
- **Task tree** — tasks inside a schema are displayed as a hierarchy in the sidebar: child tasks appear nested under their predecessor root task; finalizer tasks are shown as the last child of their root task with a purple **Finalizer** badge so the graph structure is visible at a glance without opening the graph modal
- **Right-click the Tasks folder** inside any schema to open **Create Task…** directly — the dialog covers the full `CREATE TASK` syntax:
  - **Create options**: `OR REPLACE` / `IF NOT EXISTS` checkboxes (mutually exclusive)
  - **Compute**: warehouse dropdown or serverless with initial size and optional min/max statement size selects
  - **Schedule**: visual editor — **None**, **Interval** (validated number + unit: seconds `10–691,200`, minutes `1–11,520`, hours `1–192`; out-of-range values highlighted red), or **Cron** (5-field expression + searchable timezone dropdown, ~440 Snowflake-supported timezones)
  - **Configuration**: `CONFIG` JSON string (dollar-quoted in the generated SQL)
  - **Dependencies**: predecessor task picker — type to search tasks in the current schema, hit **+** to add each one as a removable tag; already-added tasks are hidden from the dropdown; the preview emits fully-qualified `"db"."schema"."task"` references; **WHEN condition** — visual boolean expression builder with `SYSTEM$STREAM_HAS_DATA` (stream selector), `SYSTEM$GET_PREDECESSOR_RETURN_VALUE` (task selector, optional cast to BOOLEAN/FLOAT/STRING, comparison operator + value), and custom SQL condition rows; combine with AND/OR; negate with NOT; Visual/Raw SQL toggle; live WHEN preview below the builder
  - **Execution**: overlap policy (`NO_OVERLAP` / `ALLOW_CHILD_OVERLAP` / `ALLOW_ALL_OVERLAP`), execute as (Default / Caller / User), timeout, suspend-after-failures, auto-retry, minimum trigger interval, target completion interval
  - **Notifications**: error and success notification integration dropdowns (populated from `SHOW NOTIFICATION INTEGRATIONS`)
  - **Other**: log level (TRACE…OFF), comment; **finalize task** — AutoComplete dropdown listing only standalone tasks (no predecessors, not referenced as predecessor by any other task); disabled with a tooltip when the current task has child tasks
  - **SQL body** (`AS`) with live `CREATE TASK` preview; a yellow warning alert appears when the task has no trigger defined (no SCHEDULE, AFTER, FINALIZE, or WHEN)
- **Right-click a task** to:
  - **Execute Task…** — opens a dialog with two modes:
    - **Execute** — issues `EXECUTE TASK <name>` immediately; accepts an optional CONFIG JSON override (`USING CONFIG = $json$`); validates JSON on the fly and blocks execution while the input is invalid
    - **Retry Last** — issues `EXECUTE TASK <name> RETRY LAST` to resume the last failed or cancelled task graph run from the point of failure (requires the run to be `FAILED` or `CANCELED`, the graph to be unchanged, and the original attempt to be within 14 days)
    - A live SQL preview shows the exact statement before it is sent
  - **View Task Graph…** — opens an interactive DAG visualisation of the complete task graph rooted at the selected task:
    - Left-to-right layout computed automatically via Dagre; each node shows the task name, schedule state badge (STARTED / SUSPENDED), last-run state badge (Running, Succeeded, Failed, Skipped, Scheduled, Cancelled, Waiting…), and — for completed or failed runs — a completion timestamp (HH:MM:SS for today, "Jan 15 HH:MM" for earlier dates)
    - **Real-time status** — polls Snowflake every 3 seconds and updates all node states in place without re-running the layout or losing drag positions; a pulsing green **Live** indicator and last-updated timestamp are shown in the top-right of the canvas
    - **Skipped inference** — tasks with no `TASK_HISTORY` row for the current run (because a predecessor failed before they could be scheduled) are automatically shown as Skipped; transitive chains are resolved so every downstream dependent also shows Skipped; a stale Succeeded row from a previous run is correctly overridden when the predecessor's failure is more recent (timestamp guard prevents false overrides when the predecessor was fixed in a later run); timestamps are suppressed on Skipped nodes since the stored time would be from the task's last actual run, not the current skipped run
    - **Run Graph** button — calls `EXECUTE TASK <root>` immediately to start the whole graph; all child nodes switch to "Waiting…" optimistically the moment the call returns so stale last-run states no longer show; the next poll tick replaces them with real states
    - **Retry Failed** button — calls `EXECUTE TASK <root> RETRY LAST`; enabled only when the last graph run has at least one FAILED task AND the first attempt was within the last 14 days (mirrors Snowflake's eligibility conditions for `RETRY LAST`); disabled with a descriptive tooltip when conditions are not met (e.g. "Last graph run did not fail or get cancelled" / "Last failed run was more than 14 days ago"); root task's own run state is not required to be FAILED — a child task failure is sufficient
    - **Suspend All / Resume All** — suspends or resumes every task in the graph (root, all descendants, and any finalizer) in a single click; suspend order is root-first so no new runs are scheduled during the process; resume order is leaves-first so each task's predecessors are STARTED before it is resumed, with the finalizer resumed before the root to satisfy Snowflake's requirement that the root be suspended during graph modifications
    - **Export DDL** button — opens a dialog to export the entire graph's DDL as an ordered SQL script (topological order: root first, children in BFS order, finalizer last) to the clipboard; an "Include SUSPEND/RESUME statements" checkbox wraps the script with `ALTER TASK … SUSPEND` / `ALTER TASK … RESUME` in the correct dependency order (suspend root-first, resume leaves-first with finalizer before root)
    - **Finalizer task display** — a task created with `FINALIZE = <root>` appears with a dashed purple border, a purple "Finalizer" badge alongside the STARTED/SUSPENDED schedule state badge, and a dashed purple **finalizes** edge from the root node; the node is placed at the far right of the Dagre layout, after all leaf tasks; finalize relationship is detected via `GET_DDL('TASK', ...)` as a reliable fallback when the `task_relations` SHOW TASKS column is absent or in an unexpected format
    - **Right-click any node** for a context menu:
      - **Suspend / Resume** — issues `ALTER TASK IF EXISTS … SUSPEND/RESUME`; shows the applicable action based on the task's current state (STARTED → Suspend, SUSPENDED → Resume); schedule state badge updates immediately without waiting for the next poll
      - **Add Child Task…** — opens the Create Task dialog pre-configured for child mode (SCHEDULE field replaced by an info note, AFTER pre-filled with the right-clicked task name, FINALIZE field hidden); disabled on finalizer nodes
      - **Create Finalizer Task…** — opens Create Task dialog pre-configured for finalizer mode (SCHEDULE and AFTER fields replaced by info notes, FINALIZE pre-filled with the root task fully-qualified name); enabled only when right-clicking the root node and no finalizer task already exists; label reads "(already has one)" when the root already has a finalizer; reads "(root only)" on non-root nodes
      - **Delete Task…** — drops the right-clicked task individually; suspends it first if it is currently STARTED; after a confirmation dialog runs `DROP TASK IF EXISTS`; disabled (with a clarifying label) for tasks that still have child tasks — remove dependents first or use **Delete All**; if the deleted task was the only task in the graph the graph modal closes automatically
      - **Export DDL** — copies the single task's DDL to the clipboard
  - **View Run History…** — opens a modal showing the chronological execution history for the selected task, queried from `INFORMATION_SCHEMA.TASK_HISTORY()`; toggle between "Last 24 Hours" and "Last 7 Days" scopes; optional auto-refresh (10s interval); summary chips show succeeded, failed, executing, and skipped counts; root tasks display grouped DAG Runs by `SCHEDULED_TIME` with expandable child task details; child tasks show a flat execution list with status, start time, duration, and error message; also accessible from the **History** button in the Task Properties status bar
  - **Delete Task Graph…** — right-clicking any non-finalizer task shows a **Delete Task Graph…** option; after a danger confirmation it calls `DropTaskTree` which suspends the graph and drops all tasks leaf-first (children before parent) so no dependency errors occur; the sidebar tree refreshes automatically on success; finalizer tasks are excluded from this option since they are not graph roots and can be removed via the regular **Delete…** item
  - **Properties** — opens a dedicated editable modal covering the full `ALTER TASK` syntax:
    - **Owner** — shown above the status bar when set
    - **Status**: RESUME / SUSPEND for the individual task; **Resume Graph** / **Suspend Graph** buttons operate on the entire graph — suspend order is root-first then all descendants and finalizer task(s); resume order is leaves-first then the root (finalizer before root); Resume buttons are disabled when the task has no trigger configured (finalizer tasks are always treated as having a trigger since `FINALIZE` is their trigger); **Create Finalizer Task…** button appears for root tasks (no predecessors, not itself a finalizer) and opens the Create Task dialog pre-configured for finalizer mode with FINALIZE pre-filled; the button is disabled with a tooltip when the graph already has a finalizer
    - **Compute**: warehouse (select from available warehouses)
    - **Schedule**: inline visual schedule editor (None/Interval/Cron with validated interval ranges and searchable timezone dropdown; UNSET supported)
    - **Dependencies**: list of predecessor tasks; add with `ADD AFTER` or remove per row with `REMOVE AFTER`; **Set as Finalizer For** — assigns the current task as a finalizer for a chosen root task; shows a searchable dropdown of root tasks that do not yet have a finalizer; greyed out with a reason when the task is ineligible (has predecessors, has its own schedule, has child tasks, or is already a finalizer); selecting a root task and clicking **Set** issues `ALTER TASK … SET FINALIZE = "db"."schema"."root_task"` using a fully-qualified identifier
    - **Condition**: WHEN expression — visual boolean expression builder (`STREAM_HAS_DATA`, `GET_PREDECESSOR_RETURN_VALUE`, custom SQL condition rows; Visual/Raw SQL toggle; Save / Cancel / Remove WHEN)
    - **SQL Body**: task SQL (multi-line editor with Save / Cancel via `MODIFY AS`)
    - **Configuration**: CONFIG JSON string (inline edit, UNSET supported)
    - **Limits**: user task timeout (ms) and overlap policy (ALLOW / DISALLOW)
    - **Notifications**: ERROR_INTEGRATION and SUCCESS_INTEGRATION selected from dropdowns of available notification integrations (UNSET supported)
    - **General**: comment (inline edit, UNSET) and EXECUTE AS (caller / user)
    - Every change is applied immediately via `ALTER TASK IF EXISTS … <clause>` and values reload after each save
- **Right-click a notebook** to:
  - **Open Notebook** — pulls the latest version from Snowflake using `DESC NOTEBOOK` and `GET`, then opens it in a new unsaved notebook tab
  - **Execute Notebook…** — opens a dialog to run `EXECUTE NOTEBOOK` with optional string parameters (each value is automatically single-quoted); the dialog shows the notebook's current Query Warehouse fetched from `SHOW NOTEBOOKS`; if none is set a warning alert offers a **Set Warehouse** button that opens a separate dialog with a warehouse selector and explicit **Save** / **Cancel** buttons (saves via `ALTER NOTEBOOK … SET QUERY_WAREHOUSE`); the execute dialog updates live once the warehouse is saved; a live SQL preview shows the exact statement that will run
- **Right-click a table** to open **Backup Sets…** (shows backup sets scoped to its schema)
- **Drag and drop** — drag any table or view into the editor to insert a `SELECT` statement with all column names listed individually
- **Column management** — right-click any column under a **table** node to (all DDL is generated by the backend `internal/column` package, unit-tested, and never built in the frontend):
  - **Insert Column Name** — inserts the quoted column name at the current editor cursor position (also available for view columns)
  - **Rename Column…** — opens a rename dialog with case-sensitivity control; executes `ALTER TABLE … RENAME COLUMN`
  - **Change Data Type…** — opens a dialog pre-filled with the current type; executes `ALTER TABLE … ALTER COLUMN … SET DATA TYPE`
  - **Set Comment…** — opens a comment dialog prefilled with the column's current comment; executes `ALTER TABLE … ALTER COLUMN … COMMENT` (or `UNSET COMMENT` when cleared)
  - **Set NOT NULL** / **Drop NOT NULL** — shown contextually based on the column's current nullability; executes `ALTER TABLE … ALTER COLUMN … SET/DROP NOT NULL` (hidden for primary key columns)
  - **Drop Column…** — with a confirmation dialog; executes `ALTER TABLE … DROP COLUMN`
  - Right-clicking a **view** column shows only **Insert Column Name** (view columns cannot be altered)
  - All altering actions (Rename, Change Data Type, Set Comment, Set/Drop NOT NULL, Drop Column, and **Add Column…**) are gated behind the **Column Management** feature flag (**View → Enabled Features → Column Management**) for IT-admin policy control; **Insert Column Name** is never gated
- **Add Column…** — right-click any table node to add a new column via a dedicated dialog with column name (case-sensitivity control), data type (searchable dropdown), value mode (none/default/autoincrement/computed), inline constraints (NOT NULL, UNIQUE, PRIMARY KEY, FOREIGN KEY with cascading reference selectors), collation (the selectable list is sourced from the backend `internal/snowflake` collation registry), comment, and a live backend-generated SQL preview. Submission is gated for invalid combinations (e.g. a default value is required in *Default* mode, AUTOINCREMENT requires a numeric type, a foreign key requires a referenced table); constraints and collation are hidden for computed (virtual) columns
- **Column type icons** — when expanding a table or view's column list, each column is prefixed with a type-family icon (text, number, datetime, boolean, variant/array, binary, geo, vector) coloured per the theme's column palette; primary-key and foreign-key columns get a distinct key icon
- **Empty table indicator** — table names with zero rows appear in a faded colour so unpopulated tables are immediately visible in the tree
- **Hover tooltips** — hovering any object in the tree shows its DDL definition
- **View Definition** — opens the DDL in a modal with a Copy button
- **Properties** — opens a key/value panel of object metadata populated from the relevant `SHOW` command; a search bar at the top filters properties by name in real time; for tables the panel additionally provides two inline-editable sections:
  - **Table Settings** — view and edit cluster key, schema evolution, change tracking, data retention days, max data extension days, default DDL collation, and comment; booleans are toggled with a switch, numeric and text fields open an inline input with Save / Cancel; changes are applied immediately via `ALTER TABLE SET`
  - **Column Comments** — view and edit the comment on every column; each row shows the column name, its current comment (or a dash if empty), and a pencil icon to edit inline
  - For **tasks** the Properties entry opens the full Task Properties modal described above instead of the generic read-only panel
- **Refresh** — reload the full object tree with one click
- **Time Travel / Undrop** — list dropped databases, schemas, and tables within their retention window and restore them with a single click
- **ER Diagram** — generate an Entity Relationship Diagram for any database; filter by schema, zoom, pan, and copy the Mermaid source
- **Database Reports** — right-click a database node to access a **Reports** cascading menu:
  - **Table Summary** — provides a quick, high-level snapshot of the selected database's contents; displays a detailed list of all tables including:
    - **Name & Schema**
    - **Table Type** (BASE TABLE, TRANSIENT, TEMPORARY)
    - **Owner** role
    - **Row Count** and **Physical Size** (B, KB, MB, GB, TB)
    - **Time Travel Retention** (days)
    - **Created On** and **Last Altered** timestamps
    - **Comment** description
- **Visual ER Designer** — interactively design or modify tables: add columns, set data types, define primary and foreign keys, preview the live Mermaid diagram, then generate and apply the necessary `CREATE TABLE` / `ALTER TABLE` SQL in one step

---

## Text Comparison

Compare the DDL or content of any two database objects, files, roles, or warehouses side by side:

1. Right-click any object, file, role, or warehouse and choose **Select for Comparison**.
2. Right-click a second item (any category) and choose **Compare with: …** — the label of the first item is shown so you always know what you are comparing against.
3. A Monaco side-by-side diff view opens, showing additions and deletions highlighted inline.

- Works across categories — compare a table's DDL against a local `.sql` file, a role against a warehouse, etc.
- Both sides are fetched concurrently so the modal opens without delay.
- The diff editor respects the active light/dark theme and the configured editor font and size.
- Trailing whitespace is trimmed from both sides before diffing to avoid spurious empty-line differences.

---

## AI Features

### AI Inline Completions

Ghost-text SQL suggestions appear automatically as you type in the editor. Press `Tab` to accept. Powered by OpenAI or Google AI Studios.

### Model Validation

- **Model Validation** — when configuring AI, a live **model status indicator** appears next to the model selector: a green `● Model OK` confirms the model is reachable, while a red indicator shows the exact API error — so misconfigured model names are caught immediately rather than at runtime.
- **Query Profile** — click the graph icon in the results status bar (visible for successful runs) to see the execution profile for the query; shows Operator Statistics, Execution Time Breakdown, and Operator Attributes sourced from `GET_QUERY_OPERATOR_STATS`.
- **Query Log** — session-scoped log of all SQL queries Thaw sends to Snowflake (both user-initiated from the editor and internal queries like object listing and DDL fetching). Appears as a third result pane tab ("Query Log") alongside Results and Terminal. Useful for debugging and attaching to issue reports. Enable via **View → Enabled Features → Query Log** or **View → Query Log → Enable Query Log**. Supports source filtering (All/User/Internal), status filtering, text search, and one-click copy formatting.

### Configuration

Open **AI → Configure AI…** in the menu bar to set your provider, API key, and model. The API key is stored locally with restricted file permissions (`0600`) and never transmitted anywhere other than the selected AI provider.

---

## File Management

- **Open** (`⌘O` / `Ctrl+O`) — native OS file dialog filtered to `.sql`, `.yml`, `.yaml`, and `.py`; opens in the configured export directory by default; re-activates an existing tab if the file is already open; the editor automatically uses YAML or Python syntax highlighting based on the file extension
- **YAML intelligence** — dbt YAML files opened in the editor receive schema-driven autocompletions, hover documentation, and real-time validation (red squiggles) powered by the bundled dbt-jsonschema schemas — all schemas are embedded locally, no network requests at runtime; covers `dbt_project.yml`, `packages.yml`, `dependencies.yml`, `selectors.yml`, and all model/source/seed/snapshot/exposure YAML files; property names, allowed values, and inline documentation strings are surfaced as you type; non-dbt YAML files (`profiles.yml`, CI configs, etc.) are not falsely flagged with schema validation warnings
- **Save** (`⌘S` / `Ctrl+S`) — writes back to the file's original path
- **Save As…** (`⌘⇧S` / `Ctrl+Shift+S`) — native OS save dialog; promotes a scratch tab to a named file
- **New Tab** (`⌘T` / `Ctrl+T`) — opens a blank scratch tab
- **File Browser** — browse the working directory in the sidebar; click any file to open it; auto-refreshes after a DDL export; **file system watcher** monitors the working directory for external changes (files created, renamed, or deleted in the terminal, other editors, or via git) and incrementally refreshes only the affected directories — no manual reload needed; toggleable via **View → Enabled Features → File Watcher**; right-click any file or folder to access the context menu:
  - **Reveal in Finder** / **Show in Explorer** — opens the platform file manager and selects the file or folder
  - **Copy Path** — copies the full file path to the clipboard
  - **Duplicate** (files only) — creates a copy of the file in the same directory with a `_copy` suffix
  - **Rename…** — renames the file or folder inline in the tree (VS Code-style)
  - **Delete** — deletes the file or folder (with confirmation dialog); directories are removed recursively; paths are restricted to the export directory for safety
  - **New Folder…** (directories only) — creates a new subfolder
  - **New SQL File…** (directories only) — creates an empty `.sql` file
  - **Select for Comparison** / **Compare with** (files only) — DDL diff comparison workflow

---

## DDL Export

- Export DDL for every database (or a specific one) as individual files, one per object
- Fully qualified object names (`db.schema.object`) in every `CREATE` statement
- Shared / imported databases (e.g. `SNOWFLAKE_SAMPLE_DATA`) are automatically skipped
- Files are organised on disk by schema and object type (tables, views, functions, procedures, sequences, stages, streams, tasks, file formats, pipes)
- **Configurable export path format** — open **Tools → Export Path Format…** to define a custom file path template; supported placeholders: `{database}`, `{schema}`, `{object_type}`, `{object_name}`; leave blank to use the default `{database}/{schema}/{object_type}/{object_name}.sql`; a live preview shows an example path as you type; the template is persisted across sessions
- Parallel export — up to 16 databases fetched concurrently; each database uses a single `GET_DDL('DATABASE', name, true)` call for maximum throughput
- **Live progress bar** while the export runs
- **Cancel** — stop an in-progress export at any time
- Results summary shows file counts, skipped databases, and any errors; click the folder icon to **reveal the export directory** in the platform file manager

---

## Schema Migration

Open **Tools → Schema Migration…** to deploy local `.sql` DDL files to a Snowflake database. A 5-step wizard guides the process from source directory to live deployment.

### Step 1 — Configure
- Add one or more **source directory → target database** mappings using the mapping list:
  - Each row has a source directory (type or **Browse…**) and a **Target DB** dropdown (optional fallback)
  - The target database is used for objects in that directory that have no explicit `USE DATABASE` context in the SQL files
  - Click **Add Database** to add a row; click the delete button to remove one; at least one directory is required to scan
  - Multiple mappings let you migrate several databases in a single wizard run

### Step 2 — Scan
- Recursively reads every `.sql` file in all source directories
- Handles multi-statement files; tracks `USE DATABASE` / `USE SCHEMA` context; applies each mapping's fallback database for unqualified objects
- Merges and deduplicates objects across all sources by kind + name (last definition wins); shows a count breakdown by object type (TABLE: N, VIEW: N, …)

### Step 3 — Review
- **TanStack Table diff grid** with status tags:
  - **New** — object exists locally but not in Snowflake
  - **Changed** — DDL differs after normalisation (comments stripped, whitespace collapsed, uppercased, trailing `;` removed)
  - **Unchanged** — identical; hidden from selection by default
  - **Removed** — exists in Snowflake but not in the local source
- **Monaco DiffEditor** below the grid shows the local vs remote DDL for the selected row
- **Dependency auto-select** — selecting a VIEW or PROCEDURE automatically selects any referenced TABLE that is also "new" or "changed"; unchecking a TABLE that a selected VIEW or PROCEDURE depends on is blocked with an inline warning ("Required by: VIEW_NAME")

### Step 4 — Strategy & Protect
Choose how existing TABLE objects with data are handled, then optionally create safety snapshots before deploying.

#### Table Migration Strategies
Only applies to TABLE objects that already exist in Snowflake and have rows. Empty tables (`SHOW TABLES` reports 0 rows) always use a fast `DROP + CREATE` regardless of the selected strategy.

| Strategy | How it works |
|---|---|
| **Smart In-Place** *(default)* | Diffs local vs remote column definitions; issues `ALTER TABLE ADD COLUMN`, `DROP COLUMN`, and `ALTER COLUMN TYPE` — no data movement |
| **Blue/Green Swap** | Creates a temp table with the new schema, copies shared columns via `INSERT … SELECT`, atomically swaps with `ALTER TABLE … SWAP WITH`, drops the temp; non-shared columns are discarded |
| **View-Based Soft Cutover** | Renames the original table to `<name>_v1`, creates the new table, and creates a compatibility view `<name>_compat` that exposes the shared columns from the archived data |
| **Destructive Rebuild** | `DROP TABLE IF EXISTS` + `CREATE TABLE`; all existing data is permanently lost; a red warning banner is shown when this strategy is selected |

- **Open in SQL Editor** — generates a strategy-aware SQL script and opens it in a new editor tab for review and editing before running:
  - Smart In-Place → `ALTER TABLE ADD/DROP/ALTER COLUMN TYPE` statements
  - Blue/Green Swap → `CREATE TABLE tmp; INSERT … SELECT; ALTER TABLE SWAP WITH; DROP TABLE`
  - View-Based Soft Cutover → `ALTER TABLE RENAME TO _v1; CREATE TABLE; CREATE VIEW _compat AS SELECT …`
  - Destructive Rebuild → `DROP TABLE IF EXISTS; CREATE TABLE`

#### Safety Snapshots (optional, per target database)
- The snapshot section shows one block per unique target database involved in the selected objects
- **Create Backup Set** — `CREATE BACKUP SET FOR DATABASE <db>` targeting a chosen database / schema / name
- **Create Zero-Copy Clone** — `CREATE DATABASE <clone> CLONE <db>` for a point-in-time snapshot
- Each database's backup and clone settings are independent; databases with no snapshot options checked are skipped

### Step 5 — Deploy
- Objects execute in dependency order: DATABASE → SCHEMA → SEQUENCE → TABLE → FILE FORMAT → STAGE → VIEW → MATERIALIZED VIEW → FUNCTION → PROCEDURE → STREAM → TASK → PIPE
- Up to **5 retry passes** — objects that fail with a dependency error ("does not exist" / "not authorized") are automatically re-queued for the next pass; once a pass produces no progress the remaining objects are marked as failed
- **Live progress table** — pass number, object kind, fully-qualified name, and per-object status tag (running / success / failed / skipped) update in real time as events arrive
- **Cancel** — stops the deployment cleanly mid-run

---

## dbt Project Scaffolding

Open **Tools → Create dbt Project…** to scaffold a complete dbt project pre-wired to the active Snowflake connection — no dbt CLI required during generation.

### Step 1 — Configure
- Set the **project name** and **profile name** (mirrors the project name by default, independently editable once changed)
- Choose the **output directory** with a native directory picker or type a path directly
- Thaw warns when the target `<dir>/<name>` directory already exists to prevent accidental overwrites
- **Inline view SQL definitions** toggle (off by default) — when enabled, Thaw fetches the `GET_DDL` for each view in the selected schemas and embeds the actual `SELECT` body into the staging stub instead of a generic `{{ source() }}` pass-through; one extra `GET_DDL` call per view is made at generation time
- **Automatic reference rewriting** (active whenever inline view SQL is enabled) — after all schemas are fetched, Thaw scans every inlined view body for multi-part Snowflake identifiers and rewrites them to correct dbt Jinja calls:
  - Three-part references to **tables** in selected schemas → `{{ source('db_schema', 'TABLE') }}`
  - Three-part references to **views** in selected schemas → `{{ ref('stg_model_name') }}`
  - References to objects **outside** the selected schemas → left unchanged
  - CTE aliases are excluded to prevent false-positive replacements; single-part names are never replaced to avoid collisions with column aliases
- **Use dbt variables for database names** toggle (off by default) — when enabled, adds a `vars:` block to `dbt_project.yml` with one entry per selected database (e.g. `db_mydb: MYDB`, sorted alphabetically) and replaces hardcoded database names in `_sources.yml` with `{{ var('db_mydb', 'MYDB') }}` calls; the default value in the var preserves the original database name casing; retargeting the project at a different database then only requires overriding the relevant variable

### Step 2 — Select Sources
- Databases load lazily from the live Snowflake connection
- Expand any database to fetch and display its schemas as a checkbox list
- **Select all / Deselect all** link per database for quick selection
- `INFORMATION_SCHEMA` is shown with a warning icon and descriptive tooltip, excluded from **Select all**; when checked, it is added to `_sources.yml` as a system schema entry but no staging stubs or `ListObjects` calls are made — this matches dbt convention for referencing virtual Snowflake schemas
- **Cross-schema dependency hints** — checking a schema triggers a background analysis of all views in that schema (via `SHOW VIEWS IN SCHEMA`, which returns the full `CREATE VIEW` DDL); view bodies are scanned for `FROM` / `JOIN` references to other schemas; any referenced schema not yet selected is highlighted in the list with an amber indicator and a tooltip listing the selected schemas that reference it; "Select all" for a database triggers a single batched analysis of all schemas at once; analysis is non-blocking — the spinner shows "Analysing dependencies…" per schema while in flight and disappears silently when done; results are cached for the lifetime of the wizard
- At least one schema must be selected to proceed

### Step 3 — Generate
- Summary shows project path, number of databases and schemas selected, and estimated file count
- **Generate Project** creates all files on disk; a spinner shows "Creating project files…" while in flight
- **Success** — collapsible file list grouped by directory; a note below the list reminds you to copy `profiles.yml` to `~/.dbt/` before running dbt commands
- **Error** — red alert with message and a back button to return to Step 1

### Generated files

| File | Description |
|------|-------------|
| `dbt_project.yml` | Project config: name, profile reference, materialization defaults (staging → view, marts → table); optional `vars:` block when **Use dbt variables** is enabled |
| `profiles.yml` | Pre-filled from the live session: account, user, role, warehouse, database, schema |
| `models/staging/_sources.yml` | One `source:` entry per selected (database, schema) |
| `models/staging/stg_<table>.sql` | CTE stub per table/view (`with source as … renamed as … select * from renamed`) |
| `models/marts/.gitkeep` | Directory placeholder |
| `seeds/.gitkeep` | Directory placeholder |
| `macros/.gitkeep` | Directory placeholder |

When multiple databases or schemas are selected, stub filenames are prefixed with `db_schema_` (e.g. `stg_mydb_public_orders.sql`) to prevent collisions. Single-scope projects use the shorter `stg_<table>.sql` form.

---

## Git Integration

- **Embedded `go-git`** — all git operations run without a system `git` installation; no external binary dependency
- **Git Operations Dialog** — open via **Git → Git Operations…** (`⌘G` / `Ctrl+G`) or the "Git Operations…" button in the sidebar Git panel; provides four tabs:
  - **Commit & Push** — file checklist with checkboxes (added/modified/deleted colour-coded), select all / none / by extension, commit message textarea, personal access token input (ephemeral, never saved)
  - **Pull** — shows current remote URL and branch; PAT input; pull from configured remote branch
  - **Clone** — remote URL input, local path picker (native OS dialog), optional PAT (for private repos), clone progress feedback
  - **Branches** — lists all local and remote branches with the current branch highlighted; Switch button per local branch; **Merge branch** button to merge any local branch into the current one (Fast-Forward only); create new branch with name input and **Create Branch** button; refresh button to reload branch list
- **Git gutter indicators** — when a tracked file is open in the SQL editor, VS Code-style coloured bars appear in the gutter:
  - **Green bar** — lines added since the last HEAD commit
  - **Blue bar** — lines modified since the last HEAD commit
  - **Red chevron** — deletion point where lines were removed
  - Indicators update 400 ms after each keystroke; clear automatically when a scratch tab (no file path) is active
- View git status for the working directory (staged and unstaged files shown in the sidebar panel)
- Git credentials are **never saved to disk** — tokens are held in memory only for the duration of the operation
- OS junk files (`.DS_Store`, `Thumbs.db`, `desktop.ini`) are automatically excluded and added to `.gitignore`

---

## Administration

- View all roles, warehouses, users, and Snowflake integrations from the **Administration** panel in the sidebar

### Warehouse Credit Usage

Click the bar-chart icon in the Administration panel header (always visible, even before expanding) to open the **Warehouse Credit Usage** modal — backed by `SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY`:

- The button is only shown to users whose current role has `SELECT` access to `SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY`; a zero-row probe query runs on mount and hides the button automatically for roles without access
- **Warehouse** — select a specific warehouse or *All warehouses* to aggregate across the account
- **Date range** — defaults to the last 30 days; pick any custom range and click **Apply** to refresh
- **Summary cards** — total credits used, compute credits, and cloud services credits for the selected scope
- **Stacked bar chart** — toggle between **Daily** and **Hourly** granularity with the segmented control above the chart; Compute (blue) and Cloud Services (orange) are stacked so the credit split is immediately visible; X-axis labels are angled and thinned automatically so they remain legible at any date range; built with recharts inside a responsive container
- **Hourly detail table** — one row per metering record; columns: Start Time, Warehouse, Total Credits, Compute Credits, Cloud Svc Credits; paginated at 20 rows/page
- **Collapse / Expand table** — a toggle button in the table header hides the row detail while keeping the summary cards and chart visible

### Query Activity

Click the clock icon in the Administration panel header (always visible, even before expanding) to open the **Query Activity** modal:

- **Scope** — *Current Session*, *By User*, *By Warehouse*, or *All*
  - *By User* — autocomplete dropdown from `SHOW USERS`; accepts free-typed names for users that no longer exist
  - *By Warehouse* — autocomplete dropdown from the live warehouse list; accepts free-typed names for dropped or renamed warehouses
- **Time range** — optional date/time range picker to bound the history window
- **Limit** — cap results from 1 to 10 000 (default 100)
- **Include client-generated** — toggle to include Thaw's own internal statements
- **Run** — re-fetches with the current filters; auto-runs on open with current session scope
- **Query text search** — live filter bar narrows the loaded results by query text as you type; matches are highlighted in the table and in expanded rows; row count shows `N of M rows` when a filter is active
- Results table shows status (colour-coded), query type, query preview, start time, end time, and duration
- Expand any row to see the full SQL plus a detail grid with user, warehouse, database, schema, rows produced, bytes scanned, and query ID
- **Load in Editor** — inserts the query into the active editor tab and closes the modal
- **Copy** — copies the full query text to the clipboard with a brief "Copied!" confirmation

### Backup Policies

- List all backup policies with schedule, expiry, retention lock, owner, and comment
- **Create** — full `CREATE BACKUP POLICY` support: schedule, expire after days, tags, comment, `WITH RETENTION LOCK`, and `OR REPLACE` / `IF NOT EXISTS` modifiers
- **Alter** — rename, set/unset schedule, expiry, comment, and retention lock via an action dropdown
- **Drop** — with confirmation

### Integrations

Browse, create, modify, and drop all six Snowflake integration types from a lazy-loading tree in the Administration panel:

| Kind | Supported Subtypes / Providers |
|------|-------------------------------|
| **Storage** | Amazon S3, S3 GovCloud, Google Cloud Storage, Azure Blob Storage |
| **API** | AWS API Gateway, AWS Private API Gateway, Azure API Management, Google API Gateway, Git HTTPS API (Token/Secret, GitHub App, OAuth2, Private Link) |
| **Catalog** | AWS Glue, Object Store, Polaris, Iceberg REST, SAP BDC |
| **External Access** | Network-rule-based (allowed network rules + optional authentication secrets) |
| **Notification** | Email, Webhook, Azure Storage Queue (inbound), GCP Pub/Sub (inbound/outbound), AWS SNS (outbound), Azure Event Grid (outbound) |
| **Security** | API Authentication (AWS IAM / OAuth2), External OAuth, OAuth partner (Looker, Tableau, Power BI), OAuth custom, SAML2, SCIM |

- **Lazy loading** — each category's integrations are fetched from Snowflake only when the node is first expanded
- **Create** — right-click any category to open a structured form; fields change dynamically based on the selected integration type and subtype; cloud provider defaults (S3 / GCS / Azure for Storage; equivalent defaults for API) are pre-selected based on the current Snowflake region; the option is automatically disabled when the current role lacks `CREATE INTEGRATION`
- **Properties** — right-click any integration and choose **Properties** to see its `DESCRIBE INTEGRATION` output as a key/value table
- **Modify** — right-click and choose **Modify** to open a modal showing current DESCRIBE properties alongside an editable ALTER SQL textarea; click **Run** to execute the statement
- **Drop** — right-click and choose **Drop** with a Popconfirm confirmation; the category reloads automatically on success

### Backup Sets

Right-click any **database**, **schema**, or **table** in the object browser and choose **Backup Sets…**:

- **Object-scoped listing** — backup sets shown are those that actually back up the right-clicked object: `SHOW BACKUP SETS IN DATABASE <db>` is issued and the results are post-filtered by `object_kind`, `object_name`, `object_database_name`, and `object_schema_name` — so right-clicking a table returns only backup sets covering that exact table, not all backup sets stored in that database
- **Create** — `CREATE BACKUP SET FOR DATABASE|SCHEMA|TABLE <fqn>` with optional backup policy applied after creation:
  - Backup set name is fully qualified: choose the **database** and **schema** from dropdowns (defaulting to the source object's database and schema; `INFORMATION_SCHEMA` is excluded from the schema list), then enter just the name — the full `db.schema.name` is assembled automatically
- **Alter** — rename, set/unset comment, apply/suspend/resume backup policy
- **Drop** — with confirmation
- All operations (list, add, alter, drop, restore) reference backup sets by their fully-qualified name (`"db"."schema"."name"`) to avoid schema-resolution ambiguity
- The **Name** column in the backup sets list shows the full `db.schema.name` qualified name so the storage location is always visible at a glance
- **Delete oldest backup** — each backup set row has a **Delete oldest backup** button that identifies and removes the oldest backup without a legal hold via `ALTER BACKUP SET … DELETE BACKUP IDENTIFIER '<uuid>'`; the button is automatically greyed out when the set contains no backups; counts are pre-fetched in the background when the modal opens so no row expansion is needed
- **Expand any row** to see its individual backups:
  - Backup name, status, created date, size, and comment
  - **Add Backup** — runs `ALTER BACKUP SET … ADD BACKUP`, waits for completion, then refreshes the backup list automatically; the button shows a loading spinner while the operation is in progress to prevent accidental double-submission
  - **Restore** — create a new object from a backup snapshot:
    - Object type auto-detected from the backup set
    - Requires a new name (Snowflake does not allow restoring over an existing object)
    - For **TABLE** restores: choose the target **database** and **schema** from dropdowns (defaulting to the source object's location), then enter only the new table name
    - For **DATABASE** and **SCHEMA** restores: enter the new name directly
    - Executes `CREATE <type> <new_name> FROM BACKUP SET "<set>" IDENTIFIER '<uuid>'`

### Warehouse Properties

Right-click any warehouse in the Administration panel and choose **Properties** to open an editable properties modal:

- **Status** — current state badge (STARTED / SUSPENDED / RESUMING / QUIESCING) with type, size, and owner; action buttons:
  - **Suspend** / **Resume** — toggle warehouse state immediately
  - **Abort All Queries** — cancel all running queries (confirmation required)
  - **Rename** — inline name input; the sidebar warehouse list updates live
- **Compute** — warehouse size (X-Small → 6X-Large), warehouse type (Standard / Snowpark-Optimized); multi-cluster warehouses also expose max/min cluster count and scaling policy
- **Behavior** — auto-suspend seconds (0 = disabled), auto-resume toggle
- **Query Acceleration** — enable/disable, max scale factor (0–100)
- **Resource & Timeouts** — resource monitor, max concurrency level, statement queued timeout, statement timeout (from `SHOW PARAMETERS IN WAREHOUSE`)
- **General** — comment
- Each property saves immediately via `ALTER WAREHOUSE … SET` on confirm
- **Inline privilege errors** — `ALTER WAREHOUSE` failures (e.g. insufficient privileges) are shown inline below the field in red rather than silently discarded; toggle-switch errors appear as message toasts; rename errors appear below the name input; the "Insufficient privileges" phrase is extracted for a short, readable message

### User Management

- **User Management** — search users by name, login, display name, or email; view disabled accounts at a glance
- **Create User** — dialog with all user properties and a live `CREATE USER` SQL preview
- **Edit User** — pre-populated form that generates only the `ALTER USER … SET/UNSET` statements needed for the changed fields
- **Enable / Disable / Drop** users with a single right-click action
- All user management actions are automatically hidden or greyed out when the current role lacks the required privileges
- **Key Pair Authentication** — right-click any user and choose **Key Pair Auth…** to set up Snowflake key-pair authentication without leaving the app:
  - Choose a key generation method: **Go built-in crypto** (always available, no passphrase), **OpenSSL** (passphrase-encrypted private key), or **ssh-keygen** (passphrase-encrypted private key); only tools present on PATH are shown
  - Set the private key output path (type or browse); the public key is saved alongside with `_pub.pem` appended; the private key file is written with mode `0600`
  - Optionally enter a passphrase (disabled for Go built-in)
  - Click **Generate key pair** to produce an RSA-2048 PKCS#8 PEM key pair; the stripped public key content (no PEM header/footer) is shown for review
  - Click **Apply to \<username\>** to run `ALTER USER … SET RSA_PUBLIC_KEY='…'` immediately
  - The menu item is greyed out automatically when the current role lacks OWNERSHIP or MODIFY PROGRAMMATIC AUTHENTICATION METHODS on that user
- **Key pair auth in Create User** — the **Create User** dialog includes an **RSA public key** field and a **Generate key pair…** button; clicking the button opens the key pair generator in "pick" mode so you can generate a key pair and auto-fill the public key without leaving the create flow

---

## Results & Export

- Query results displayed in a virtualised grid — handles large result sets smoothly
- **NULL display** — `NULL` values are rendered as a faded italic `NULL` label so they are never confused with empty strings
- **Copy from results** — right-click any cell to open a context menu with: **Copy cell value**, **Copy row (tab-separated)**, and **Copy row with headers**; all three write to the native OS clipboard so they work reliably on macOS
- **Result history** — the last 10 successful result sets are kept in memory for the session; a dropdown in the results status bar (visible after two or more runs) lets you switch between them instantly, similar to `LAST_QUERY_ID(-n)` in SQL; after a query failure the error is shown and the dropdown appears as a standalone **Previous results** picker — the last result grid is not auto-displayed so the failure is immediately obvious, but any historical result can be recalled on demand; click the **pin** icon next to any entry in the dropdown to keep it indefinitely — pinned results are exempt from the 10-entry cap and always appear at the top of the list (click again to unpin); **right-click** any entry and choose **View side by side** to open it alongside the current result in a horizontally split view — each grid scrolls independently so you can compare different regions of two result sets; the compare panel's SQL snippet, query ID, and row count appear on a second line of the status bar (right-aligned for clarity); close the compare panel with the × button in the status bar
- **Export results** — CSV (RFC 4180) and Excel (`.xlsx`) export with a native save dialog; exports always reflect whichever result is currently selected in the history dropdown
- Column sorting and horizontal scrolling
- **Auto-Size Columns** — double-click a column resize handle to auto-fit the column width based on header text and data content (samples up to 500 rows)
- **Column Pinning** — right-click any column header and choose **Pin to Left** or **Pin to Right** to freeze it during horizontal scrolling; pinned columns render as `position: sticky` and are excluded from column virtualisation
- **Global Grid Search** — press `⌘G` (or click the search icon) to open a search bar above the results grid; matches are highlighted in-cell and navigable with Enter/Shift+Enter
- **Data Type Formatting** — right-click a column header → **Format Column…** to apply number, currency, percentage, or date/time formatting via `Intl` APIs; preview before applying
- **Conditional Formatting** — right-click a column header → **Conditional Formatting…** to add colour-scale, data-bar, or text-match highlight rules
- **Excel-Style Column Filtering** — right-click a column header → **Filter…** to open a dropdown with a unique-value checklist and conditional filter (contains, starts with, greater than, etc.)
- **Multi-Cell Copy & Selection** (feature-flagged) — click and drag to select a range of cells; `⌘C` copies the selection as TSV with headers to the native clipboard; a row-number gutter and select-all button appear when enabled
- **Selection Aggregations** — when a range is selected, a status bar below the grid shows Sum, Avg, Count, Min, Max of numeric values in the selection
- **Quick Charting** — with a range selected, right-click → **Create Chart…** to open a modal with bar, line, and scatter chart types powered by Recharts
- **Cell Detail Panel** (feature-flagged) — clicking a cell opens a resizable side panel on the right edge of the results area showing the column name, row number, and the full cell content in a scrollable, selectable text view; JSON values are pretty-printed with a Raw/Formatted toggle; a copy button writes the raw value to the native clipboard; close with the × button or `Esc` — selecting another cell reopens it (requires Multi-Cell Copy & Selection)


### Snowflake Connectivity

- Connect with account / user / password / warehouse / role
- **Snowflake CLI Profile Manager** — full CRUD management of Snowflake CLI profiles in `~/.snowflake/config.toml` (or a custom location) directly from the connection dialog:
  - **Auto-fill** — select a profile to populate the connection form; includes support for key-pair (`SNOWFLAKE_JWT`) profiles; the config file path can be changed during sign-in and is persisted as the new default location
  - **New** — create a new profile from the current form values; blocked if a profile with the same name already exists
  - **Save** — overwrite the selected profile with the current form values
  - **Rename** — rename the selected profile; blocked if the new name already exists; updates `default_connection_name` if it pointed to the old name
  - **Clone** — duplicate a profile under a new name; blocked if the new name already exists
  - **Set Default** — set any profile as the `default_connection_name`
  - **Delete** — remove a profile (with confirmation); if the deleted profile was the default, the default is cleared
  - Text-level TOML manipulation preserves user comments, blank lines, and unknown keys
- **Offline-first startup** — the app launches instantly without waiting for a Snowflake connection; connection parameters are validated and the session is established on demand when you first run a query or browse objects, rather than blocking the UI at launch.
- **Cancel connection** — abort an in-progress connection attempt
- **User Agreement** — a **User Agreement** link at the bottom of the connect screen opens the End User License Agreement in a scrollable modal
- **Unified Toolbar** — a reusable `<Toolbar />` component with execution controls (Run/Cancel), quick-action icon buttons (New SQL, New Notebook, Save), session selectors (role, warehouse, database, schema), and a context-specific slot that dynamically adapts based on the active tab type (e.g. notebook kernel status indicators and actions)
- **Switch role, warehouse, database, or schema** from the toolbar without disconnecting — all subsequent queries, privilege checks, and object browsing immediately reflect the new session state
- Role dropdown shows only roles the current user can actually assume
- Schema dropdown lists only schemas belonging to the currently selected database; the list resets automatically when the database is changed
- After any `USE DATABASE`, `USE SCHEMA`, `USE ROLE`, or `USE WAREHOUSE` command runs in the editor, all four toolbar dropdowns update automatically to reflect the resulting session state; the internal connection context is also fully synced so subsequent toolbar dropdown selections (e.g. choosing a schema from the dropdown after a `USE DATABASE` SQL command) always target the correct database — no stale-context errors
- **Current username** — the active Snowflake username (from `CURRENT_USER()`, preserving exact case) is displayed above the toolbar session selectors and above the account · user tag so the connected identity is always visible
- **Session state persisted across reloads** — the account · user tag and non-sensitive connection details survive a page reload; credentials (password, passcode, private key passphrase) are never written to storage; the connected state is verified against the backend on every reload so a backend restart correctly shows ConnectModal pre-filled with the last-used parameters rather than a broken UI; the UI waits for state hydration to complete before rendering, preventing a spurious ConnectModal flash on HMR page reloads
- **Session Properties** — right-click the account · user tag in the toolbar to open a **Session Properties** modal:
  - **Search** — a search bar at the top of the modal filters both Parameters and Variables in real time by name; clear the input to restore the full list
  - **Parameters** section — all rows from `SHOW PARAMETERS IN SESSION`; boolean parameters render as a toggle switch (saves immediately); all other parameters show a pencil button that opens an inline input with Save / Cancel; changes apply via `ALTER SESSION SET`; hovering the parameter name shows its Snowflake description in a styled tooltip
  - **Variables** section — all rows from `SHOW VARIABLES`; editing works identically; changes apply via `SET variable = value`
  - String-type values are automatically single-quoted in the generated SQL; booleans and numbers are passed raw
  - **Copy** button copies all parameters and variables to the clipboard
- **Session Management** — open **View → Advanced → Session Management…** to configure:
  - **Max concurrent sessions** (1–32) — LRU cap; excess idle sessions are evicted
  - **Max open connections per session** (1–16) — `database/sql` MaxOpenConns per tab pool
  - **Max idle connections per session** (1–16) — `database/sql` MaxIdleConns per tab pool
  - **Init mode** (lazy / eager) — whether sessions are created on first query or immediately when a tab opens
  - **Idle timeout** (0–480 minutes) — time-based eviction alongside LRU; 0 = LRU only
  - **Reset to Defaults** — restores all values to CPU-based defaults

---

## Embedded Terminal

An OS shell terminal is available as a tab in the results area alongside Results.

- **Open** via **Terminal → New Terminal** in the menu bar (`⌘ \`` / `Ctrl+\``)
- **Shell picker** — a dropdown lists all shells from `/etc/shells`; switching shells immediately restarts the session in the selected shell
- **New** button restarts the current shell; **Kill** stops it without closing the tab; **×** closes the tab and returns to the Results tab
- The terminal opens in the configured export directory so file operations run in context
- Resizes automatically when the results pane is resized
- Full ANSI colour and cursor support via xterm.js

---

## Snowpark & Jupyter Notebooks

Open the **Snowpark** menu to set up a local Python environment and run Jupyter-style notebooks directly inside Thaw.

### Environment setup

- **Check Environment** (`Snowpark → Check Environment…`) — scans the local machine and shows the status of system Python, the selected backend (conda env or venv), `snowflake-snowpark-python`, `notebook`, `ipython-sql`, and `sqlalchemy`; offers a direct shortcut to the setup wizard when anything is missing
- **Setup Environment** (`Snowpark → Setup Environment…`) — three-step guided wizard that streams command output line-by-line into a scrollable log:
  1. Create a conda environment (`thaw_snowpark`, Python 3.12, Snowflake channel) **or** a Python venv
  2. Install `snowflake-snowpark-python` (with optional `[pandas]` extras for venv)
  3. Install `notebook`, `ipython-sql`, and `sqlalchemy`
- **Backend choice** — radio group selects **conda** or **venv**; all commands adapt accordingly
- **Python interpreter selector** (venv only) — dropdown lists every Python interpreter found on the system (`/usr/bin`, Homebrew, pyenv, etc.); duplicates are removed by resolving symlinks; the selection is saved to `config.json`
- **Apple Silicon warning** (conda only) — `CONDA_SUBDIR=osx-64` is applied automatically on Apple M-series chips to work around a known `pyOpenSSL` incompatibility; a banner explains this
- **Use Existing venv** (venv only) — point the wizard at a pre-existing virtual environment (project-specific, shared team env, pyenv-managed, etc.) instead of creating a new one:
  - **Browse** button opens a native directory picker; the path can also be typed manually
  - **Use Existing / Re-validate** validates the selected directory via `CheckSnowparkEnv`, showing a checklist (venv present, `snowflake-snowpark-python`, `notebook`) with detected Python version
  - Steps that are already satisfied are auto-marked done; the wizard jumps to the first missing step (or straight to the package manager if everything is installed)
  - The Python interpreter selector is hidden in "use existing" mode (the venv already has its own Python)
  - Re-opening the modal with a partially configured venv auto-enters "use existing" mode
  - **Create New Instead** resets back to the standard create-from-scratch flow
- **Delete venv folder** — danger button (hidden in "use existing" mode) with a confirmation dialog removes the venv directory and resets all steps
- The project directory (same path used for DDL export and the terminal) is shown for reference
- **Manage Packages** — a 4th step in the setup wizard is always accessible (via the stepper or the "Manage Packages" footer button) regardless of whether the setup steps have been run in the current session:
  - **Install** — enter any package name and press Install or hit Enter; output streams line-by-line into a log panel; the package list refreshes automatically on success
  - **Uninstall** — all installed packages are listed with their versions; click Uninstall on any row (with confirmation) to remove it; the list refreshes after removal
  - Backed by `pip list --format=json` and `pip install` / `pip uninstall -y` inside the active conda or venv environment
- **Private Pip Registries** — configure corporate or private pip repositories (including credentials) in the Snowpark setup wizard; Thaw automatically injects them into all `pip install` commands.

### Notebook tabs

- **New Notebook** (`Snowpark → New Notebook…`) — native save dialog writes a blank `nbformat v4` file and opens it as a new notebook tab
- **Open Notebook** (`Snowpark → Open Notebook…`) — file picker filtered to `.ipynb`; opens alongside SQL tabs
- **Open from Snowflake** — right-click any notebook in the object browser and choose **Open Notebook**; the latest version is downloaded from Snowflake and opened as a new unsaved notebook tab
- Notebooks are saved as standard `.ipynb` files compatible with JupyterLab and VS Code

### Cell editor

- **Monaco editor per cell** with full syntax highlighting:
  - **Code cells** → Python syntax (keywords, builtins, decorators, strings, comments)
  - **SQL cells** → custom Snowflake SQL tokenizer (same as the main SQL editor)
  - **Markdown cells** → Markdown syntax highlighting
- Editor auto-sizes vertically to its content
- Native undo/redo (`⌘Z` / `⌘⇧Z`) and clipboard (`⌘C` / `⌘V` / `⌘X`) via Monaco and Wails native APIs
- `Shift+Enter` runs the current cell; cell kind (Code / SQL / Markdown) can be changed at any time
- **Multi-cell Debugger** — set breakpoints in Python cells and debug your Snowpark logic with a live variable explorer and call stack.

### Python code cells

- Cells share a **persistent Python kernel** subprocess per notebook tab — variables and imports carry across cells
- The kernel uses the `snowflake-snowpark-python` environment (conda or venv)
- Output shows stdout, stderr, and tracebacks in colour-coded blocks with a per-block copy button
- **Inline plots** — matplotlib figures (e.g. from `plt.show()`) are captured as PNG images after each cell run and rendered inline below the cell output; no separate window opens; the kernel automatically configures the `Agg` backend on startup; multiple figures per cell are each rendered in order
- **Auto-connected Snowpark session** — a Snowpark session is automatically created on kernel startup using the same account, role, warehouse, database, and schema as the active app connection; `get_active_session()` (from `snowflake.snowpark.context`) works in every Python cell with no `Session.builder` boilerplate — matching Snowflake's native notebook behaviour; session init errors (e.g. wrong credentials or missing private key) are surfaced in the first cell's stderr
- **Session kept in sync — bidirectional** — changing role, warehouse, database, or schema via the toolbar automatically applies the change to the kernel session via `get_active_session()`; switching to a notebook tab also triggers a sync; conversely, when a Python or SQL cell runs a `USE` command the change propagates back to the main Snowflake connection pool — all four toolbar dropdowns update automatically and subsequent queries in SQL editor tabs immediately reflect the new context; Python cells, SQL cells, and SQL editor tabs always see the same session state
- **DDL executes immediately** — `session.sql("USE DATABASE X")` takes effect without an explicit `.collect()` call, matching Snowflake native notebook behaviour; USE, CREATE, ALTER, DROP, TRUNCATE, COMMENT, GRANT, and REVOKE are auto-collected on the session instance at startup
- **Python intellisense** — [Jedi](https://jedi.readthedocs.io/)-powered completions and hover documentation in every code cell, sourced from the running kernel so the live namespace (all variables defined in previous cells) is available:
  - **Autocomplete** — trigger with `.` or `Ctrl+Space`; shows function, class, module, keyword, variable, and property completions with kind icons, fully-qualified name detail, and docstring popovers; runtime-aware so `df.` on a Pandas DataFrame shows all DataFrame methods
  - **Hover documentation** — hover any name to see its signature and docstring; function calls show the full parameter signature first; content is fetched from the kernel on demand

### SQL cells

- SQL cells execute through the **Snowpark kernel session** — the same session Python cells use — so `USE` commands in SQL cells affect Python cells and vice versa, and `SELECT CURRENT_DATABASE()` always returns the same value in both cell types
- SQL is split into individual statements by a parser that handles `--` line comments, `/* */` block comments, single-quoted strings, and `$$`-dollar-quoted strings; each statement runs in order and the last result is displayed
- **Run selection** — if text is selected in a SQL cell, only the selected SQL is executed
- `USE DATABASE X;` in a SQL cell updates the toolbar dropdowns and the Python session automatically
- Results render in a **ResultGrid** (up to 50 000 rows); when a query returns more than 50 000 rows a **truncated** tag is shown in the status bar
- DDL / DML with no result set shows "OK — N rows affected"

### Notebook management

- **Run All**, **Restart Kernel**, **Save** in the toolbar; **Deploy** button is stacked above the icon row in a vertical toolbar layout
- **Deploy** — deploys the notebook to Snowflake via a dialog with all `CREATE NOTEBOOK` options (database, schema, name, `OR REPLACE` / `IF NOT EXISTS`, comment, query warehouse, Python runtime warehouse, idle auto-shutdown seconds, runtime name, compute pool); works for both saved and unsaved notebooks — unsaved content is serialised and written to a temporary file automatically
- Per-cell controls: run, move up/down, add below, **delete** (confirmation dialog)
- **Cell gutter** — each cell has a left gutter showing the execution count and a colour-coded kind tag (Code / SQL / Markdown) with a per-kind accent stripe
- **AddCellBar** — hover-reveal bars between cells let you insert Code, SQL, or Markdown cells inline; the bar below the last cell is permanently visible
- **Command mode** — when no cell Monaco editor is focused, the selected cell (last clicked or focused, shown with an accent left border) can be operated on with single-key shortcuts:
  - `B` — add a new code cell below the selected cell
  - `A` — add a new code cell above the selected cell
  - `D D` — delete the selected cell (a confirmation dialog is always shown)
  - `Y` / `M` / `S` — change the selected cell's type to Code / Markdown / SQL
- Kernel status indicator: starting spinner → "Kernel ready" → "Kernel error"

---

## MCP Server

Thaw can expose the active Snowflake connection to external AI clients (Claude Desktop, Cursor, etc.) through the **Model Context Protocol**, built on the official Go MCP SDK over a localhost SSE/HTTP transport.

- **Multi-session** — open **View → MCP Sessions…** to start one or more independent servers. Each session is bound to its own dedicated Snowflake connection (inheriting the current connect parameters) and listens on its own localhost port, auto-assigned from `9100` (a port can be overridden). Because each session opens a *separate* Snowflake connection, interactive authenticators (e.g. `externalbrowser`) may re-prompt when a session starts, and every running session consumes one additional Snowflake session.
- **Lifecycle** — sessions start and stop only on explicit user action; all sessions stop cleanly when the app quits. There is no auto-start on launch. Sessions are **not persisted** — they exist only for the lifetime of the running app and are not restored on the next launch.
- **Execution modes** — three modes control what a session can do:
  - **Metadata Only** — schema-browsing and diagnostics tools only. No SQL execution.
  - **Read-Only SQL** — SQL execution via `execute_snowflake_sql`. Every statement passes through the EXPLAIN precompilation gate (only read-only operations allowed).
  - **Explain Only** — same gate validation as Read-Only, but returns only the EXPLAIN plan metadata without executing the statement.
- **EXPLAIN precompilation gate** — a defense-in-depth layer that validates every SQL statement before execution. Three layers: single-statement check, USE statement rejection, and EXPLAIN plan operation allow-listing (default-deny). The gate fails safe by over-rejecting — any unknown operation is denied. The real security boundary is the Snowflake role's grants; the gate provides an additional defense layer.
- **Session configuration** — non-metadata sessions can optionally pin the role and/or warehouse at startup. When pinned, the corresponding `use_role`/`use_warehouse` tool is not exposed to the AI client, preventing context-switching. Secondary roles can be set to "none" to restrict the session to only its primary role's grants.
- **Copy Config** — each running session offers a one-click copy of the client configuration block. The embedded URL carries the session's auth token, so the copied block is a **secret** — treat it like a password:
  ```json
  { "mcpServers": { "thaw-<label>": { "url": "http://localhost:<port>/sse?token=<token>" } } }
  ```
- **Tab delivery (`open_sql_tab`)** — an MCP tool that formats SQL with the user's editor preferences, runs the full diagnostics pipeline, and opens a new editor tab in Thaw with the result. Diagnostic markers appear inline immediately. The user must manually run the query (human-in-the-loop preserved). MCP-created tabs display a robot icon in the tab bar.
- **Notebook/Snowpark tools** — read notebook files (`read_notebook`, workspace-gated), get intellisense completions (`get_notebook_completions`), validate Python syntax (`check_python_syntax`), and deliver pre-filled notebooks into Thaw (`open_notebook_tab`). Kernel-dependent tools require an active notebook kernel. `open_notebook_tab` builds nbformat v4 JSON from python/markdown/sql cells and opens a new notebook tab with the robot icon badge. The user must manually run cells (human-in-the-loop preserved).
- **ER designer delivery (`open_er_designer`)** — an MCP tool that fetches live ER data from Snowflake, merges AI-generated tables onto the canvas, and opens the interactive ER Designer in Thaw. The AI can scaffold a complete data model from natural language; the user then visually refines table positions, columns, and FK relationships, reviews the generated diff SQL, and applies changes. Matching tables (by schema + name) are replaced; new tables are appended; untouched live tables are preserved.
- **ER designer inspection & modification (`get_er_designer_state`, `modify_er_designer`)** — MCP tools that let an AI client read the current state of an open ER designer (tables, columns, PKs, nullability, FKs) and push modifications into it. The designer's state is synced to the backend via IPC on mount, changes (debounced 300ms), and unmount. `modify_er_designer` emits a Wails event that the frontend merges into the live canvas, preserving table positions and React state. Enables iterative AI-assisted data modeling without re-opening the designer.
- **Data pipeline tools** — task graph inspection (`list_tasks`, `get_task_run_history`, `get_task_dependencies`), stage file browsing (`list_stage_files`, `preview_stage_file`), and Snowpipe status/history (`get_pipe_status`, `get_pipe_copy_history`). `preview_stage_file` is mode-gated (readonly/explain_only only). `open_task_graph` opens the interactive task graph visualization in Thaw from an MCP client.
- **Function & procedure metadata tools** — search the local function metadata cache (`search_functions`, `get_function_tooltip`), retrieve parameter metadata from Snowflake DDL (`get_procedure_params`, `get_function_info`), and generate invocation SQL (`build_call_statement`, `build_function_select`). Always registered in all modes.
- **DDL builder tools** — pure SQL generators for stages (`build_create_stage_sql`, `build_alter_stage_sql`), file formats (`build_create_file_format_sql`), pipes (`build_create_pipe_sql`, `build_refresh_pipe_sql`), secrets (`build_create_secret_sql`), and all six integration types (`build_storage_integration_sql`, `build_api_integration_sql`, `build_catalog_integration_sql`, `build_external_access_integration_sql`, `build_notification_integration_sql`, `build_security_integration_sql`). No Snowflake connection required — all tools return syntactically correct DDL without executing it. Always registered in all modes.
- **Migration & dbt tools** — scan local `.sql` files for DDL objects (`scan_migration_source`, workspace-gated), compare local objects against a live Snowflake database (`analyze_migration`), generate a human-readable migration script (`generate_migration_script`), and scaffold a dbt project pre-wired to the active connection (`generate_dbt_project`, workspace-gated). `scan_migration_source` and `generate_dbt_project` are only registered when a workspace root is configured; `analyze_migration` and `generate_migration_script` are always registered.
- **Toolbar indicator** — a "MCP: N active" pill appears in the toolbar while sessions are running; clicking it opens the MCP Sessions panel.
- Gated behind the **MCP Server** feature flag (admin-lockable; **View → Enabled Features → MCP Server**). The flag is enforced in the backend (`StartMCPSession`) using the effective flags, so an IT-admin lock cannot be bypassed via the native menu.
- **Security** — the listener binds only the loopback interface and rejects requests with a non-loopback `Host` header or a cross-origin `Origin` header, defending against DNS-rebinding attacks from a malicious web page. Each session additionally has a **per-session auth token** (crypto-random) required to open the SSE connection, presented as an `Authorization: Bearer` header or a `?token=…` query parameter — so another local process cannot read schema metadata without the token from the copied config. The token defends against non-admin local users only; a local administrator can bypass it (process memory, loopback capture). For SQL execution modes, always use a scoped read-only Snowflake role for defense in depth. Sessions should be stopped when not in use.

---

## Optional Features (Feature Flags)

Thaw allows toggling specific features to optimize performance or simplify the UI. Open **View → Enabled Features…** to manage them.

Features are organized into six categories, each with individual toggles:

**Data Export & Import** — Resultset Export, Table Data Export, Table Data Import, DDL Export

**Governance & Administration** — User & Role Management, Warehouse Management, Warehouse Credit Usage, Query Activity History, Integrations Management, Backup Policies & Sets

**AI & Assistance** — AI Inline Completions

**Advanced Tools & Data Engineering** — Schema Migration, dbt Project Scaffolding, ER Diagram & Designer, Task Graph Visualizer, Insert Mapping, Code Snippets

**Developer Environments** — Snowpark & Notebooks, Embedded Terminal, Git Integration

**Performance & Diagnostics** — Query Profile, Explain SQL, Query Log

### IT Admin Management

Enterprise deployments can enforce feature policies without user interaction. The Go backend evaluates configuration sources in a strict priority hierarchy:

1. **MDM / OS Registry (Highest)** — pushed by enterprise management tools
2. **System-Level Config** — a global JSON file installed by IT on each machine
3. **User-Level Config** — the user's personal preferences (modified via the UI)
4. **Hardcoded Defaults (Lowest)** — all features enabled

**System config file locations:**
- macOS: `/Library/Application Support/Thaw/features.json`
- Windows: `%PROGRAMDATA%\Thaw\features.json`
- Linux: `/etc/thaw/features.json`

**MDM / OS-native mechanisms:**
- macOS: Managed Preferences plist at `/Library/Managed Preferences/com.thaw.app.plist` or `~/Library/Preferences/com.thaw.app.plist`; keys use `Disable<FeatureName>` (e.g. `DisableDDLExport = true`)
- Windows: Group Policy / Registry at `HKEY_LOCAL_MACHINE\SOFTWARE\Policies\Thaw\Features` or `HKEY_CURRENT_USER\SOFTWARE\Policies\Thaw\Features`; DWORD values use `Disable<FeatureName> = 1`

**Config schema (JSON file):**
```json
{
  "dataExportImport":         { "ddlExport": false, "tableDataExport": false },
  "governanceAdministration": { "userRoleManagement": false },
  "ai":                       { "aiInlineCompletions": false },
  "advancedTools":            { "schemaMigration": false },
  "developerEnvironments":    { "snowparkNotebooks": false },
  "performanceDiagnostics":   { "explainSql": false }
}
```

When a feature is admin-controlled, its toggle in **View → Enabled Features…** is greyed out with a lock icon and the tooltip *"This setting is managed by your IT Administrator."*

### Feasible Optional Features

The following features are identified as feasible to be turned off via feature flags if needed, offering fine-grained control over the application's capabilities:

**Data Export & Import**
- **Resultset Export** (CSV and Excel downloads from query results)
- **Table Data Export** (Bulk data export to local files via Snowflake stages)
- **Table Data Import** (Bulk data ingestion from local CSV/JSON/Parquet files)
- **DDL Export** (Parallel database schema export to local disk)

**Governance & Administration**
- **User & Role Management** (Create, edit, and drop users; manage key-pair authentication)
- **Warehouse Management** (Edit properties, suspend/resume, abort queries)
- **Warehouse Credit Usage** (Visual charts for `ACCOUNT_USAGE` metering)
- **Query Activity History** (Searchable logs for session, user, or warehouse queries)
- **Integrations Management** (Manage Storage, API, Security, and other integrations)
- **Backup Policies & Sets** (Manage account-level backup policies and object-scoped backup sets)

**AI & Assistance**
- **AI Inline Completions** (Ghost-text suggestions in the editor)

**Advanced Tools & Data Engineering**
- **Schema Migration** (DDL diffing and deployment wizard)
- **dbt Project Scaffolding** (Automated dbt project generation)
- **DBT Project Browser** (Browse and manage Snowflake-native DBT PROJECT objects in the sidebar)
- **ER Diagram & Designer** (Visual database modeling and `ALTER TABLE` generation)
- **Task Graph Visualizer** (Interactive DAG viewer and manager for Snowflake tasks)
- **Insert Mapping** (Visual side-by-side mapping for `INSERT INTO ... SELECT` with UNIONs)
- **File Format Builder** (Visual CREATE FILE FORMAT builder and data previewer)
- **Code Snippets** (Library of curated `CREATE OR REPLACE` templates)

**Developer Environments**
- **Snowpark & Notebooks** (Embedded Python kernel and environment manager)
- **Embedded Terminal** (xterm.js OS shell panel)
- **Git Integration** (Git status, commit, and push/pull UI)

**Performance & Diagnostics**
- **Query Profile** (Operator statistics and execution time breakdown graphs)
- **Explain SQL** (Pre-execution linter for full table scans and cartesian joins)
- **Query Log** (Session-scoped log of all SQL queries Thaw sends to Snowflake, for debugging and issue reporting)

**Results Grid**
- **Multi-Cell Copy & Selection** (Range selection, multi-cell copy, selection aggregations, and quick charting)
- **Cell Detail Panel** (Side panel for inspecting and copying the full content of the selected cell)

**SQL Editor**
- **Cross-Tab Search & Replace** (Search and replace text across all open query tabs and notebook cells)

**Connection**
- **Snowflake CLI Profile Manager** (Manage Snowflake CLI profiles from the connection dialog)

**File Browser**
- **File Watcher** (Auto-refresh the file browser when files are created, renamed, or deleted externally)

**Schema Management**
- **Column Management** (Add, rename, retype, set/drop NOT NULL, set comment, and drop table columns from the sidebar tree)

**Integrations**
- **MCP Server** (Expose the active Snowflake connection to external AI clients over a local Model Context Protocol server)

---

## UI & Theming

- **Light, Dark, and System** themes — switch via **View → Appearance**; preference is saved across sessions
- **Session restoration across app restarts** — all open tabs (scratch SQL, file tabs, notebook tabs) and their SQL content are restored exactly when the app is relaunched; file-backed tabs re-read their content from disk on startup so they always show the current file; if a file has been deleted or moved the tab becomes a scratch tab (prefixed `↺`) so the last-known SQL content is not lost; window size is saved on quit and restored on the next launch
- **Tools menu** — native menu bar **Tools** entry provides **Code Snippets…**, **Export Path Format…**, **Schema Migration…**, and **Create dbt Project…**
- **Snowpark menu** — native menu bar **Snowpark** entry provides **Check Environment…**, **Setup Environment…**, **New Notebook…**, and **Open Notebook…**
- **Help menu** — **Function Catalog…** opens the built-in Snowflake function reference with overload signatures and descriptions for every function; **Keyboard Shortcuts…** opens a searchable modal listing every shortcut with macOS and Windows columns
- **Resizable sidebars** — drag either sidebar edge to any width between 160 px and 600 px
- **Resizable editor/results split** — drag the horizontal divider between the SQL editor and the results pane to any ratio; position is saved across sessions
- **Drag-and-drop panel layout** — every sidebar panel (Export DDL, File Browser, Git, Object Browser, Administration) has a drag handle at its top edge; drag panels between the left and right sidebars or reorder them within a sidebar; layout is persisted across sessions
- **Reset Layout** — restore the default panel positions and editor/results split via the **Customize Layout…** dialog (accessible from the **View** menu)
- **Resizable object browser** — collapse, expand, or drag to resize the object tree panel
- Right-click context menus are always clamped inside the viewport
- Closing the app while a query is running prompts a confirmation dialog; the query is cancelled in Snowflake before exit

---

## Code Quality & CI/CD

- **Unit tests** — DDL parser, lineage parser, dbt generator, migration helper, SQL diagnostics engine (`internal/sqleditor`), and **SQL formatter** tests run on every commit; 169 vitest tests cover the frontend diagnostic and formatter layers; Go unit tests for `internal/sqleditor` validate the analysis engine against complex scripting and multi-join patterns; DDL parser, lineage parser, dbt generator, and migration helper tests run on every commit
- **Integration tests** — 18 `TestFormatterSQL` cases validate that formatted SQL patterns execute on a real Snowflake account without syntax errors (no `CREATE TABLE` or elevated privileges needed); DDL export and schema migration integration tests are gated behind a build tag and run separately
- **golangci-lint** — static analysis (weekly, every Monday): unchecked errors, vet, staticcheck, unused symbols, misspellings, and style (`errcheck`, `govet`, `staticcheck`, `ineffassign`, `unused`, `misspell`, `revive`)
- **govulncheck** — vulnerability scanning against the Go vulnerability database (weekly); reports only vulnerabilities reachable from the compiled binary
- **gosec** — security static analysis (weekly): hardcoded credentials, weak crypto, TLS misconfigurations, unsafe operations
- **IP protection** — proprietary SQL analysis algorithms (syntax tokenizer, semantic validator, JOIN condition engine) are implemented in the Go backend (`internal/sqleditor`) and compiled into the binary; the frontend bundle is additionally processed by Terser (2-pass minification, no source maps) and `javascript-obfuscator` (RC4 string-array encoding, hexadecimal identifier renaming) so that app logic is not recoverable from the shipped binary
- **Release builds** — macOS (arm64), Windows (amd64), and Linux (amd64) binaries are built automatically when a version tag (`v*`) is pushed to `main`; artifacts are named after the tag (e.g. `thaw-v1.2.3-darwin-arm64.zip`)

All security/quality tools can also be run locally — see the `README.md` for install and run commands.

---

## Keyboard Shortcuts

Open **Help → Keyboard Shortcuts…** in the menu bar for a searchable, always-up-to-date reference.

### Tabs & Navigation

| macOS | Windows / Linux | Action |
|-------|-----------------|--------|
| `⌘T` | `Ctrl+T` | New scratch tab |
| `⌘O` | `Ctrl+O` | Open SQL file |
| `⌘S` | `Ctrl+S` | Save active file |
| `⌘⇧S` | `Ctrl+Shift+S` | Save As… |
| `⌘W` | `Ctrl+W` | Close current tab |
| `⌘⇧T` | `Ctrl+Shift+T` | Reopen last closed tab |
| `⌃Tab` | `Ctrl+Tab` | Switch to next tab |
| `⌃⇧Tab` | `Ctrl+Shift+Tab` | Switch to previous tab |
| `⌘,` | `Ctrl+,` | Open Preferences (AI settings) |

### Query Execution

| macOS | Windows / Linux | Action |
|-------|-----------------|--------|
| `⌘ Enter` | `Ctrl+Enter` | Run query (or selected text) |
| `⌘⇧ Enter` | `Ctrl+Shift+Enter` | Run all statements |
| `Esc` | `Esc` | Cancel running query |
| `⌘↓` | `Ctrl+↓` | Focus results grid |
| `⌘G` | `Ctrl+G` | Toggle grid search |
| `⌘E` | `Ctrl+E` | Export current results as CSV |

### Editor

| macOS | Windows / Linux | Action |
|-------|-----------------|--------|
| `⌘/` | `Ctrl+/` | Toggle line comment |
| `⇧⌥A` | `Shift+Alt+A` | Toggle block comment |
| `⇧⌥F` | `Shift+Alt+F` | Format SQL (selection or full document) |
| `Ctrl+Space` | `Ctrl+Space` | Trigger autocomplete |
| `Tab` | `Tab` | Accept AI suggestion |
| `⌘F` | `Ctrl+F` | Find in document |
| `⌘⌥F` | `Ctrl+H` | Find and replace |
| `⌘⇧H` | `Ctrl+Shift+H` | Find & replace across tabs |
| `⌘D` | `Ctrl+D` | Select next occurrence |
| `⌃G` | `Ctrl+G` | Go to line |
| `⌘⌥↑` | `Ctrl+Alt+↑` | Add cursor above |
| `⌘⌥↓` | `Ctrl+Alt+↓` | Add cursor below |
| `⌘+` | `Ctrl++` | Increase editor font size |
| `⌘-` | `Ctrl+-` | Decrease editor font size |
| `⌘0` | `Ctrl+0` | Reset editor font size to default |

### UI & Panels

| macOS | Windows / Linux | Action |
|-------|-----------------|--------|
| `⌘B` | `Ctrl+B` | Toggle left sidebar |
| `⌘⇧F` | `Ctrl+Shift+F` | Focus object browser search |
| `⌘\` | `Ctrl+\` | Toggle split editor view |
| `⌘\`` | `Ctrl+\`` | Open embedded terminal |
| `⌘G` | `Ctrl+G` | Open Git Operations… |

### Notebook (Command Mode — no cell editor focused)

| Key | Action |
|-----|--------|
| `Shift+Enter` | Run current cell |
| `B` | Add cell below |
| `A` | Add cell above |
| `D D` | Delete current cell (confirmation required) |
| `Y` | Change cell type to Code |
| `M` | Change cell type to Markdown |
| `S` | Change cell type to SQL |

---

*Thaw is built with Go, Wails, React, Ant Design, Monaco Editor, and TanStack Table.*
