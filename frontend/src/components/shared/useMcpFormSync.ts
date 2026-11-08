// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: MCP Server

import { useEffect } from "react";
import { Form } from "antd";
import type { FormInstance } from "antd";
import { UpdateOpenFormState, ClearOpenFormState } from "../../../wailsjs/go/app/App";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { mcp } from "../../../wailsjs/go/models";

/**
 * Exposes an Ant Design form to the MCP `get_open_form` / `fill_open_form`
 * tools: pushes the form's values to the backend on mount and on change
 * (debounced 300 ms), clears them on unmount, and applies `mcp:fill-form`
 * events via `form.setFieldsValue`, briefly highlighting the filled fields.
 * The user still reviews and submits — nothing is executed.
 */
export function useMcpFormSync(
  formId: string,
  form: FormInstance,
  meta?: { kind?: string; provider?: string },
): void {
  const values = Form.useWatch((v) => v, { form, preserve: true });
  const kind = meta?.kind;
  const provider = meta?.provider;

  useEffect(() => {
    const timer = setTimeout(() => {
      UpdateOpenFormState(new mcp.OpenFormState({
        form: formId, kind, provider, fields: form.getFieldsValue(true),
      })).catch(() => {});
    }, 300);
    return () => clearTimeout(timer);
  }, [formId, form, kind, provider, values]);

  useEffect(() => () => { ClearOpenFormState().catch(() => {}); }, []);

  useEffect(() => EventsOn("mcp:fill-form", (p: { form: string; fields: Record<string, unknown> }) => {
    if (p.form !== formId) return;
    form.setFieldsValue(p.fields);
    // Fields behind a provider switch mount on the next render — highlight after it.
    setTimeout(() => {
      for (const name of Object.keys(p.fields)) {
        const item = document.getElementById(name)?.closest(".ant-form-item");
        if (!item) continue;
        item.classList.add("mcp-ai-filled");
        setTimeout(() => item.classList.remove("mcp-ai-filled"), 2000);
      }
    }, 50);
  }), [formId, form]);
}
