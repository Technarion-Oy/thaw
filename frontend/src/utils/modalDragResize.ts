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
// @thaw-domain: Core IPC & App Lifecycle

// Drag-to-move for every Ant Design modal (issue #572).
//
// Ant Design's <Modal> has no `draggable` prop, but every instance renders the
// same stable DOM (`.ant-modal` box, `.ant-modal-header` title bar). A single
// document-level mousedown delegate grabs the header and translates the box —
// so all ~160 modals, present and future, become draggable without touching a
// single Modal call site. Resize is pure CSS (`resize: both` on
// `.ant-modal-content`, see global.css); no coordinate JS needed for it.
//
// Position lives as an inline transform on the `.ant-modal` node. Thaw's modals
// unmount on close (conditional render + `<Modal open>`), so the node — and its
// transform — is discarded and rebuilt on reopen, giving the "reset to default
// on reopen" behaviour for free.
//
// ponytail: modals that toggle the `open` prop without unmounting keep their
// dragged position across reopen; add an on-hide transform reset if that ever
// matters.

let drag: { el: HTMLElement; startX: number; startY: number; baseX: number; baseY: number } | null = null;

export function parseTranslate(transform: string): [number, number] {
  const m = /translate\(\s*([-\d.]+)px\s*,\s*([-\d.]+)px\s*\)/.exec(transform);
  return m ? [parseFloat(m[1]), parseFloat(m[2])] : [0, 0];
}

function onMouseDown(e: MouseEvent) {
  if (e.button !== 0) return;
  const target = e.target as HTMLElement;
  // Drag handle = the title bar, plus the modal's own padding box. antd puts a
  // ~20px top padding on `.ant-modal-content` and none on the header, so the
  // topmost strip of the dialog lands on the content element, not the header —
  // grabbing there must still start a drag, or the very top edge feels dead.
  const onHandle =
    target.closest(".ant-modal-header") !== null ||
    target.classList.contains("ant-modal-content");
  if (!onHandle) return;
  // Don't hijack drags that start on interactive controls placed in the title.
  if (target.closest("button, a, input, .ant-select, .ant-modal-close")) return;
  const modal = target.closest(".ant-modal") as HTMLElement | null;
  if (!modal) return;

  const [baseX, baseY] = parseTranslate(modal.style.transform);
  drag = { el: modal, startX: e.clientX, startY: e.clientY, baseX, baseY };
  document.body.style.userSelect = "none";
  e.preventDefault();
}

function onMouseMove(e: MouseEvent) {
  if (!drag) return;
  const x = drag.baseX + (e.clientX - drag.startX);
  const y = drag.baseY + (e.clientY - drag.startY);
  drag.el.style.transform = `translate(${x}px, ${y}px)`;
}

function onMouseUp() {
  if (!drag) return;
  drag = null;
  document.body.style.userSelect = "";
}

if (typeof document !== "undefined") {
  document.addEventListener("mousedown", onMouseDown);
  document.addEventListener("mousemove", onMouseMove);
  document.addEventListener("mouseup", onMouseUp);
}
