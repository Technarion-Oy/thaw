// SPDX-License-Identifier: GPL-3.0-or-later

// Drag-to-move for every Ant Design modal (issue #572).
//
// antd's <Modal> has no `draggable` prop, but every instance renders the same
// stable DOM (`.ant-modal` box, `.ant-modal-header` title bar). A single
// document-level mousedown delegate grabs the header and moves the box, so all
// ~160 modals — present and future — become draggable without touching a single
// Modal call site. Resize is native CSS (`resize` on `.ant-modal`, see
// global.css); this file owns only the drag.
//
// Drag moves the box with inline `left`/`top`, NOT `transform`. A CSS transform
// on `.ant-modal` would establish a containing block for `position: fixed`
// descendants (e.g. the custom context menus in TaskGraphModal / ERCanvas /
// StageBrowserModal), pinning them to the modal instead of the viewport.
// `.ant-modal` is already `position: relative`, so left/top offsets move it
// without that side effect (offsets are relative to the normal-flow position,
// so they compose cleanly with antd's centering / `top: 100px`).
//
// Offset lives as inline left/top and, for resize, inline width on `.ant-modal`.
// Thaw's modals unmount on close (conditional render + `<Modal open>` — including
// GitOperationsDialog, gated in AppLayout), so both are discarded and rebuilt on
// reopen: the "reset on reopen" behaviour for free.

// Keep this much of the modal on-screen horizontally, and this much of the
// header band vertically, so neither a drag nor a later window shrink can push
// the handle out of reach.
const KEEP_X = 80;
const KEEP_Y = 40;

// Controls whose own click/drag must win over the modal drag. A denylist rather
// than an allowlist because header content varies per modal; err toward "this
// looks interactive, don't hijack it".
const INTERACTIVE =
  "button, a, input, select, textarea, label, [role=button], [tabindex]," +
  " .ant-select, .ant-switch, .ant-checkbox, .ant-radio, .ant-modal-close";

let drag:
  | { el: HTMLElement; startX: number; startY: number;
      baseLeft: number; baseTop: number; startLeft: number; startTop: number; width: number }
  | null = null;

export function clamp(v: number, lo: number, hi: number) {
  return Math.min(Math.max(v, lo), hi);
}

// Current left/top offset of a relatively-positioned modal: the inline value if
// we've already moved it, else antd's computed base (top: 100px; left → 0).
function readOffset(el: HTMLElement): [number, number] {
  const cs = getComputedStyle(el);
  const left = el.style.left ? parseFloat(el.style.left) : parseFloat(cs.left) || 0;
  const top = el.style.top ? parseFloat(el.style.top) : parseFloat(cs.top) || 0;
  return [left, top];
}

// Nudge a modal's inline left/top so its box keeps KEEP_X/KEEP_Y on-screen.
// Relative offsets move the box 1:1 on screen, so a screen-space correction maps
// straight onto the inline offset.
function clampIntoView(el: HTMLElement) {
  const r = el.getBoundingClientRect();
  const wantLeft = clamp(r.left, KEEP_X - r.width, window.innerWidth - KEEP_X);
  const wantTop = clamp(r.top, 0, window.innerHeight - KEEP_Y);
  const [left, top] = readOffset(el);
  if (wantLeft !== r.left) el.style.left = `${left + (wantLeft - r.left)}px`;
  if (wantTop !== r.top) el.style.top = `${top + (wantTop - r.top)}px`;
}

function endDrag() {
  if (!drag) return;
  drag = null;
  document.body.style.userSelect = "";
}

function onMouseDown(e: MouseEvent) {
  if (e.button !== 0) return;
  const target = e.target as HTMLElement;
  const modal = target.closest(".ant-modal") as HTMLElement | null;
  if (!modal) return;
  // Drag handle = the title bar, plus the content's top padding band. antd puts
  // ~20px top padding on `.ant-modal-content` with none on the header, so the
  // very top edge lands on the content element, not the header. Restrict the
  // content-as-handle to that top band (above the body) so nothing lower — the
  // body, footer, or the resize grip in the corner — is mistaken for a drag.
  const headerEl = target.closest(".ant-modal-header") as HTMLElement | null;
  if (!headerEl) {
    if (!target.classList.contains("ant-modal-content")) return;
    const body = target.querySelector(".ant-modal-body");
    if (body && e.clientY >= body.getBoundingClientRect().top) return;
  }
  // Don't hijack drags that start on an interactive control placed *in* the
  // handle. Bound the match by the handle element: antd wraps the dialog in
  // focus-trap/`[tabindex]` divs that are ANCESTORS of the header, so an
  // unbounded `closest(INTERACTIVE)` matches those and blocks every drag — only
  // a control the handle actually contains should count.
  const handle = headerEl ?? (modal.querySelector(".ant-modal-content") as HTMLElement | null);
  const hit = target.closest(INTERACTIVE);
  if (hit && handle && hit !== handle && handle.contains(hit)) return;

  const [baseLeft, baseTop] = readOffset(modal);
  const rect = modal.getBoundingClientRect();
  drag = {
    el: modal, startX: e.clientX, startY: e.clientY,
    baseLeft, baseTop, startLeft: rect.left, startTop: rect.top, width: rect.width,
  };
  document.body.style.userSelect = "none";
  e.preventDefault();
}

function onMouseMove(e: MouseEvent) {
  if (!drag) return;
  // NB: do not gate on `e.buttons` here — WKWebView (this app's engine) reports
  // buttons === 0 during mousemove even while the button is held, which would
  // kill the drag on the first move. Recovery from a lost mouseup is handled by
  // the mouseup + window `blur` listeners instead.
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  // Clamp the target screen position, then express it as the inline offset delta
  // from the drag-start position (relative left/top move the box 1:1 on screen).
  const screenLeft = clamp(drag.startLeft + (e.clientX - drag.startX), KEEP_X - drag.width, vw - KEEP_X);
  const screenTop = clamp(drag.startTop + (e.clientY - drag.startY), 0, vh - KEEP_Y);
  drag.el.style.left = `${drag.baseLeft + (screenLeft - drag.startLeft)}px`;
  drag.el.style.top = `${drag.baseTop + (screenTop - drag.startTop)}px`;
}

function onWindowResize() {
  // A native window shrink can strand a moved/resized modal off-screen; pull any
  // that carry an inline offset back into reach. (Untouched, centered modals are
  // already within bounds, so the clamp is a no-op for them.)
  document.querySelectorAll<HTMLElement>(".ant-modal").forEach((el) => {
    if (el.style.left || el.style.top) clampIntoView(el);
  });
}

if (typeof document !== "undefined") {
  document.addEventListener("mousedown", onMouseDown);
  document.addEventListener("mousemove", onMouseMove);
  document.addEventListener("mouseup", endDrag);
  // A focus loss (native dialog, OS drag interruption) can swallow the mouseup.
  window.addEventListener("blur", endDrag);
  window.addEventListener("resize", onWindowResize);
}
