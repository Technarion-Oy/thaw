// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
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

/**
 * Keeps a live SQL preview string in sync with form state. `build` produces the
 * preview (sync or async — backend builders return a Promise); it re-runs
 * whenever `deps` change. Errors are swallowed so a transient build failure
 * never crashes the modal.
 */
export function useSqlPreview(build: () => string | Promise<string>, deps: DependencyList): string {
  const [preview, setPreview] = useState("");
  useEffect(() => {
    Promise.resolve(build())
      .then(setPreview)
      .catch(() => {});
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
