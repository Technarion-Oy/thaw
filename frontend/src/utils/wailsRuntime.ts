// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Core IPC & App Lifecycle

/**
 * Single frontend boundary for Wails runtime calls (events, native clipboard,
 * system browser). Keeping every component and utility pointed here instead of
 * `wailsjs/runtime/runtime` isolates the v2 → v3 cutover (#23) — including v3's
 * `event.data` payload wrapper in `EventsOn` — to this one file.
 */

import {
  BrowserOpenURL as wailsBrowserOpenURL,
  ClipboardGetText as wailsClipboardGetText,
  ClipboardSetText as wailsClipboardSetText,
  EventsEmit as wailsEventsEmit,
  EventsOn as wailsEventsOn,
} from "../../wailsjs/runtime/runtime";

/** Subscribe to a backend Wails event. Returns an unsubscribe callback. */
export function EventsOn(
  eventName: string,
  callback: (...data: any[]) => void,
): () => void {
  return wailsEventsOn(eventName, callback);
}

/** Emit an event to backend Wails listeners. */
export function EventsEmit(eventName: string, ...data: any[]): void {
  wailsEventsEmit(eventName, ...data);
}

/** Read plain text from the OS clipboard via the native Wails runtime. */
export function ClipboardGetText(): Promise<string> {
  return wailsClipboardGetText();
}

/** Write plain text to the OS clipboard via the native Wails runtime. */
export function ClipboardSetText(text: string): Promise<boolean> {
  return wailsClipboardSetText(text);
}

/** Open a URL in the user's default system browser. */
export function BrowserOpenURL(url: string): void {
  wailsBrowserOpenURL(url);
}
