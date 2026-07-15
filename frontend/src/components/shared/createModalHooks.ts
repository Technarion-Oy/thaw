// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import type { DependencyList } from "react";
import { GetQuotedIdentifiersIgnoreCase } from "../../../wailsjs/go/app/App";

/**
 * Reads the session's QUOTED_IDENTIFIERS_IGNORE_CASE flag once on mount. Fed to
 * `ObjectNameCaseControl` so it can warn when case-insensitive quoting applies.
 */
export function useQuotedIdentifiers(): boolean {
  const [v, setV] = useState(false);
  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase()
      .then((x) => setV(x ?? false))
      .catch(() => {});
  }, []);
  return v;
}

interface SqlPreviewOptions {
  /**
   * When true, a failed build blanks the preview instead of keeping the last
   * good value. Use for modals that gate submit on a non-empty preview and would
   * otherwise risk executing stale SQL after a build error (e.g. git repository).
   */
  blankOnError?: boolean;
}

/**
 * Keeps a live SQL preview string in sync with form state. `build` produces the
 * preview (sync or async — backend builders return a Promise); it re-runs
 * whenever `deps` change.
 *
 * An out-of-order async build can't clobber a newer one: each run is invalidated
 * on cleanup, so only the latest in-flight build updates the preview. By default
 * a build error is swallowed and the *last good* preview is retained so a
 * transient failure never crashes the modal — callers that gate submit on the
 * preview should also guard on `canSubmit`. Pass `{ blankOnError: true }` to
 * blank the preview on error instead.
 */
export function useSqlPreview(
  build: () => string | Promise<string>,
  deps: DependencyList,
  options?: SqlPreviewOptions,
): string {
  const [preview, setPreview] = useState("");
  const blankOnError = options?.blankOnError ?? false;
  useEffect(() => {
    let cancelled = false;
    Promise.resolve(build())
      .then((sql) => { if (!cancelled) setPreview(sql); })
      .catch(() => { if (!cancelled && blankOnError) setPreview(""); });
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
  return preview;
}

/**
 * Standard submit plumbing for create modals: a `creating` flag, an `error`
 * string, and a `submit` runner that toggles the flag, clears prior errors,
 * runs the supplied async action, and captures any thrown error. The action is
 * responsible for its own success side effects (e.g. `onSuccess()` + `onClose()`),
 * which only run when it resolves without throwing.
 */
export function useCreateSubmit() {
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = useCallback(async (run: () => Promise<void>) => {
    setCreating(true);
    setError(null);
    try {
      await run();
    } catch (err) {
      setError(String(err));
    } finally {
      setCreating(false);
    }
  }, []);

  return { creating, error, setError, submit };
}
