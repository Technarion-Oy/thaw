// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowpark

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"thaw/internal/apperrors"
	"thaw/internal/config"
	"thaw/internal/snowflake"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SnowparkCondaEnv is the name of the conda environment used for Snowpark.
const SnowparkCondaEnv = "thaw_snowpark"

// Markers used by the embedded Python kernel to delimit cell execution.
const kernelSentinel = "<<<THAW_CELL_DONE>>>"
const kernelRunMarker = "<<<THAW_RUN>>>"
const kernelCompleteMarker = "<<<THAW_COMPLETE>>>"
const kernelHoverMarker = "<<<THAW_HOVER>>>"
const kernelSqlMarker = "<<<THAW_SQL>>>"
const kernelSyntaxMarker = "<<<THAW_SYNTAX>>>"
const kernelDebugMarker = "<<<THAW_DEBUG_RUN>>>"
const kernelDebugResultMarker = "<<<THAW_DEBUG_RESULT>>>"
const kernelExecMarker = "<<<THAW_EXEC_CELL>>>"

// Global state for the Debug Adapter Protocol (DAP) connection
var (
	dapConn      net.Conn
	dapMutex     sync.Mutex
	dapProxyOnce sync.Once
)

// kernelPyScript is a minimal stateful Python kernel.  It reads code blocks
// from stdin (terminated by kernelRunMarker), executes them in a shared
// namespace, captures stdout/stderr, captures matplotlib figures as base64
// PNGs, and writes a JSON result + sentinel line.
const kernelPyScript = `import sys, io, traceback, json, base64, warnings, os
import ast as _ast

SENTINEL   = "<<<THAW_CELL_DONE>>>"
RUN_MARKER = "<<<THAW_RUN>>>"
DEBUG_MARKER = "<<<THAW_DEBUG_RUN>>>"
DEBUG_RESULT_MARKER = "<<<THAW_DEBUG_RESULT>>>"
EXEC_MARKER = "<<<THAW_EXEC_CELL>>>"

# Force matplotlib to use the non-interactive Agg backend so that show()
# does not try to open a GUI window.
try:
    import matplotlib
    matplotlib.use("Agg")
    # plt.show() on the Agg backend emits a UserWarning explaining that the
    # canvas is non-interactive.  Suppress it — figures are captured as PNG.
    warnings.filterwarnings("ignore", message="FigureCanvasAgg is non-interactive")
except Exception:
    pass

import re as _re
# DDL pattern: statements that should execute immediately (no lazy .collect() needed).
_THAW_DDL_RE = _re.compile(
    r'^\s*(USE|CREATE|ALTER|DROP|TRUNCATE|COMMENT|GRANT|REVOKE)\b',
    _re.IGNORECASE,
)

# Private state for internal kernel use, completely isolated from user code.
_THAW_INTERNAL_STATE = {}
# User namespace
g = {}

# ── Auto-create Snowpark session matching the Thaw app connection ──────────────
# Connection parameters are injected via THAW_SF_* environment variables so
# that Python cells share the same account, role, warehouse, database and
# schema as SQL cells.  The function puts a 'session' variable directly into
# the user-cell namespace 'g' so it is available in every cell without any
# import or builder call.
def _thaw_create_session(ns):
    account = os.environ.get("THAW_SF_ACCOUNT", "")
    user    = os.environ.get("THAW_SF_USER", "")
    if not account or not user:
        return  # not connected

    auth = os.environ.get("THAW_SF_AUTHENTICATOR", "snowflake").lower()
    if auth == "externalbrowser":
        # Cannot automate browser-based SSO — the user must call Session.builder manually.
        print("[thaw] externalbrowser auth: create session manually with Session.builder", file=sys.stderr)
        return

    cfg = {"account": account, "user": user}
    for k, v in [
        ("role",      os.environ.get("THAW_SF_ROLE", "")),
        ("warehouse", os.environ.get("THAW_SF_WAREHOUSE", "")),
        ("database",  os.environ.get("THAW_SF_DATABASE", "")),
        ("schema",    os.environ.get("THAW_SF_SCHEMA", "")),
    ]:
        if v:
            cfg[k] = v

    pk_path = os.environ.get("THAW_SF_PRIVATE_KEY_PATH", "")
    pk_pass = os.environ.get("THAW_SF_PRIVATE_KEY_PASSPHRASE", "")
    okta_url = os.environ.get("THAW_SF_OKTA_URL", "")

    if auth in ("snowflake_jwt", "jwt"):
        cfg["authenticator"]   = "snowflake_jwt"
        cfg["private_key_file"] = pk_path  # snowflake-connector-python expects "private_key_file"
        if pk_pass:
            # passphrase must be bytes, not str
            cfg["private_key_file_pwd"] = pk_pass.encode("utf-8")
    elif auth == "okta" and okta_url:
        cfg["authenticator"] = okta_url  # Snowpark uses the Okta URL as authenticator
        cfg["password"]      = os.environ.get("THAW_SF_PASSWORD", "")
    else:
        cfg["password"] = os.environ.get("THAW_SF_PASSWORD", "")
        if auth and auth != "snowflake":
            cfg["authenticator"] = auth

    try:
        from snowflake.snowpark import Session as _Session
        # Redirect stdout during session creation — Snowpark may print banners
        # that would corrupt the stdin/stdout protocol.
        _old_out, sys.stdout = sys.stdout, io.StringIO()
        try:
            _sess = _Session.builder.configs(cfg).create()
            # Store original sql() before patching into INTERNAL STATE (not the user namespace).
            _orig_sql = _sess.sql
            _THAW_INTERNAL_STATE['orig_sql'] = _orig_sql
            # Patch instance-level sql() so DDL/USE statements execute
            # immediately without an explicit .collect(), matching Snowflake
            # native notebook behavior.
            def _auto_collect_sql(_query, *_a, **_kw):
                _df = _orig_sql(_query, *_a, **_kw)
                if _THAW_DDL_RE.match(_query.strip()):
                    try:
                        _df.collect()
                    except Exception:
                        pass
                return _df
            _sess.sql = _auto_collect_sql
            #ns['session'] = _sess
        finally:
            sys.stdout = _old_out
    except Exception as _e:
        # Store full traceback so it surfaces in the first cell's stderr output
        # rather than being lost to the discarded kernel stderr stream.
        import traceback as _tb
        _thaw_init_errors.append(_tb.format_exc())

_thaw_init_errors = []
_thaw_create_session(g)
del _thaw_create_session

# ── Intellisense protocol markers ─────────────────────────────────────────────
COMPLETE_MARKER = "<<<THAW_COMPLETE>>>"
HOVER_MARKER    = "<<<THAW_HOVER>>>"
SQL_MARKER      = "<<<THAW_SQL>>>"
SYNTAX_MARKER   = "<<<THAW_SYNTAX>>>"

def _thaw_handle_complete(req_json):
    """Return jedi completions as JSON, using the live kernel namespace."""
    try:
        import jedi as _jedi, json as _json
        _req = _json.loads(req_json)
        _src = _req.get("code", "")
        _ln  = _req.get("line", 1)
        _col = _req.get("col", 0)
        try:
            _s = _jedi.Interpreter(_src, [g])
        except Exception:
            _s = _jedi.Script(_src)
        _items = []
        for _c in _s.complete(_ln, _col)[:200]:
            # Obfuscation: Filter out any internal variables or functions starting with _ or containing 'thaw'
            if _c.name.startswith('_') or 'thaw' in _c.name.lower():
                continue

            try:    _doc = _c.docstring(raw=True)[:1000]
            except: _doc = ""
            _items.append({
                "label": _c.name,
                "type":  _c.type,
                "detail": _c.full_name or "",
                "documentation": _doc,
            })
        return _json.dumps({"completions": _items})
    except Exception as _ex:
        import json as _j
        return _j.dumps({"completions": [], "error": str(_ex)})

def _thaw_handle_hover(req_json):
    """Return jedi hover documentation as JSON, using the live kernel namespace."""
    try:
        import jedi as _jedi, json as _json
        _req = _json.loads(req_json)
        _src = _req.get("code", "")
        _ln  = _req.get("line", 1)
        _col = _req.get("col", 0)
        try:
            _s = _jedi.Interpreter(_src, [g])
        except Exception:
            _s = _jedi.Script(_src)
        _text = ""
        # Signatures are richer for function calls
        try:
            _sigs = _s.get_signatures(_ln, _col)
            if _sigs:
                _lbl = _sigs[0].to_string()
                try:    _d = _sigs[0].docstring(raw=True)
                except: _d = ""
                _text = _lbl + ("\n\n" + _d if _d else "")
        except Exception:
            pass
        if not _text:
            try:
                _helps = _s.help(_ln, _col)
                if _helps:
                    try:    _d = _helps[0].docstring(raw=True)
                    except: _d = ""
                    _n = getattr(_helps[0], "full_name", None) or getattr(_helps[0], "name", None) or ""
                    _text = (_n + "\n\n" + _d) if (_n and _d) else (_d or _n)
            except Exception:
                pass
        return _json.dumps({"hover": _text})
    except Exception as _ex:
        import json as _j
        return _j.dumps({"hover": "", "error": str(_ex)})

def _thaw_handle_syntax(req_json):
    """Check Python syntax + undefined names."""
    try:
        import json as _json
        _req = _json.loads(req_json)
        code = _req.get("code", "")
        mode = _req.get("mode", "kernel")
    except Exception:
        import json as _json
        code = req_json.strip()
        mode = "kernel"

    if mode == "off":
        return _json.dumps({"errors": []})

    # ── 1. Syntax check ───────────────────────────────────────────────────────
    try:
        tree = _ast.parse(code)
    except SyntaxError as _e:
        err = {
            "severity": "error",
            "line":     _e.lineno or 1,
            "col":      (_e.offset or 1) - 1,
            "msg":      _e.msg,
        }
        if getattr(_e, "end_offset", None) is not None:
            err["endCol"] = _e.end_offset - 1
        return _json.dumps({"errors": [err]})
    except Exception:
        return _json.dumps({"errors": []})

    # ── 2. Pyflakes: undefined names + import errors ─────────────────────────
    try:
        from pyflakes import checker as _pfc, messages as _pfm
        if mode == "kernel":
            # Prepend stub assignments (name = None at lineno 0) for every name
            # in the live kernel namespace g.  g is only populated by cells that
            # have actually been executed, so pyflakes will not flag cross-cell
            # variables as undefined as long as the user has run those cells.
            _stubs = []
            for _name in g:
                if isinstance(_name, str) and _name.isidentifier():
                    _stub = _ast.Assign(
                        targets=[_ast.Name(id=_name, ctx=_ast.Store())],
                        value=_ast.Constant(value=None),
                        lineno=0, col_offset=0,
                    )
                    _ast.fix_missing_locations(_stub)
                    _stubs.append(_stub)
            _aug = _ast.Module(body=_stubs + tree.body, type_ignores=tree.type_ignores)
            _ast.fix_missing_locations(_aug)
            _w = _pfc.Checker(_aug, "<cell>")
        else:
            # "static" mode: analyze the cell in isolation, no kernel state
            _w = _pfc.Checker(tree, "<cell>")
        errors = []
        for _msg in _w.messages:
            if getattr(_msg, "lineno", 1) < 1:
                continue  # skip stub-injected lines (lineno 0)
            # "redefinition of unused X from line 0" is a false positive: the
            # original "definition" is one of the kernel-namespace stubs injected
            # above (lineno=0).  Suppress it so re-running a cell that defines a
            # function or variable doesn't produce a spurious warning.
            if isinstance(_msg, _pfm.RedefinedWhileUnused):
                _orig_line = _msg.message_args[1] if len(_msg.message_args) >= 2 else 1
                if _orig_line <= 0:
                    continue
            _is_err = isinstance(_msg, (_pfm.UndefinedName, _pfm.UndefinedLocal))
            errors.append({
                "severity": "error" if _is_err else "warning",
                "line":     _msg.lineno,
                "col":      getattr(_msg, "col", 0),
                "msg":      _msg.message % _msg.message_args,
            })
        return _json.dumps({"errors": errors})
    except ImportError:
        pass  # pyflakes not available; fall back to import-existence check

    # ── 3. Fallback: importlib.util.find_spec for missing top-level modules ────
    import importlib.util as _ilu
    errors = []
    for _node in _ast.walk(tree):
        if isinstance(_node, _ast.Import):
            for _alias in _node.names:
                _top = _alias.name.split(".")[0]
                try:
                    _found = _ilu.find_spec(_top)
                except (ModuleNotFoundError, ValueError):
                    _found = None
                if _found is None:
                    errors.append({
                        "severity": "warning",
                        "line": _node.lineno,
                        "col": _node.col_offset,
                        "msg": "Module not found: '" + _alias.name + "'",
                    })
        elif isinstance(_node, _ast.ImportFrom) and _node.module:
            _top = _node.module.split(".")[0]
            try:
                _found = _ilu.find_spec(_top)
            except (ModuleNotFoundError, ValueError):
                _found = None
            if _found is None:
                errors.append({
                    "severity": "warning",
                    "line": _node.lineno,
                    "col": _node.col_offset,
                    "msg": "Module not found: '" + _node.module + "'",
                })
    return _json.dumps({"errors": errors})

def _capture_figures():
    """Return a list of base64-encoded PNG strings for all open figures."""
    images = []
    try:
        import matplotlib.pyplot as plt
        for num in plt.get_fignums():
            fig = plt.figure(num)
            buf = io.BytesIO()
            fig.savefig(buf, format="png", bbox_inches="tight", dpi=150)
            buf.seek(0)
            images.append(base64.b64encode(buf.read()).decode("utf-8"))
        plt.close("all")
    except Exception:
        pass
    return images

def _thaw_get_session_context():
    """Return current Snowpark session context (role/warehouse/database/schema)."""
    try:
        from snowflake.snowpark.context import get_active_session as _gas
        _s = _gas()
        _v = lambda x: (x or "")
        return {
            "role":      _v(_s.get_current_role()),
            "warehouse": _v(_s.get_current_warehouse()),
            "database":  _v(_s.get_current_database()),
            "schema":    _v(_s.get_current_schema()),
        }
    except Exception:
        return {}

def _thaw_split_sql(sql):
    """Split SQL into statements on ';', respecting strings and comments."""
    stmts, buf, i, n = [], [], 0, len(sql)
    while i < n:
        ch = sql[i]
        if ch == '-' and i+1 < n and sql[i+1] == '-':          # line comment
            end = sql.find('\n', i)
            buf.append(sql[i:] if end < 0 else sql[i:end+1])
            i = n if end < 0 else end+1
        elif ch == '/' and i+1 < n and sql[i+1] == '*':        # block comment
            end = sql.find('*/', i+2)
            buf.append(sql[i:] if end < 0 else sql[i:end+2])
            i = n if end < 0 else end+2
        elif ch == "'":                                          # single-quoted string
            j = i+1
            while j < n:
                if sql[j] == "'" and j+1 < n and sql[j+1] == "'": j += 2
                elif sql[j] == "'": j += 1; break
                else: j += 1
            buf.append(sql[i:j]); i = j
        elif ch == '$' and i+1 < n and sql[i+1] == '$':        # dollar-quoted string
            end = sql.find('$$', i+2)
            buf.append(sql[i:] if end < 0 else sql[i:end+2])
            i = n if end < 0 else end+2
        elif ch == ';':
            s = ''.join(buf).strip()
            if s: stmts.append(s)
            buf = []; i += 1
        else:
            buf.append(ch); i += 1
    s = ''.join(buf).strip()
    if s: stmts.append(s)
    return stmts

def _thaw_run_sql_cell(sql_str):
    """Run sql_str via the active Snowpark session; returns JSON with columns/rows/error/session_context."""
    import json as _json
    def _jval(v):
        if v is None: return None
        import decimal as _dec, datetime as _dt
        if isinstance(v, _dec.Decimal): return float(v)
        if isinstance(v, (_dt.datetime, _dt.date, _dt.time)): return str(v)
        if isinstance(v, (int, float, bool, str)): return v
        return str(v)

    # Use the original (unpatched) sql() to avoid double-collecting DDL.
    _sql_fn = _THAW_INTERNAL_STATE.get('orig_sql')
    if _sql_fn is None:
        try:
            from snowflake.snowpark.context import get_active_session as _gas
            _sql_fn = _gas().sql
        except Exception as _e:
            return _json.dumps({"columns": [], "rows": [], "rowCount": 0,
                                "error": "No active session: " + str(_e),
                                "session_context": _thaw_get_session_context()})

    stmts = _thaw_split_sql(sql_str)
    last = {"columns": [], "rows": [], "rowCount": 0, "error": None, "truncated": False}
    for _stmt in stmts:
        _stmt = _stmt.strip()
        if not _stmt:
            continue
        try:
            _df = _sql_fn(_stmt)
            # Row capping: fetch max 50,001 to detect truncation.
            _rows = _df.limit(50001).collect()
            _truncated = len(_rows) > 50000
            if _truncated:
                _rows = _rows[:50000]

            last = {
                "columns": [f.name for f in _df.schema.fields],
                "rows":    [[_jval(v) for v in r] for r in _rows],
                "rowCount": len(_rows),
                "error": None,
                "truncated": _truncated,
            }
        except Exception as _ex:
            last = {"columns": [], "rows": [], "rowCount": 0, "error": str(_ex), "truncated": False}
    last["session_context"] = _thaw_get_session_context()
    return _json.dumps(last)

while True:
    lines = []
    while True:
        line = sys.stdin.readline()
        if not line:
            sys.exit(0)
        if line.rstrip('\n') == RUN_MARKER:
            break
        lines.append(line)

    code = "".join(lines)

    # ── Protocol requests ─────────────────────────────────────────────────────
    if code.startswith(COMPLETE_MARKER + "\n"):
        print(_thaw_handle_complete(code[len(COMPLETE_MARKER) + 1:]), flush=True)
        print(SENTINEL, flush=True)
        continue
    if code.startswith(HOVER_MARKER + "\n"):
        print(_thaw_handle_hover(code[len(HOVER_MARKER) + 1:]), flush=True)
        print(SENTINEL, flush=True)
        continue
    if code.startswith(SQL_MARKER + "\n"):
        print(_thaw_run_sql_cell(code[len(SQL_MARKER) + 1:]), flush=True)
        print(SENTINEL, flush=True)
        continue
    if code.startswith(SYNTAX_MARKER + "\n"):
        print(_thaw_handle_syntax(code[len(SYNTAX_MARKER) + 1:]), flush=True)
        print(SENTINEL, flush=True)
        continue

    # ── Debug Execution (DAP/debugpy) ─────────────────────────────────────────
    if code.startswith(DEBUG_MARKER + "\n"):
        import tempfile
        import debugpy
        import runpy

        parts = code.split("\n", 2)
        cell_id = parts[1].strip() if len(parts) > 1 else "unknown"
        cell_code = parts[2] if len(parts) > 2 else ""

        # Write code to a physical file so debugpy has a path to set breakpoints.
        # A trailing 'pass' sentinel gives debugpy a line to land on after the
        # last real line executes, so breakpoints on the final line fire correctly.
        debug_filepath = os.path.join(tempfile.gettempdir(), f"thaw_cell_{cell_id}.py")
        with open(debug_filepath, "w", encoding="utf-8") as f:
            f.write(cell_code)
            if not cell_code.endswith("\n"):
                f.write("\n")
            f.write("pass  # _thaw_end\n")

        _debug_first_sentinel_done = False
        try:
            try:
                # Start the debugger (ignores if already listening)
                debugpy.listen(("127.0.0.1", 5678))
            except RuntimeError:
                pass

            # Signal to Go proxy that debugpy is waiting AND send the filepath
            print(json.dumps({
                "status": "DEBUG_READY",
                "filepath": debug_filepath
            }), flush=True)
            print(SENTINEL, flush=True)
            _debug_first_sentinel_done = True

            # Freeze until the frontend completes the DAP handshake (configurationDone).
            # Closing the TCP connection (via StopDapProxy) will unblock this if the
            # frontend gives up early.
            # If the server is unavailable (internal state reset after a previous
            # session), re-listen once and retry before giving up.
            try:
                debugpy.wait_for_client()
            except RuntimeError as _wfc_e:
                if "not available" in str(_wfc_e).lower():
                    debugpy.listen(("127.0.0.1", 5678))
                    debugpy.wait_for_client()
                else:
                    raise

            # Redirect only stderr; leave stdout un-redirected so that print()
            # calls flow directly to the kernel pipe and Go can stream them to
            # the frontend in real-time while the debugger is paused.
            buf_err = io.StringIO()
            old_err = sys.stderr
            sys.stderr = buf_err
            err_info = None

            try:
                runpy.run_path(debug_filepath, init_globals=g)
            except Exception:
                err_info = traceback.format_exc()
            finally:
                sys.stderr = old_err

            sys.stdout.flush()
            images = _capture_figures()
            result = {"stdout": "", "stderr": buf_err.getvalue(), "error": err_info, "images": images, "session_context": _thaw_get_session_context()}
            print(DEBUG_RESULT_MARKER + json.dumps(result), flush=True)
            print(SENTINEL, flush=True)
        except Exception as e:
            if not _debug_first_sentinel_done:
                print(json.dumps({"error": str(e)}), flush=True)
            else:
                print(DEBUG_RESULT_MARKER + json.dumps({"error": str(e)}), flush=True)
            print(SENTINEL, flush=True)

        continue

    # ── Standard Execution ────────────────────────────────────────────────────
    # Optionally prefixed with EXEC_MARKER + cellId so the code is written to
    # a physical temp file.  This gives any functions defined here a real
    # co_filename so debugpy can step into them from a later debug session.
    _exec_filepath = None
    if code.startswith(EXEC_MARKER + "\n"):
        import tempfile as _tempfile
        parts = code.split("\n", 2)
        _exec_cell_id = parts[1].strip() if len(parts) > 1 else None
        code = parts[2] if len(parts) > 2 else ""
        if _exec_cell_id:
            try:
                _exec_filepath = os.path.join(_tempfile.gettempdir(), f"thaw_cell_{_exec_cell_id}.py")
                with open(_exec_filepath, "w", encoding="utf-8") as _f:
                    _f.write(code)
            except Exception:
                _exec_filepath = None

    buf_out = io.StringIO()
    buf_err = io.StringIO()
    # Drain any deferred session-init errors into the first cell's stderr.
    while _thaw_init_errors:
        buf_err.write(_thaw_init_errors.pop(0))
    old_out, old_err = sys.stdout, sys.stderr
    sys.stdout = buf_out
    sys.stderr = buf_err
    err_info   = None

    try:
        # Obfuscation: AST Security Check. Prevent users from reassigning session or accessing internal dicts.
        tree = compile(code, "<cell>", "exec", flags=_ast.PyCF_ONLY_AST)
        for node in _ast.walk(tree):
            if isinstance(node, _ast.Name) and node.id in ['_THAW_INTERNAL_STATE', 'kernelRPC', 'SENTINEL', 'RUN_MARKER']:
                raise NameError(f"Access denied: '{node.id}' is a reserved internal variable.")

        # Compile with the real file path when available so that functions
        # defined here have a valid co_filename that debugpy can step into.
        bytecode = compile(tree, _exec_filepath or "<cell>", "exec")
        exec(bytecode, g)
    except Exception:
        err_info = traceback.format_exc()
    finally:
        sys.stdout = old_out
        sys.stderr = old_err

    images = _capture_figures()
    result = {"stdout": buf_out.getvalue(), "stderr": buf_err.getvalue(), "error": err_info, "images": images, "session_context": _thaw_get_session_context()}
    print(json.dumps(result), flush=True)
    print(SENTINEL, flush=True)
`

