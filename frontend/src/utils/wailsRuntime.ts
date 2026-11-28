// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Core IPC & App Lifecycle

/**
 * Single frontend boundary for Wails v3 runtime calls (events, native
 * clipboard, system browser). Every component and utility imports runtime
 * helpers from here rather than `@wailsio/runtime` directly — isolating v3's
 * `WailsEvent.data` payload wrapper and `Promise<void>` clipboard API to this
 * one module.
 */

import {
  Browser,
  Call,
  Clipboard,
  Events,
} from "@wailsio/runtime";

// Wails v2 rejected failed IPC calls with the plain Go error string; v3 rejects
// with a RuntimeError (extends Error, name = "RuntimeError"), so `String(e)` /
// `${e}` across ~500 UI call sites would otherwise show a "RuntimeError: "
// prefix. Override `toString` once here to return the clean Go error message.
if (Call?.RuntimeError?.prototype) {
  Call.RuntimeError.prototype.toString = function toString(): string {
    return this.message;
  };
}

/** Subscribe to a backend Wails event. Returns an unsubscribe callback. */
export function EventsOn(
  eventName: string,
  callback: (...data: any[]) => void,
): () => void {
  return Events.On(eventName, (event) => {
    if (event.data === null || event.data === undefined) {
      callback();
    } else {
      callback(event.data);
    }
  });
}

/** Emit an event to backend Wails listeners. */
export function EventsEmit(eventName: string, ...data: any[]): void {
  const promise =
    data.length === 0
      ? Events.Emit(eventName)
      : data.length === 1
        ? Events.Emit(eventName, data[0])
        : Events.Emit(eventName, data);
  void Promise.resolve(promise).catch(() => {});
}

/** Read plain text from the OS clipboard via the native Wails runtime. */
export function ClipboardGetText(): Promise<string> {
  return Clipboard.Text();
}

/** Write plain text to the OS clipboard via the native Wails runtime. */
export async function ClipboardSetText(text: string): Promise<boolean> {
  try {
    await Clipboard.SetText(text);
    return true;
  } catch {
    return false;
  }
}

/** Open a URL in the user's default system browser. */
export function BrowserOpenURL(url: string): void {
  void Promise.resolve(Browser.OpenURL(url)).catch(() => {});
}
