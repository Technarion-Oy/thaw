// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Drag-to-move for every Ant Design modal (issue #572).
//
// antd's <Modal> has no `draggable` prop, but every instance renders the same
// stable DOM (`.ant-modal` box, `.ant-modal-header` title bar). A single
// document-level mousedown delegate grabs the header and translates the box, so
// all ~160 modals — present and future — become draggable without touching a
// single Modal call site. Resize is pure CSS (`resize: both` on
// `.ant-modal-content`, see global.css); this file only owns the drag.
//
// Position lives as an inline transform on the `.ant-modal` node. Thaw's modals
// unmount on close (conditional render + `<Modal open>`), so the node — and its
// transform — is discarded and rebuilt on reopen, giving the "reset to default
// on reopen" behaviour for free.
//
// ponytail: modals that toggle the `open` prop without unmounting keep their
// dragged position across reopen; add an on-hide transform reset if that ever
// matters.

// px of the modal kept on-screen horizontally, and the header band kept within
// the viewport vertically, so a drag can never push the handle out of reach.
const KEEP_X = 80;
const KEEP_Y = 40;
// Bottom-right square reserved for the native CSS resize grip; a mousedown here
// must start a resize, not a drag (both would otherwise read the same
// mousemove stream and fight — the grip has no DOM node of its own).
const GRIP = 20;

// Controls whose own click/drag must win over the modal drag. A denylist rather
// than an allowlist because header content varies per modal; err toward "this
// looks interactive, don't hijack it".
const INTERACTIVE =
  "button, a, input, select, textarea, label, [role=button], [tabindex]," +
  " .ant-select, .ant-switch, .ant-checkbox, .ant-radio, .ant-modal-close";

let drag:
  | { el: HTMLElement; startX: number; startY: number; baseX: number; baseY: number;
      startLeft: number; startTop: number; width: number; height: number }
  | null = null;

export function parseTranslate(transform: string): [number, number] {
  const m = /translate\(\s*([-\d.]+)px\s*,\s*([-\d.]+)px\s*\)/.exec(transform);
  return m ? [parseFloat(m[1]), parseFloat(m[2])] : [0, 0];
}

function clamp(v: number, lo: number, hi: number) {
  return Math.min(Math.max(v, lo), hi);
}

function endDrag() {
  if (!drag) return;
  drag = null;
  document.body.style.userSelect = "";
}

function onMouseDown(e: MouseEvent) {
  if (e.button !== 0) return;
  const target = e.target as HTMLElement;
  // Drag handle = the title bar, plus the modal's own padding box. antd puts a
  // ~20px top padding on `.ant-modal-content` and none on the header, so the
  // topmost strip of the dialog lands on the content element, not the header —
  // grabbing there must still start a drag, or the very top edge feels dead.
  const onContent = target.classList.contains("ant-modal-content");
  if (!onContent && target.closest(".ant-modal-header") === null) return;
  // Leave the bottom-right corner to the browser's native resize grip.
  if (onContent) {
    const r = target.getBoundingClientRect();
    if (r.right - e.clientX < GRIP && r.bottom - e.clientY < GRIP) return;
  }
  // Don't hijack drags that start on interactive controls placed in the title.
  if (target.closest(INTERACTIVE)) return;
  const modal = target.closest(".ant-modal") as HTMLElement | null;
  if (!modal) return;

  const [baseX, baseY] = parseTranslate(modal.style.transform);
  const rect = modal.getBoundingClientRect();
  drag = {
    el: modal, startX: e.clientX, startY: e.clientY, baseX, baseY,
    startLeft: rect.left, startTop: rect.top, width: rect.width, height: rect.height,
  };
  document.body.style.userSelect = "none";
  e.preventDefault();
}

function onMouseMove(e: MouseEvent) {
  if (!drag) return;
  // The button was released outside our event stream (alt-tab, a native dialog
  // stealing focus, …) — recover instead of sticking to the cursor forever.
  if (e.buttons === 0) { endDrag(); return; }

  const vw = window.innerWidth;
  const vh = window.innerHeight;
  // Clamp against the viewport using the box position captured at drag start;
  // translate offsets the layout position linearly, so left/top move 1:1 with
  // the cursor delta.
  const newLeft = clamp(drag.startLeft + (e.clientX - drag.startX), KEEP_X - drag.width, vw - KEEP_X);
  const newTop  = clamp(drag.startTop  + (e.clientY - drag.startY), 0, vh - KEEP_Y);
  const x = drag.baseX + (newLeft - drag.startLeft);
  const y = drag.baseY + (newTop - drag.startTop);
  drag.el.style.transform = `translate(${x}px, ${y}px)`;
}

if (typeof document !== "undefined") {
  document.addEventListener("mousedown", onMouseDown);
  document.addEventListener("mousemove", onMouseMove);
  document.addEventListener("mouseup", endDrag);
  // A focus loss (native dialog, OS drag interruption) can swallow the mouseup.
  window.addEventListener("blur", endDrag);
}