// ─── exported structs ─────────────────────────────────────────────────────────

// SnowparkCheckResult describes the state of the local Snowpark environment.
type SnowparkCheckResult struct {
	IsReady             bool   `json:"isReady"`
	Details             string `json:"details"`
	PythonPath          string `json:"pythonPath"`
	Version             string `json:"version"`             // Python version inside the active env
	SystemPythonVersion string `json:"systemPythonVersion"` // Python version found on PATH
	Backend             string `json:"backend"`             // "conda" | "venv"
	VenvPath            string `json:"venvPath"`            // effective venv path when backend=venv
	HasConda            bool   `json:"hasConda"`
	HasEnv              bool   `json:"hasEnv"`  // conda env exists
	HasVenv             bool   `json:"hasVenv"` // venv exists
	HasSnowpark         bool   `json:"hasSnowpark"`
	HasNotebook         bool   `json:"hasNotebook"`
}

// SnowparkConfigResult is returned by GetSnowparkConfig.
type SnowparkConfigResult struct {
	Backend    string `json:"backend"`    // "conda" | "venv"
	VenvPath   string `json:"venvPath"`   // effective venv path (never empty)
	PythonPath string `json:"pythonPath"` // saved python binary for venv (may be empty)
}

// PythonInfo describes a Python interpreter found on the system.
type PythonInfo struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// NotebookSessionContext captures the Snowpark session state returned by the
// Python kernel after each cell execution.  When any field changes it means
// the user's cell ran a USE command (e.g. session.sql("USE DATABASE X")).
type NotebookSessionContext struct {
	Role      string `json:"role"`
	Warehouse string `json:"warehouse"`
	Database  string `json:"database"`
	Schema    string `json:"schema"`
}

// NotebookCellOutput is the result of running a single notebook cell.
type NotebookCellOutput struct {
	Stdout         string                  `json:"stdout"`
	Stderr         string                  `json:"stderr"`
	Error          string                  `json:"error"`
	Images         []string                `json:"images"`          // base64-encoded PNG figures (e.g. matplotlib plots)
	SessionContext *NotebookSessionContext `json:"session_context"` // current kernel session state
}

// NotebookSqlResult is returned by RunNotebookSql.
type NotebookSqlResult struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	RowCount  int64    `json:"rowCount"`
	QueryID   string   `json:"queryID"`
	Truncated bool     `json:"truncated"`
}

// PackageInfo describes a Python package installed in the Snowpark environment.
type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// NotebookCompletion is a single intellisense completion item returned by jedi
// from the running Python kernel.
type NotebookCompletion struct {
	Label         string `json:"label"`
	Type          string `json:"type"`          // jedi type: "function", "class", "module", …
	Detail        string `json:"detail"`        // fully-qualified name
	Documentation string `json:"documentation"` // raw docstring (may be empty)
}

// NotebookSyntaxError describes a single Python diagnostic returned by CheckPythonSyntax.
// Line is 1-indexed; Col and EndCol are 0-indexed (Monaco adds 1 when applying markers).
// Severity is "error" (syntax errors) or "warning" (e.g. module-not-found).
type NotebookSyntaxError struct {
	Severity string `json:"severity"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	EndCol   *int   `json:"endCol"`
	Msg      string `json:"msg"`
}

// ─── kernel session management ────────────────────────────────────────────────

type notebookSession struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        *bufio.Reader
	mu            sync.Mutex
	lastCtx       NotebookSessionContext // last known kernel session context
	pythonVersion string                 // e.g. "3.11.9", detected at start
}

var (
	notebookSessions sync.Map // tabId (string) → *notebookSession
	kernelScriptOnce sync.Once
	kernelScriptPath string
	kernelScriptErr  error
)

func ensureKernelScript() (string, error) {
	kernelScriptOnce.Do(func() {
		f, err := os.CreateTemp("", "thaw_kernel_*.py")
		if err != nil {
			kernelScriptErr = err
			return
		}
		if _, err = f.WriteString(kernelPyScript); err != nil {
			kernelScriptErr = err
			f.Close() //nolint:errcheck
			return
		}
		f.Close() //nolint:errcheck
		kernelScriptPath = f.Name()
	})
	return kernelScriptPath, kernelScriptErr
}

// ─── Service ──────────────────────────────────────────────────────────────────

// SyncTabContextFunc is called when a notebook kernel's session context changes.
// The function receives the tabId and each context field that needs a USE command
// (empty string means that field did not change).
type SyncTabContextFunc func(tabId, role, warehouse, database, schema string)

// Service manages Snowpark/Jupyter notebook operations.
type Service struct {
	ctx            context.Context
	syncTabContext SyncTabContextFunc
}

// NewService creates a Service. ctx is the Wails application context used for
// dialogs and event emission. syncTabContext is called when a kernel's session
// context drifts from the tab's session (may be nil if tab-session sync is
// not needed).
func NewService(ctx context.Context, syncTabContext SyncTabContextFunc) *Service {
	return &Service{ctx: ctx, syncTabContext: syncTabContext}
}

// ─── pure helpers ─────────────────────────────────────────────────────────────

// IsAppleSilicon reports whether the host is an Apple Silicon (arm64) Mac.
func IsAppleSilicon() bool {
	return runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
}

// pythonVersionAtLeast reports whether a "major.minor" version string satisfies
// the given minimum (e.g. pythonVersionAtLeast("3.13", 3, 9) → true).
// Returns false if the string cannot be parsed.
func pythonVersionAtLeast(version string, wantMajor, wantMinor int) bool {
	parts := strings.SplitN(version, ".", 2)
	if len(parts) < 2 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return major > wantMajor || (major == wantMajor && minor >= wantMinor)
}

// defaultVenvPath returns the default venv location.
// Prefers <exportDir>/snowpark_venv so the env lives next to the project files;
// falls back to ~/snowpark_venv when no export directory is configured.
func defaultVenvPath() string {
	cfg, err := config.Load()
	if err == nil && cfg.Git.ExportDir != "" {
		return filepath.Join(cfg.Git.ExportDir, "snowpark_venv")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "snowpark_venv")
}

// venvPythonBin returns the python binary path inside a venv.
func venvPythonBin(venvPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvPath, "Scripts", "python.exe")
	}
	return filepath.Join(venvPath, "bin", "python")
}

// venvPipBin returns the pip binary path inside a venv.
func venvPipBin(venvPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvPath, "Scripts", "pip.exe")
	}
	return filepath.Join(venvPath, "bin", "pip")
}

// snowparkPython returns the Python executable to use for kernel sessions,
// choosing between the conda env and the venv based on persisted config.
func snowparkPython() (string, error) {
	cfg, _ := config.Load()
	if cfg.Snowpark.Backend == "venv" {
		venvPath := cfg.Snowpark.VenvPath
		if venvPath == "" {
			venvPath = defaultVenvPath()
		}
		py := venvPythonBin(venvPath)
		if _, err := os.Stat(py); err != nil {
			return "", fmt.Errorf("venv python not found at %s — run Setup", py)
		}
		return py, nil
	}
	// conda: find the python inside the env and return its absolute path.
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		return "", fmt.Errorf("conda not found")
	}
	out, err := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv, "which", "python").Output()
	if err != nil {
		return "", fmt.Errorf("conda env '%s' python not found — run Setup", SnowparkCondaEnv)
	}
	return strings.TrimSpace(string(out)), nil
}

// detectSystemPython returns the version string (e.g. "3.11") of the first
// python3/python binary found on PATH, or "" if none is found.
func detectSystemPython() string {
	re := regexp.MustCompile(`Python (\d+\.\d+)`)
	for _, bin := range []string{"python3", "python"} {
		p, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		out, err := exec.Command(p, "--version").CombinedOutput()
		if err != nil {
			continue
		}
		if m := re.FindStringSubmatch(string(out)); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

// bpFilePath returns the companion breakpoints file path for a notebook.
func bpFilePath(notebookPath string) string {
	return notebookPath + ".thaw-bp.json"
}

// ─── pip registry helpers ──────────────────────────────────────────────────────

// pipRegistrySetup holds the pip CLI flags and env vars generated from PipRegistryConfig.
type pipRegistrySetup struct {
	Args []string // pip CLI flags to append to every install command
	Env  []string // "KEY=VALUE" environment variables (e.g. NO_PROXY)
}

// embedCredsInURL inserts user:pass@ into rawURL before the host.
// Returns rawURL unchanged when user is empty or rawURL cannot be parsed.
func embedCredsInURL(rawURL, user, pass string) string {
	if user == "" {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.User = url.UserPassword(user, pass)
	return u.String()
}

// buildProxyURL returns proxyURL with credentials embedded if proxyUser is non-empty.
func buildProxyURL(proxyURL, proxyUser, proxyPass string) string {
	return embedCredsInURL(proxyURL, proxyUser, proxyPass)
}

// findCredForRegistry returns the first credential whose Registry matches registry,
// or nil if none is found.
func findCredForRegistry(creds []config.PipRegistryCredential, registry string) *config.PipRegistryCredential {
	for i := range creds {
		if creds[i].Registry == registry {
			return &creds[i]
		}
	}
	return nil
}

// splitHosts splits a comma-separated host list and trims whitespace.
// Returns nil when s is empty.
func splitHosts(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		h := strings.TrimSpace(part)
		if h != "" {
			out = append(out, h)
		}
	}
	return out
}

// buildPipRegistrySetup converts the persisted PipRegistryConfig into concrete
// pip CLI flags and environment variables.  It is called immediately before
// every pip install invocation.
func (s *Service) buildPipRegistrySetup() pipRegistrySetup {
	appCfg, err := config.Load()
	if err != nil {
		return pipRegistrySetup{}
	}
	rc := appCfg.PipRegistry

	var args []string
	var env []string

	// Primary registry.
	if rc.PrimaryURL != "" {
		registryURL := rc.PrimaryURL
		if cred := findCredForRegistry(rc.Credentials, rc.PrimaryURL); cred != nil {
			registryURL = embedCredsInURL(rc.PrimaryURL, cred.Username, cred.Password)
		}
		if rc.Behavior == "override" {
			args = append(args, "--index-url", registryURL)
		} else {
			args = append(args, "--extra-index-url", registryURL)
		}
	}

	// Additional registries — always --extra-index-url.
	for _, reg := range rc.AdditionalRegistries {
		if reg == "" {
			continue
		}
		registryURL := reg
		if cred := findCredForRegistry(rc.Credentials, reg); cred != nil {
			registryURL = embedCredsInURL(reg, cred.Username, cred.Password)
		}
		args = append(args, "--extra-index-url", registryURL)
	}

	// Proxy.
	if rc.EnableProxy && rc.ProxyURL != "" {
		proxyURL := buildProxyURL(rc.ProxyURL, rc.ProxyUsername, rc.ProxyPassword)
		args = append(args, "--proxy", proxyURL)
		if rc.ProxyBypassHosts != "" {
			env = append(env, "NO_PROXY="+rc.ProxyBypassHosts)
		}
	}

	// Trusted hosts.
	for _, host := range splitHosts(rc.TrustedHosts) {
		args = append(args, "--trusted-host", host)
	}

	// CA certificate.
	if rc.CustomCACertPath != "" {
		args = append(args, "--cert", rc.CustomCACertPath)
	}

	return pipRegistrySetup{Args: args, Env: env}
}

// kernelRPC sends a pre-formatted request string to the kernel stdin, reads
// stdout until the sentinel, and returns the last non-sentinel line (JSON).
// The caller must already hold s.mu.
func kernelRPC(s *notebookSession, request string) (string, error) {
	if _, err := fmt.Fprint(s.stdin, request); err != nil {
		return "", fmt.Errorf("write to kernel: %w", err)
	}
	var lastLine string
	for {
		line, err := s.stdout.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read from kernel: %w", err)
		}
		line = strings.TrimRight(line, "\n\r")
		if line == kernelSentinel {
			break
		}
		lastLine = line
	}
	return lastLine, nil
}

// ─── config IPC ───────────────────────────────────────────────────────────────

// GetSnowparkConfig returns the effective Snowpark environment settings.
func (s *Service) GetSnowparkConfig() SnowparkConfigResult {
	cfg, _ := config.Load()
	backend := cfg.Snowpark.Backend
	if backend == "" {
		backend = "conda"
	}
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}
	return SnowparkConfigResult{Backend: backend, VenvPath: venvPath, PythonPath: cfg.Snowpark.PythonPath}
}

// SaveSnowparkConfig persists the backend choice.
func (s *Service) SaveSnowparkConfig(backend string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Snowpark.Backend = backend
	return config.Save(cfg)
}

// SaveSnowparkVenvPath persists the custom venv directory path.
func (s *Service) SaveSnowparkVenvPath(path string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Snowpark.VenvPath = path
	return config.Save(cfg)
}

// VenvFolderExists reports whether the configured venv directory exists on disk.
func (s *Service) VenvFolderExists() bool {
	cfg, _ := config.Load()
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}
	_, err := os.Stat(venvPath)
	return err == nil
}

// SaveSnowparkPythonPath persists the chosen Python binary path for venv creation.
func (s *Service) SaveSnowparkPythonPath(pythonPath string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Snowpark.PythonPath = pythonPath
	return config.Save(cfg)
}

// ─── pip registry IPC ─────────────────────────────────────────────────────────

// GetPipRegistryConfig returns the persisted pip registry configuration.
func (s *Service) GetPipRegistryConfig() (config.PipRegistryConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.PipRegistryConfig{}, err
	}
	return cfg.PipRegistry, nil
}

// SavePipRegistryConfig persists the pip registry configuration to disk.
func (s *Service) SavePipRegistryConfig(cfg config.PipRegistryConfig) error {
	appCfg, err := config.Load()
	if err != nil {
		return err
	}
	appCfg.PipRegistry = cfg
	return config.Save(appCfg)
}

// ResetPipRegistryConfig clears the pip registry configuration.
func (s *Service) ResetPipRegistryConfig() error {
	appCfg, err := config.Load()
	if err != nil {
		return err
	}
	appCfg.PipRegistry = config.PipRegistryConfig{}
	return config.Save(appCfg)
}

// PickCACertFile opens a file picker for certificate files and returns the chosen path.
func (s *Service) PickCACertFile() (string, error) {
	path, err := wailsruntime.OpenFileDialog(s.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select CA Certificate",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Certificate Files (*.pem, *.crt, *.cer)", Pattern: "*.pem;*.crt;*.cer"},
		},
	})
	return path, err
}

// ─── system python discovery ──────────────────────────────────────────────────

// ListSystemPythons returns all Python interpreters found on the system.
func (s *Service) ListSystemPythons() []PythonInfo {
	seen := map[string]bool{}
	var result []PythonInfo
	re := regexp.MustCompile(`Python (\d+\.\d+(?:\.\d+)?)`)

	add := func(path string) {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			real = path
		}
		if seen[real] {
			return
		}
		out, err := exec.Command(path, "--version").CombinedOutput()
		if err != nil {
			return
		}
		m := re.FindStringSubmatch(string(out))
		if len(m) < 2 {
			return
		}
		seen[real] = true
		result = append(result, PythonInfo{Path: path, Version: m[1]})
	}

	// PATH-based names.
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			add(p)
		}
	}
	for minor := 9; minor <= 15; minor++ {
		if p, err := exec.LookPath(fmt.Sprintf("python3.%d", minor)); err == nil {
			add(p)
		}
	}

	// Common installation prefixes.
	globs := []string{
		"/usr/bin/python3*",
		"/usr/local/bin/python3*",
		"/opt/homebrew/bin/python3*",
		"/opt/homebrew/opt/python*/bin/python3*",
	}
	if home, err := os.UserHomeDir(); err == nil {
		globs = append(globs,
			filepath.Join(home, ".pyenv/versions/*/bin/python3"),
			filepath.Join(home, ".pyenv/versions/*/bin/python3.*"),
		)
	}
	for _, pattern := range globs {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			add(m)
		}
	}

	return result
}

// ─── environment check ────────────────────────────────────────────────────────

// CheckSnowparkEnv inspects the local environment based on the configured backend.
func (s *Service) CheckSnowparkEnv() SnowparkCheckResult {
	result := SnowparkCheckResult{}

	// Always detect system Python regardless of backend.
	result.SystemPythonVersion = detectSystemPython()

	cfg, _ := config.Load()
	backend := cfg.Snowpark.Backend
	if backend == "" {
		backend = "conda"
	}
	result.Backend = backend

	if backend == "venv" {
		return s.checkVenvEnv(&result, cfg)
	}
	return s.checkCondaEnv(&result)
}

func (s *Service) checkCondaEnv(result *SnowparkCheckResult) SnowparkCheckResult {
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		result.Details = "conda not found. Install Miniconda or Anaconda and ensure it is on your PATH."
		return *result
	}
	result.HasConda = true

	listOut, err := exec.Command(condaPath, "env", "list").Output()
	if err != nil {
		result.Details = "Failed to list conda environments."
		return *result
	}
	if !strings.Contains(string(listOut), SnowparkCondaEnv) {
		result.Details = fmt.Sprintf("Conda environment '%s' not found. Run Snowpark > Setup Environment to create it.", SnowparkCondaEnv)
		return *result
	}
	result.HasEnv = true

	verOut, _ := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv, "python", "--version").CombinedOutput()
	re := regexp.MustCompile(`Python (\d+\.\d+)`)
	if m := re.FindStringSubmatch(string(verOut)); len(m) >= 2 {
		result.Version = m[1]
	}
	if result.Version != "" && !pythonVersionAtLeast(result.Version, 3, 9) {
		result.Details = fmt.Sprintf("Python %s in the env is too old (need 3.9+).", result.Version)
		return *result
	}

	if err := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv,
		"python", "-c", "import snowflake.snowpark").Run(); err != nil {
		result.Details = "snowflake-snowpark-python not installed. Run Snowpark > Setup Environment."
		return *result
	}
	result.HasSnowpark = true

	if err := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv,
		"python", "-c", "import notebook").Run(); err != nil {
		result.Details = "notebook package not installed. Run Snowpark > Setup Environment."
		return *result
	}
	result.HasNotebook = true

	pyPath, _ := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv, "which", "python").Output()
	result.PythonPath = strings.TrimSpace(string(pyPath))
	result.IsReady = true
	result.Details = "Environment is fully configured."
	return *result
}

func (s *Service) checkVenvEnv(result *SnowparkCheckResult, cfg *config.AppConfig) SnowparkCheckResult {
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}
	result.VenvPath = venvPath

	python := venvPythonBin(venvPath)
	if _, err := os.Stat(python); err != nil {
		result.Details = fmt.Sprintf("venv not found at %s. Run Snowpark > Setup Environment.", venvPath)
		return *result
	}
	result.HasVenv = true

	verOut, _ := exec.Command(python, "--version").CombinedOutput()
	re := regexp.MustCompile(`Python (\d+\.\d+)`)
	if m := re.FindStringSubmatch(string(verOut)); len(m) >= 2 {
		result.Version = m[1]
	}
	if result.Version != "" && !pythonVersionAtLeast(result.Version, 3, 9) {
		result.Details = fmt.Sprintf("Python %s in the venv is too old (need 3.9+).", result.Version)
		return *result
	}

	if err := exec.Command(python, "-c", "import snowflake.snowpark").Run(); err != nil {
		result.Details = "snowflake-snowpark-python not installed. Run Snowpark > Setup Environment."
		return *result
	}
	result.HasSnowpark = true

	if err := exec.Command(python, "-c", "import notebook").Run(); err != nil {
		result.Details = "notebook package not installed. Run Snowpark > Setup Environment."
		return *result
	}
	result.HasNotebook = true

	result.PythonPath = python
	result.IsReady = true
	result.Details = "Environment is fully configured."
	return *result
}

// ─── command streaming ────────────────────────────────────────────────────────

// streamCommandTo runs cmd and emits each output line as the given event.
func (s *Service) streamCommandTo(cmd *exec.Cmd, eventName string) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	emit := func(line string) {
		wailsruntime.EventsEmit(s.ctx, eventName, line)
	}

	var wg sync.WaitGroup
	scanPipe := func(r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			emit(sc.Text())
		}
	}
	wg.Add(2)
	go scanPipe(stdout)
	go scanPipe(stderr)
	wg.Wait()
	return cmd.Wait()
}

// streamCommand runs cmd and emits each output line as a "snowpark:install-output" event.
func (s *Service) streamCommand(cmd *exec.Cmd) error {
	return s.streamCommandTo(cmd, "snowpark:install-output")
}

// ─── pip/env helpers ──────────────────────────────────────────────────────────

// pipBinForEnv returns the pip binary path for the active backend environment.
func (s *Service) pipBinForEnv() (string, error) {
	cfg, _ := config.Load()
	backend := cfg.Snowpark.Backend
	if backend == "" {
		backend = "conda"
	}
	if backend == "venv" {
		venvPath := cfg.Snowpark.VenvPath
		if venvPath == "" {
			venvPath = defaultVenvPath()
		}
		pip := venvPipBin(venvPath)
		if _, err := os.Stat(pip); err != nil {
			return "", fmt.Errorf("venv pip not found at %s — run Setup first", pip)
		}
		return pip, nil
	}
	// conda: use "conda run -n thaw_snowpark pip"
	return "", nil
}

// ListEnvPackages returns all packages installed in the active Snowpark environment.
func (s *Service) ListEnvPackages() ([]PackageInfo, error) {
	cfg, _ := config.Load()
	backend := cfg.Snowpark.Backend
	if backend == "" {
		backend = "conda"
	}

	var cmd *exec.Cmd
	if backend == "venv" {
		pip, err := s.pipBinForEnv()
		if err != nil {
			return nil, err
		}
		cmd = exec.Command(pip, "list", "--format=json")
	} else {
		condaPath, err := exec.LookPath("conda")
		if err != nil {
			return nil, fmt.Errorf("conda not found: %w", err)
		}
		cmd = exec.Command(condaPath, "run", "-n", SnowparkCondaEnv,
			"pip", "list", "--format=json")
	}

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pip list: %w", err)
	}

	var raw []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse pip list: %w", err)
	}

	pkgs := make([]PackageInfo, 0, len(raw))
	for _, r := range raw {
		pkgs = append(pkgs, PackageInfo{Name: r.Name, Version: r.Version})
	}
	return pkgs, nil
}

// InstallEnvPackage installs a single Python package in the active environment,
// streaming output via the "snowpark:package-output" event.
func (s *Service) InstallEnvPackage(pkg string) error {
	setup := s.buildPipRegistrySetup()
	args := append([]string{"install", pkg}, setup.Args...)
	cmd, err := s.pipCmd(args...)
	if err != nil {
		return err
	}
	if len(setup.Env) > 0 {
		cmd.Env = append(os.Environ(), setup.Env...)
	}
	if err := s.streamCommandTo(cmd, "snowpark:package-output"); err != nil {
		return fmt.Errorf("install %s failed: %w", pkg, err)
	}
	return nil
}

// UninstallEnvPackage removes a Python package from the active environment,
// streaming output via the "snowpark:package-output" event.
func (s *Service) UninstallEnvPackage(pkg string) error {
	cmd, err := s.pipCmd("uninstall", "-y", pkg)
	if err != nil {
		return err
	}
	if err := s.streamCommandTo(cmd, "snowpark:package-output"); err != nil {
		return fmt.Errorf("uninstall %s failed: %w", pkg, err)
	}
	return nil
}

// ─── dependency-file operations ────────────────────────────────────────────────

// pipCmd builds an *exec.Cmd that runs pip with the given args against the
// active backend — the venv pip binary, or "conda run -n <env> pip" for conda.
func (s *Service) pipCmd(args ...string) (*exec.Cmd, error) {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = &config.AppConfig{}
	}
	backend := cfg.Snowpark.Backend
	if backend == "" {
		backend = "conda"
	}
	if backend == "venv" {
		pip, err := s.pipBinForEnv()
		if err != nil {
			return nil, err
		}
		return exec.Command(pip, args...), nil
	}
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		return nil, fmt.Errorf("conda not found: %w", err)
	}
	full := append([]string{"run", "-n", SnowparkCondaEnv, "pip"}, args...)
	return exec.Command(condaPath, full...), nil
}

// PickRequirementsFile opens a file dialog to select a pip requirements file.
func (s *Service) PickRequirementsFile() (string, error) {
	return wailsruntime.OpenFileDialog(s.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select requirements file",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Requirements (*.txt)", Pattern: "*.txt"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
}

// PickPyprojectFile opens a file dialog to select a pyproject.toml / TOML file.
func (s *Service) PickPyprojectFile() (string, error) {
	return wailsruntime.OpenFileDialog(s.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select pyproject.toml",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "TOML (*.toml)", Pattern: "*.toml"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
}

// InstallRequirementsFile installs every package listed in a pip requirements
// file (`pip install -r <path>`), applying the pip registry configuration and
// streaming output via the "snowpark:package-output" event.
func (s *Service) InstallRequirementsFile(path string) error {
	if path == "" {
		return fmt.Errorf("no requirements file selected")
	}
	setup := s.buildPipRegistrySetup()
	args := append([]string{"install", "-r", path}, setup.Args...)
	cmd, err := s.pipCmd(args...)
	if err != nil {
		return err
	}
	if len(setup.Env) > 0 {
		cmd.Env = append(os.Environ(), setup.Env...)
	}
	if err := s.streamCommandTo(cmd, "snowpark:package-output"); err != nil {
		return fmt.Errorf("install from %s failed: %w", filepath.Base(path), err)
	}
	return nil
}

// InstallPyprojectFile installs the project defined by a pyproject.toml (or any
// TOML build file) by running `pip install <dir>` against the directory that
// contains it, applying the pip registry configuration and streaming output.
func (s *Service) InstallPyprojectFile(path string) error {
	if path == "" {
		return fmt.Errorf("no pyproject.toml selected")
	}
	dir := filepath.Dir(path)
	setup := s.buildPipRegistrySetup()
	args := append([]string{"install", dir}, setup.Args...)
	cmd, err := s.pipCmd(args...)
	if err != nil {
		return err
	}
	if len(setup.Env) > 0 {
		cmd.Env = append(os.Environ(), setup.Env...)
	}
	if err := s.streamCommandTo(cmd, "snowpark:package-output"); err != nil {
		return fmt.Errorf("install from %s failed: %w", filepath.Base(path), err)
	}
	return nil
}

// FreezeRequirements writes `pip freeze` output to a requirements file. When
// path is empty it opens a save dialog; it returns the path written, or "" if
// the dialog was cancelled.
func (s *Service) FreezeRequirements(path string) (string, error) {
	if path == "" {
		chosen, err := wailsruntime.SaveFileDialog(s.ctx, wailsruntime.SaveDialogOptions{
			Title:           "Save requirements.txt",
			DefaultFilename: "requirements.txt",
			Filters: []wailsruntime.FileFilter{
				{DisplayName: "Requirements (*.txt)", Pattern: "*.txt"},
			},
		})
		if err != nil {
			return "", err
		}
		if chosen == "" {
			return "", nil // cancelled
		}
		path = chosen
	}
	cmd, err := s.pipCmd("freeze")
	if err != nil {
		return "", err
	}
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("pip freeze: %w\n%s", err, exitErr.Stderr)
		}
		return "", fmt.Errorf("pip freeze: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// ─── conda install methods ────────────────────────────────────────────────────

// InstallCondaEnv creates the thaw_snowpark conda environment.
// On Apple Silicon the CONDA_SUBDIR=osx-64 workaround is applied automatically.
func (s *Service) InstallCondaEnv() error {
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		return fmt.Errorf("conda not found: %w", err)
	}

	// Skip if already exists.
	if out, _ := exec.Command(condaPath, "env", "list").Output(); strings.Contains(string(out), SnowparkCondaEnv) {
		wailsruntime.EventsEmit(s.ctx, "snowpark:install-output",
			fmt.Sprintf("Conda environment '%s' already exists, skipping creation.", SnowparkCondaEnv))
		return nil
	}

	args := []string{
		"create", "--name", SnowparkCondaEnv, "-y",
		"--override-channels",
		"-c", "https://repo.anaconda.com/pkgs/snowflake",
		"python=3.12", "numpy", "pandas", "pyarrow",
	}
	cmd := exec.Command(condaPath, args...)
	if IsAppleSilicon() {
		// Apple M-series: force x86_64 to avoid pyOpenSSL ffi.callback crash.
		cmd.Env = append(os.Environ(), "CONDA_SUBDIR=osx-64")
		wailsruntime.EventsEmit(s.ctx, "snowpark:install-output",
			"Apple Silicon detected — creating x86_64 environment (CONDA_SUBDIR=osx-64).")
	}
	if err := s.streamCommand(cmd); err != nil {
		return fmt.Errorf("conda create failed: %w", err)
	}

	// Pin subdir for future installs on Apple Silicon.
	if IsAppleSilicon() {
		cfgCmd := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv,
			"conda", "config", "--env", "--set", "subdir", "osx-64")
		if e := s.streamCommand(cfgCmd); e != nil {
			wailsruntime.EventsEmit(s.ctx, "snowpark:install-output",
				"[warn] conda config --env --set subdir osx-64: "+e.Error())
		}
	}
	return nil
}

// InstallSnowparkPackage installs snowflake-snowpark-python (plus pandas/pyarrow)
// into the thaw_snowpark conda environment.
func (s *Service) InstallSnowparkPackage() error {
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		return fmt.Errorf("conda not found: %w", err)
	}
	cmd := exec.Command(condaPath, "install", "-n", SnowparkCondaEnv, "-y",
		"snowflake-snowpark-python", "pandas", "pyarrow")
	if err := s.streamCommand(cmd); err != nil {
		return fmt.Errorf("snowpark install failed: %w", err)
	}
	return nil
}

// InstallJupyterNotebook installs notebook, ipython-sql, sqlalchemy and pyflakes via pip.
func (s *Service) InstallJupyterNotebook() error {
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		return fmt.Errorf("conda not found: %w", err)
	}
	setup := s.buildPipRegistrySetup()
	baseArgs := []string{"run", "-n", SnowparkCondaEnv, "pip", "install", "notebook", "ipython-sql", "sqlalchemy", "pyflakes", "debugpy", "cryptography<44.0.0"}
	args := append(baseArgs, setup.Args...)
	cmd := exec.Command(condaPath, args...)
	if len(setup.Env) > 0 {
		cmd.Env = append(os.Environ(), setup.Env...)
	}
	if err := s.streamCommand(cmd); err != nil {
		return fmt.Errorf("notebook install failed: %w", err)
	}
	return nil
}

// ─── venv install methods ─────────────────────────────────────────────────────

// InstallVenvEnv creates a Python venv at the configured path using system python3.
func (s *Service) InstallVenvEnv() error {
	cfg, _ := config.Load()
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}

	// Skip if already exists.
	if _, err := os.Stat(venvPythonBin(venvPath)); err == nil {
		wailsruntime.EventsEmit(s.ctx, "snowpark:install-output",
			fmt.Sprintf("Virtual environment already exists at %s, skipping.", venvPath))
		return nil
	}

	python := cfg.Snowpark.PythonPath
	if python == "" {
		var err error
		python, err = exec.LookPath("python3")
		if err != nil {
			python, err = exec.LookPath("python")
			if err != nil {
				return fmt.Errorf("python3 not found on PATH: %w", err)
			}
		}
	}

	wailsruntime.EventsEmit(s.ctx, "snowpark:install-output",
		fmt.Sprintf("Creating venv at %s…", venvPath))
	if err := os.MkdirAll(filepath.Dir(venvPath), 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	cmd := exec.Command(python, "-m", "venv", venvPath)
	if err := s.streamCommand(cmd); err != nil {
		return fmt.Errorf("venv create failed: %w", err)
	}
	return nil
}

// InstallSnowparkVenv installs snowflake-snowpark-python into the venv.
// Pass withPandas=true for the [pandas] extras variant.
func (s *Service) InstallSnowparkVenv(withPandas bool) error {
	cfg, _ := config.Load()
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}
	pkg := "snowflake-snowpark-python"
	if withPandas {
		pkg = "snowflake-snowpark-python[pandas]"
	}
	setup := s.buildPipRegistrySetup()
	args := append([]string{"install", pkg}, setup.Args...)
	cmd := exec.Command(venvPipBin(venvPath), args...)
	if len(setup.Env) > 0 {
		cmd.Env = append(os.Environ(), setup.Env...)
	}
	if err := s.streamCommand(cmd); err != nil {
		return fmt.Errorf("snowpark venv install failed: %w", err)
	}
	return nil
}

// DeleteVenvFolder removes the venv directory at the configured path.
func (s *Service) DeleteVenvFolder() error {
	cfg, _ := config.Load()
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}
	if _, err := os.Stat(venvPath); os.IsNotExist(err) {
		return nil // already gone
	}
	return os.RemoveAll(venvPath)
}

// InstallJupyterVenv installs notebook, ipython-sql, sqlalchemy and pyflakes into the venv.
func (s *Service) InstallJupyterVenv() error {
	cfg, _ := config.Load()
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}
	setup := s.buildPipRegistrySetup()
	baseArgs := []string{"install", "notebook", "ipython-sql", "sqlalchemy", "pyflakes", "debugpy", "cryptography<44.0.0"}
	args := append(baseArgs, setup.Args...)
	cmd := exec.Command(venvPipBin(venvPath), args...)
	if len(setup.Env) > 0 {
		cmd.Env = append(os.Environ(), setup.Env...)
	}
	if err := s.streamCommand(cmd); err != nil {
		return fmt.Errorf("notebook venv install failed: %w", err)
	}
	return nil
}

// ─── notebook CRUD ────────────────────────────────────────────────────────────

// minimalNotebook is the nbformat v4 JSON written when creating a new notebook.
const minimalNotebook = `{
 "nbformat": 4,
 "nbformat_minor": 5,
 "metadata": {
  "kernelspec": {"display_name": "Python 3", "language": "python", "name": "python3"},
  "language_info": {"name": "python", "version": "3.12.0"}
 },
 "cells": [
  {
   "cell_type": "code",
   "execution_count": null,
   "metadata": {},
   "outputs": [],
   "source": []
  }
 ]
}
`

// NewNotebook shows a save dialog, writes a blank notebook, and returns the path and content.
func (s *Service) NewNotebook() (string, error) {
	path, err := wailsruntime.SaveFileDialog(s.ctx, wailsruntime.SaveDialogOptions{
		Title:           "New Notebook",
		DefaultFilename: "notebook.ipynb",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Jupyter Notebooks (*.ipynb)", Pattern: "*.ipynb"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	// Ensure .ipynb extension.
	if filepath.Ext(path) != ".ipynb" {
		path += ".ipynb"
	}
	if err := os.WriteFile(path, []byte(minimalNotebook), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// PickNotebookFile opens a file picker for .ipynb files and returns the chosen path.
func (s *Service) PickNotebookFile() (string, error) {
	path, err := wailsruntime.OpenFileDialog(s.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open Notebook",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Jupyter Notebooks (*.ipynb)", Pattern: "*.ipynb"},
		},
	})
	return path, err
}

// ReadNotebook reads an .ipynb file and returns its raw JSON content.
func (s *Service) ReadNotebook(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveNotebook writes notebook JSON to the given path.
func (s *Service) SaveNotebook(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

// SaveNotebookBreakpoints persists breakpoints (cellId → sorted line numbers)
// to a companion file next to the notebook. An empty map deletes the file.
func (s *Service) SaveNotebookBreakpoints(notebookPath string, bps map[string][]int) error {
	p := bpFilePath(notebookPath)
	if len(bps) == 0 {
		_ = os.Remove(p) // best-effort delete when no breakpoints remain
		return nil
	}
	data, err := json.Marshal(bps)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// LoadNotebookBreakpoints reads the companion breakpoints file for a notebook.
// Returns an empty map (no error) when the file does not exist.
func (s *Service) LoadNotebookBreakpoints(notebookPath string) (map[string][]int, error) {
	data, err := os.ReadFile(bpFilePath(notebookPath))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]int{}, nil
		}
		return nil, err
	}
	var bps map[string][]int
	if err := json.Unmarshal(data, &bps); err != nil {
		return map[string][]int{}, nil // corrupt file → treat as empty
	}
	return bps, nil
}

// ─── notebook SQL execution ───────────────────────────────────────────────────

// RunNotebookSql executes a SQL query via the provided Snowflake client and
// returns the result as columns + rows, suitable for table display in a notebook.
func (s *Service) RunNotebookSql(client *snowflake.Client, sql string) (NotebookSqlResult, error) {
	if client == nil {
		return NotebookSqlResult{}, apperrors.ErrNotConnected
	}
	result, err := client.Execute(s.ctx, sql)
	if err != nil {
		return NotebookSqlResult{}, err
	}
	rows := result.Rows
	if rows == nil {
		rows = [][]any{}
	}
	return NotebookSqlResult{
		Columns:   result.Columns,
		Rows:      rows,
		RowCount:  result.RowsAffected,
		QueryID:   result.QueryID,
		Truncated: result.Truncated,
	}, nil
}

// ─── kernel environment ───────────────────────────────────────────────────────

// notebookKernelEnv returns the THAW_SF_* environment variables to inject into
// the kernel subprocess so it can auto-create a matching Snowpark session.
// Falls back to nil (no extra vars) when not connected or params are unavailable.
func (s *Service) notebookKernelEnv(client *snowflake.Client, connectParams *snowflake.ConnectParams) []string {
	if connectParams == nil || client == nil {
		return nil
	}
	p := connectParams
	// Fetch the live session state — the user may have switched role / warehouse
	// / database / schema since connecting.
	ctx, err := client.GetSessionContext(s.ctx)
	if err != nil {
		// Fall back to original params if the query fails.
		ctx.Role = p.Role
		ctx.Warehouse = p.Warehouse
		ctx.Database = p.Database
		ctx.Schema = p.Schema
	}
	return []string{
		"THAW_SF_ACCOUNT=" + p.Account,
		"THAW_SF_USER=" + p.User,
		"THAW_SF_PASSWORD=" + p.Password,
		"THAW_SF_AUTHENTICATOR=" + p.Authenticator,
		"THAW_SF_OKTA_URL=" + p.OktaURL,
		"THAW_SF_PRIVATE_KEY_PATH=" + p.PrivateKeyPath,
		"THAW_SF_PRIVATE_KEY_PASSPHRASE=" + p.PrivateKeyPassphrase,
		"THAW_SF_ROLE=" + ctx.Role,
		"THAW_SF_WAREHOUSE=" + ctx.Warehouse,
		"THAW_SF_DATABASE=" + ctx.Database,
		"THAW_SF_SCHEMA=" + ctx.Schema,
	}
}

// ─── kernel session lifecycle ─────────────────────────────────────────────────

// StartNotebookSession launches the Python kernel subprocess for a notebook tab.
// Safe to call multiple times — returns immediately if a session already exists.
func (s *Service) StartNotebookSession(client *snowflake.Client, connectParams *snowflake.ConnectParams, tabId string) error {
	if _, ok := notebookSessions.Load(tabId); ok {
		return nil
	}

	scriptPath, err := ensureKernelScript()
	if err != nil {
		return fmt.Errorf("kernel script: %w", err)
	}

	python, err := snowparkPython()
	if err != nil {
		return err
	}

	// Detect the Python version for the kernel indicator.
	pyVer := detectPythonVersion(python)

	cmd := exec.Command(python, "-u", scriptPath)
	// Inject connection parameters so the kernel can auto-create a Snowpark
	// session that matches the app's active connection.
	if extra := s.notebookKernelEnv(client, connectParams); len(extra) > 0 {
		cmd.Env = append(os.Environ(), extra...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	// Redirect stderr to /dev/null to avoid cluttering (kernel errors go through JSON).
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("kernel start: %w", err)
	}

	// Capture the current main-connection context so we can detect drift later.
	var initCtx NotebookSessionContext
	if client != nil {
		if sc, err := client.GetSessionContext(s.ctx); err == nil {
			initCtx = NotebookSessionContext{
				Role:      sc.Role,
				Warehouse: sc.Warehouse,
				Database:  sc.Database,
				Schema:    sc.Schema,
			}
		}
	}

	notebookSessions.Store(tabId, &notebookSession{
		cmd:           cmd,
		stdin:         stdin,
		stdout:        bufio.NewReader(stdout),
		lastCtx:       initCtx,
		pythonVersion: pyVer,
	})
	return nil
}

// GetKernelPythonVersion returns the Python version string (e.g. "3.11.9")
// for the kernel running in the given tab, or "" if unknown.
func (s *Service) GetKernelPythonVersion(tabId string) string {
	val, ok := notebookSessions.Load(tabId)
	if !ok {
		return ""
	}
	return val.(*notebookSession).pythonVersion
}

// detectPythonVersion runs `python --version` and extracts the version number.
// Returns "" on any error so callers can fall back gracefully.
func detectPythonVersion(pythonBin string) string {
	out, err := exec.Command(pythonBin, "--version").Output()
	if err != nil {
		return ""
	}
	// Output is "Python 3.11.9\n" — extract the version part.
	s := strings.TrimSpace(string(out))
	if after, ok := strings.CutPrefix(s, "Python "); ok {
		return after
	}
	return s
}

// RunNotebookCell sends code to the kernel and returns its output.
// StartNotebookSession must have been called for this tabId first.
func (s *Service) RunNotebookCell(tabId string, cellId string, code string) (NotebookCellOutput, error) {
	val, ok := notebookSessions.Load(tabId)
	if !ok {
		return NotebookCellOutput{}, fmt.Errorf("no kernel for tab %s — call StartNotebookSession first", tabId)
	}
	sess := val.(*notebookSession)
	sess.mu.Lock()
	defer sess.mu.Unlock()

	// Write code block + run marker, prefixed with the exec marker + cellId so
	// the kernel can write the code to a physical temp file.  This gives any
	// functions defined in this cell a real co_filename that debugpy can
	// navigate to when stepping in from a later debug session.
	payload := fmt.Sprintf("%s\n%s\n%s\n%s\n", kernelExecMarker, cellId, code, kernelRunMarker)
	if _, err := fmt.Fprint(sess.stdin, payload); err != nil {
		return NotebookCellOutput{}, fmt.Errorf("write to kernel: %w", err)
	}

	// Read lines until the sentinel appears; the line just before it is JSON.
	var lastJSON string
	for {
		line, err := sess.stdout.ReadString('\n')
		if err != nil {
			return NotebookCellOutput{}, fmt.Errorf("read from kernel: %w", err)
		}
		line = strings.TrimRight(line, "\n\r")
		if line == kernelSentinel {
			break
		}
		lastJSON = line
	}

	var out NotebookCellOutput
	if lastJSON != "" {
		if err := json.Unmarshal([]byte(lastJSON), &out); err != nil {
			out.Stdout = lastJSON
		}
	}

	// Sync session context changes back to the tab's isolated session.
	s.syncKernelContext(tabId, sess, out.SessionContext)

	return out, nil
}

// ─── DAP Proxy Methods ────────────────────────────────────────────────────────

// StartDapProxy connects to debugpy and starts shuttling messages.
func (s *Service) StartDapProxy() error {
	dapMutex.Lock()
	defer dapMutex.Unlock()

	// Add retry loop to guarantee connection succeeds even if python is slow
	var conn net.Conn
	var err error
	for i := 0; i < 20; i++ {
		conn, err = net.Dial("tcp", "127.0.0.1:5678")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to debugpy: %w", err)
	}
	dapConn = conn

	// CRITICAL: Wrap inside sync.Once to prevent duplicate listeners/memory leaks!
	dapProxyOnce.Do(func() {
		wailsruntime.EventsOn(s.ctx, "dap:client-to-backend", func(optionalData ...interface{}) {
			dapMutex.Lock()
			defer dapMutex.Unlock()
			if len(optionalData) > 0 {
				if msg, ok := optionalData[0].(string); ok {
					if dapConn != nil {
						_, _ = dapConn.Write([]byte(msg))
					}
				}
			}
		})
	})

	// Listen for DAP messages from Python, send them to React
	go func() {
		buf := make([]byte, 8192)
		for dapConn != nil {

			n, err := dapConn.Read(buf)
			if err != nil {
				wailsruntime.EventsEmit(s.ctx, "dap:disconnected", err.Error())
				break
			}
			// Encode to Base64 to guarantee that binary data/multi-byte chars are never corrupted!
			wailsruntime.EventsEmit(s.ctx, "dap:backend-to-client", base64.StdEncoding.EncodeToString(buf[:n]))
		}
	}()

	return nil
}

// StopDapProxy closes the connection when debugging is done
func (s *Service) StopDapProxy() {
	dapMutex.Lock()
	defer dapMutex.Unlock()
	if dapConn != nil {
		_ = dapConn.Close()
		dapConn = nil
	}
}

// DebugNotebookCell executes the cell using debugpy and shuttles the output back to React
func (s *Service) DebugNotebookCell(tabId string, cellId string, code string) (NotebookCellOutput, error) {
	val, ok := notebookSessions.Load(tabId)
	if !ok {
		return NotebookCellOutput{}, fmt.Errorf("no kernel running")
	}
	sess := val.(*notebookSession)
	sess.mu.Lock()
	defer sess.mu.Unlock()

	// 1. Send the debug command to Python
	payload := fmt.Sprintf("%s\n%s\n%s\n%s\n", kernelDebugMarker, cellId, code, kernelRunMarker)

	if _, err := fmt.Fprint(sess.stdin, payload); err != nil {
		return NotebookCellOutput{}, err
	}

	// 2. Read the "DEBUG_READY" response
	for {
		line, err := sess.stdout.ReadString('\n')
		if err != nil {
			return NotebookCellOutput{}, err
		}
		line = strings.TrimRight(line, "\n\r")

		if line == kernelSentinel {
			break // Setup sentinel reached
		}

		if strings.Contains(line, "DEBUG_READY") {
			// Parse the JSON to get the physical filepath Python created
			var payload struct {
				Filepath string `json:"filepath"`
			}
			_ = json.Unmarshal([]byte(line), &payload)

			// 1. Tell React what the filepath is so it can set breakpoints
			wailsruntime.EventsEmit(s.ctx, "dap:debug-ready", map[string]string{
				"filepath": payload.Filepath,
			})

			// 2. Start the TCP Proxy in a goroutine and tell React it's open (or failed!)
			go func() {
				if err := s.StartDapProxy(); err != nil {
					wailsruntime.EventsEmit(s.ctx, "dap:proxy-ready", err.Error())
				} else {
					wailsruntime.EventsEmit(s.ctx, "dap:proxy-ready", "")
				}
			}()
		} else if strings.Contains(line, "error") {
			// Handle failure to start debugpy
			var out NotebookCellOutput
			_ = json.Unmarshal([]byte(line), &out)
			return out, nil
		}
	}

	// 4. Now wait for the ACTUAL execution result (after the DAP client resumes the debugger)
	var lastJSON string
	for {
		line, err := sess.stdout.ReadString('\n')
		if err != nil {
			return NotebookCellOutput{}, fmt.Errorf("read from kernel: %w", err)
		}
		line = strings.TrimRight(line, "\n\r")
		if line == kernelSentinel {
			break
		}
		if strings.HasPrefix(line, kernelDebugResultMarker) {
			// Prefixed result JSON — extract and store for parsing below.
			lastJSON = strings.TrimPrefix(line, kernelDebugResultMarker)
		} else if line != "" {
			// Non-empty, non-sentinel, non-result line → real-time user stdout.
			wailsruntime.EventsEmit(s.ctx, "notebook:debug:output", line)
		}
	}

	// 5. Clean up the DAP proxy after execution is finished
	s.StopDapProxy()

	var out NotebookCellOutput
	if lastJSON != "" {
		if err := json.Unmarshal([]byte(lastJSON), &out); err != nil {
			out.Stdout = lastJSON
		}
	}

	s.syncKernelContext(tabId, sess, out.SessionContext)

	return out, nil
}

// ─── kernel context sync ──────────────────────────────────────────────────────

// syncKernelContext compares a newly-returned kernel context against the last
// known context stored in sess, applies any USE commands to the tab's isolated
// session via the syncTabContext callback, emits a context-changed event when
// something changed, and updates sess.lastCtx.  The caller must hold sess.mu.
func (s *Service) syncKernelContext(tabId string, sess *notebookSession, ctx *NotebookSessionContext) {
	if ctx == nil {
		return
	}
	// strip removes Snowflake-style surrounding double-quotes from a returned
	// identifier value, e.g. `"Oddly Named Schema!"` → `Oddly Named Schema!`.
	// Used for comparison only.
	strip := func(v string) string { return strings.Trim(v, `"`) }

	newCtx := NotebookSessionContext{
		Role:      strip(ctx.Role),
		Warehouse: strip(ctx.Warehouse),
		Database:  strip(ctx.Database),
		Schema:    strip(ctx.Schema),
	}
	oldCtx := NotebookSessionContext{
		Role:      strip(sess.lastCtx.Role),
		Warehouse: strip(sess.lastCtx.Warehouse),
		Database:  strip(sess.lastCtx.Database),
		Schema:    strip(sess.lastCtx.Schema),
	}
	if newCtx != oldCtx {
		if s.syncTabContext != nil {
			role, wh, db, schema := "", "", "", ""
			if newCtx.Role != oldCtx.Role && newCtx.Role != "" {
				role = newCtx.Role
			}
			if newCtx.Warehouse != oldCtx.Warehouse && newCtx.Warehouse != "" {
				wh = newCtx.Warehouse
			}
			if newCtx.Database != oldCtx.Database && newCtx.Database != "" {
				db = newCtx.Database
			}
			if newCtx.Schema != oldCtx.Schema && newCtx.Schema != "" {
				schema = newCtx.Schema
			}
			s.syncTabContext(tabId, role, wh, db, schema)
		}
		sess.lastCtx = newCtx
		wailsruntime.EventsEmit(s.ctx, "notebook:session:context:changed", newCtx)
	} else {
		sess.lastCtx = newCtx
	}
}

// ─── kernel SQL cell execution ────────────────────────────────────────────────

// RunNotebookCellSql executes SQL via the active Snowpark kernel session for the
// given notebook tab so that SQL cells share context with Python cells (USE,
// role/warehouse changes, etc.).  Falls back to the main connection when no
// kernel is running.
func (s *Service) RunNotebookCellSql(client *snowflake.Client, tabId, sql string) (NotebookSqlResult, error) {
	val, ok := notebookSessions.Load(tabId)
	if !ok {
		// No kernel — fall back to main connection.
		return s.RunNotebookSql(client, sql)
	}
	sess := val.(*notebookSession)
	sess.mu.Lock()
	defer sess.mu.Unlock()

	payload := fmt.Sprintf("%s\n%s\n%s\n", kernelSqlMarker, sql, kernelRunMarker)
	resultJSON, err := kernelRPC(sess, payload)
	if err != nil {
		return NotebookSqlResult{}, err
	}

	var raw struct {
		Columns        []string                `json:"columns"`
		Rows           [][]any                 `json:"rows"`
		RowCount       int64                   `json:"rowCount"`
		Error          *string                 `json:"error"`
		QueryID        string                  `json:"queryID"`
		SessionContext *NotebookSessionContext `json:"session_context"`
		Truncated      bool                    `json:"truncated"`
	}
	if resultJSON != "" {
		if err := json.Unmarshal([]byte(resultJSON), &raw); err != nil {
			return NotebookSqlResult{}, fmt.Errorf("kernel SQL result: %w", err)
		}
	}
	if raw.Error != nil && *raw.Error != "" {
		return NotebookSqlResult{}, fmt.Errorf("%s", *raw.Error)
	}

	s.syncKernelContext(tabId, sess, raw.SessionContext)

	rows := raw.Rows
	if rows == nil {
		rows = [][]any{}
	}
	return NotebookSqlResult{
		Columns:   raw.Columns,
		Rows:      rows,
		RowCount:  raw.RowCount,
		QueryID:   raw.QueryID,
		Truncated: raw.Truncated,
	}, nil
}

// NotebookUseContext sends USE statements to the running Snowpark kernel for a
// notebook tab so the session matches the tab's role/warehouse/database/schema.
// Returns nil immediately if no kernel is running for tabId or all params are empty.
func (s *Service) NotebookUseContext(tabId, role, warehouse, database, schema string) error {
	if role == "" && warehouse == "" && database == "" && schema == "" {
		return nil
	}
	val, ok := notebookSessions.Load(tabId)
	if !ok {
		return nil // no kernel running — silently ignore
	}
	sess := val.(*notebookSession)

	escape := func(v string) string {
		return strings.ReplaceAll(v, "'", "\\'")
	}

	lines := []string{
		"try:",
		"    from snowflake.snowpark.context import get_active_session as _gas",
		"    _s = _gas()",
	}
	if role != "" {
		lines = append(lines, fmt.Sprintf("    _s.use_role('%s')", escape(role)))
	}
	if warehouse != "" {
		lines = append(lines, fmt.Sprintf("    _s.use_warehouse('%s')", escape(warehouse)))
	}
	if database != "" {
		lines = append(lines, fmt.Sprintf("    _s.use_database('%s')", escape(database)))
	}
	if schema != "" {
		lines = append(lines, fmt.Sprintf("    _s.use_schema('%s')", escape(schema)))
	}
	lines = append(lines, "except Exception:", "    pass")
	code := strings.Join(lines, "\n")

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if _, err := fmt.Fprintf(sess.stdin, "%s\n%s\n", code, kernelRunMarker); err != nil {
		return fmt.Errorf("write to kernel: %w", err)
	}

	// Drain stdout until the sentinel (discard output).
	for {
		line, err := sess.stdout.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read from kernel: %w", err)
		}
		if strings.TrimRight(line, "\n\r") == kernelSentinel {
			break
		}
	}
	return nil
}

// ─── kernel intellisense ──────────────────────────────────────────────────────

// GetNotebookCompletions queries jedi completions from the running kernel for
// the given Python source at cursor position (line, col).  line is 1-indexed
// and col is 0-indexed, matching jedi's convention.  Returns nil without error
// when no kernel is running for the tab (e.g. kernel not yet started).
func (s *Service) GetNotebookCompletions(tabId, code string, line, col int) ([]NotebookCompletion, error) {
	val, ok := notebookSessions.Load(tabId)
	if !ok {
		return nil, nil
	}
	sess := val.(*notebookSession)
	sess.mu.Lock()
	defer sess.mu.Unlock()

	req, _ := json.Marshal(map[string]any{"code": code, "line": line, "col": col})
	payload := fmt.Sprintf("%s\n%s\n%s\n", kernelCompleteMarker, string(req), kernelRunMarker)
	resultJSON, err := kernelRPC(sess, payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Completions []NotebookCompletion `json:"completions"`
	}
	if resultJSON != "" {
		_ = json.Unmarshal([]byte(resultJSON), &resp)
	}
	return resp.Completions, nil
}

// GetNotebookHover queries jedi hover documentation from the running kernel for
// the given Python source at cursor position (line, col).  Returns an empty
// string when no kernel is running or no documentation is available.
func (s *Service) GetNotebookHover(tabId, code string, line, col int) (string, error) {
	val, ok := notebookSessions.Load(tabId)
	if !ok {
		return "", nil
	}
	sess := val.(*notebookSession)
	sess.mu.Lock()
	defer sess.mu.Unlock()

	req, _ := json.Marshal(map[string]any{"code": code, "line": line, "col": col})
	payload := fmt.Sprintf("%s\n%s\n%s\n", kernelHoverMarker, string(req), kernelRunMarker)
	resultJSON, err := kernelRPC(sess, payload)
	if err != nil {
		return "", err
	}
	var resp struct {
		Hover string `json:"hover"`
	}
	if resultJSON != "" {
		_ = json.Unmarshal([]byte(resultJSON), &resp)
	}
	return resp.Hover, nil
}

// CheckPythonSyntax asks the running Python kernel to analyze code and return
// diagnostics.  mode controls the analysis depth:
//   - "off"    — always returns nil (caller should skip, but Go side handles it too)
//   - "static" — ast.parse + pyflakes without kernel namespace stubs
//   - "kernel" — ast.parse + pyflakes with live namespace stubs (default)
//
// Returns nil (no error) when no kernel is running for the tab.
func (s *Service) CheckPythonSyntax(tabId, code, mode string) ([]NotebookSyntaxError, error) {
	if mode == "off" {
		return nil, nil
	}
	val, ok := notebookSessions.Load(tabId)
	if !ok {
		return nil, nil
	}
	sess := val.(*notebookSession)
	sess.mu.Lock()
	defer sess.mu.Unlock()

	req, _ := json.Marshal(map[string]any{"code": code, "mode": mode})
	payload := fmt.Sprintf("%s\n%s\n%s\n", kernelSyntaxMarker, string(req), kernelRunMarker)
	resultJSON, err := kernelRPC(sess, payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Errors []NotebookSyntaxError `json:"errors"`
	}
	if resultJSON != "" {
		_ = json.Unmarshal([]byte(resultJSON), &resp)
	}
	return resp.Errors, nil
}

// StopNotebookSession kills the Python kernel for a notebook tab.
func (s *Service) StopNotebookSession(tabId string) error {
	val, ok := notebookSessions.LoadAndDelete(tabId)
	if !ok {
		return nil
	}
	sess := val.(*notebookSession)
	sess.mu.Lock()
	defer sess.mu.Unlock()
	_ = sess.stdin.Close()
	if sess.cmd.Process != nil {
		return sess.cmd.Process.Kill()
	}
	return nil
}
