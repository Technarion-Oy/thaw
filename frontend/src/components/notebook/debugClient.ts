// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

/**
 * Minimal Debug Adapter Protocol (DAP) client for notebook cell debugging.
 *
 * The client communicates via Wails events:
 * dap:client-to-backend — raw DAP frames sent from React → Go proxy → debugpy
 * dap:backend-to-client — raw DAP frames received from debugpy → Go proxy → React
 *
 * The DAP framing is: "Content-Length: N\r\n\r\n{json}"
 */

import { EventsEmit, EventsOn } from "../../../wailsjs/runtime/runtime";

export interface DebugVariable {
  name: string;
  value: string;
  type: string;
}

// ─── DAP Client ───────────────────────────────────────────────────────────────

export class DapClient {
  private seq = 1;
  // CRITICAL: Use a true binary buffer to prevent multi-byte character corruption
  private byteBuffer = new Uint8Array(0); 
  /** Map from request seq → resolve/reject pair */
  private pending = new Map<number, { resolve: (body: any) => void; reject: (err: Error) => void }>();
  private offListener: (() => void) | null = null;

  /** Called when execution pauses at a breakpoint. */
  onStopped?: (variables: DebugVariable[]) => void;
  /** Called when execution resumes after Continue. */
  onContinued?: () => void;

  private breakpoints: Set<number>;
  private filepath: string;

  constructor(breakpoints: Set<number>, filepath: string) {
    this.breakpoints = breakpoints;
    this.filepath = filepath;
  }

  private offDisconnected: (() => void) | null = null;
  /** Resolved when the server fires the "initialized" event. */
  private _onInitialized: (() => void) | null = null;

  /** Register the backend event listener. Call before initialize(). */
  start() {
    // We now receive Base64 strings from Go to ensure raw bytes survive the IPC bridge
    this.offListener = EventsOn("dap:backend-to-client", (b64Chunk: string) => {
      this.receive(b64Chunk);
    }) as () => void;

    // If the TCP proxy drops, reject all pending DAP requests immediately.
    this.offDisconnected = EventsOn("dap:disconnected", () => {
      const err = new Error("DAP connection lost");
      for (const handler of this.pending.values()) handler.reject(err);
      this.pending.clear();
    }) as () => void;
  }

  /** Unregister the backend event listener. */
  stop() {
    if (this.offListener) {
      this.offListener();
      this.offListener = null;
    }
    if (this.offDisconnected) {
      this.offDisconnected();
      this.offDisconnected = null;
    }
  }

  // ── DAP framing ─────────────────────────────────────────────────────────────

  private sendRaw(msg: Record<string, unknown>) {
    const json = JSON.stringify(msg);
    // Calculate precise UTF-8 byte length, not JS string length!
    const byteLength = new TextEncoder().encode(json).length; 
    const frame = `Content-Length: ${byteLength}\r\n\r\n${json}`;
    EventsEmit("dap:client-to-backend", frame);
  }

  private request<T = unknown>(command: string, args?: Record<string, unknown>): Promise<T> {
    return new Promise((resolve, reject) => {
      const seq = this.seq++;
      const timer = setTimeout(() => {
        if (this.pending.has(seq)) {
          this.pending.delete(seq);
          reject(new Error(`DAP ${command} timed out after 15s`));
        }
      }, 15000);
      this.pending.set(seq, {
        resolve: (body: unknown) => { clearTimeout(timer); resolve(body as T); },
        reject: (err: Error) => { clearTimeout(timer); reject(err); },
      });
      // Explicitly pass the arguments object
      this.sendRaw({ seq, type: "request", command, arguments: args || {} });
    });
  }

  /** Feed a raw Base64 chunk received from the proxy into the binary buffer. */
  receive(b64Chunk: string) {
    // 1. Decode Base64 back into raw bytes
    const binaryStr = atob(b64Chunk);
    const chunkBytes = new Uint8Array(binaryStr.length);
    for (let i = 0; i < binaryStr.length; i++) {
      chunkBytes[i] = binaryStr.charCodeAt(i);
    }
    
    // 2. Append to master buffer
    const newBuf = new Uint8Array(this.byteBuffer.length + chunkBytes.length);
    newBuf.set(this.byteBuffer);
    newBuf.set(chunkBytes, this.byteBuffer.length);
    this.byteBuffer = newBuf;

    // 3. Parse headers and body purely by counting bytes
    while (true) {
      let headerEnd = -1;
      // Search for \r\n\r\n (13, 10, 13, 10 in ASCII)
      for (let i = 0; i < this.byteBuffer.length - 3; i++) {
        if (this.byteBuffer[i] === 13 && this.byteBuffer[i+1] === 10 && 
            this.byteBuffer[i+2] === 13 && this.byteBuffer[i+3] === 10) {
          headerEnd = i;
          break;
        }
      }
      
      if (headerEnd === -1) break; // Incomplete header, wait for more data

      const headerStr = new TextDecoder().decode(this.byteBuffer.slice(0, headerEnd));
      const lengthMatch = headerStr.match(/Content-Length: (\d+)/i);
      
      if (!lengthMatch) {
        this.byteBuffer = new Uint8Array(0); // Corrupt? Clear buffer.
        break;
      }
      
      const bodyLen = parseInt(lengthMatch[1], 10);
      const fullMessageLen = headerEnd + 4 + bodyLen;
      
      if (this.byteBuffer.length < fullMessageLen) {
        break; // Incomplete body, wait for more chunks
      }

      // Slice out the exact bytes of the JSON body
      const bodyBytes = this.byteBuffer.slice(headerEnd + 4, fullMessageLen);
      this.byteBuffer = this.byteBuffer.slice(fullMessageLen);

      // Decode the precise JSON bytes into a string and dispatch
      const bodyStr = new TextDecoder().decode(bodyBytes);
      try {
        this.dispatch(JSON.parse(bodyStr));
      } catch (e) {
        console.error("Failed to parse DAP body", e);
      }
    }
  }

  private dispatch(msg: Record<string, unknown>) {
    if (msg.type === "response") {
      const handler = this.pending.get(msg.request_seq as number);
      if (handler) {
        this.pending.delete(msg.request_seq as number);
        if (msg.success) {
          handler.resolve(msg.body);
        } else {
          handler.reject(new Error((msg.message as string) || `DAP ${msg.command} failed`));
        }
      }
    } else if (msg.type === "event") {
      this.handleEvent(msg);
    }
  }

  private handleEvent(event: Record<string, unknown>) {
    switch (event.event) {
      case "initialized":
        // Server is ready to receive attach/launch and configuration requests.
        this._onInitialized?.();
        this._onInitialized = null;
        break;
      case "stopped":
        void this.onBreakpointStop((event.body as any)?.threadId ?? 1);
        break;
      case "continued":
        this.onContinued?.();
        break;
    }
  }

  // ── Breakpoint stop handling ─────────────────────────────────────────────────

  private async onBreakpointStop(threadId: number) {
    try {
      // 1. Get the top stack frame to obtain a frameId
      const stackTrace = await this.request<any>("stackTrace", {
        threadId,
        startFrame: 0,
        levels: 1,
      });
      const frameId: number | undefined = stackTrace?.stackFrames?.[0]?.id;
      if (frameId === undefined) {
        this.onStopped?.([]);
        return;
      }

      // 2. Get scopes for the frame
      const scopesResp = await this.request<any>("scopes", { frameId });
      // Prefer "Locals"; fall back to the first scope
      const scope = (scopesResp?.scopes ?? []).find((s: any) => s.name === "Locals")
        ?? scopesResp?.scopes?.[0];
      if (!scope) {
        this.onStopped?.([]);
        return;
      }

      // 3. Get variables for the locals scope
      const varsResp = await this.request<any>("variables", {
        variablesReference: scope.variablesReference,
        count: 100,
      });
      const variables: DebugVariable[] = (varsResp?.variables ?? [])
        // Skip dunder/internal names and special variables
        .filter((v: any) => !v.name.startsWith("__") && !v.name.startsWith("_thaw"))
        .slice(0, 60)
        .map((v: any) => ({
          name: String(v.name),
          value: String(v.value),
          type: String(v.type || ""),
        }));

      this.onStopped?.(variables);
    } catch {
      this.onStopped?.([]);
    }
  }

  // ── Session lifecycle ────────────────────────────────────────────────────────

  /** Run the full DAP handshake: initialize → attach (fire) → setBreakpoints → configurationDone. */
  async initialize(): Promise<void> {
    // 1. Initialize — wait for the response (contains server capabilities).
    await this.request("initialize", {
      clientID: "thaw",
      clientName: "Thaw",
      adapterID: "python",
      pathFormat: "path",
      linesStartAt1: true,
      columnsStartAt1: true,
      supportsVariableType: true,
    });

    // 2. Fire "attach" WITHOUT awaiting the response.
    //
    //    Debugpy in listen() mode requires "configurationDone" to arrive WHILE it is
    //    still processing "attach" — only then does it send the "attach" response and
    //    unblock wait_for_client(). Awaiting the "attach" response first creates a
    //    deadlock: we wait for the response, debugpy waits for configurationDone.
    //    The VS Code Python extension uses the same fire-and-forget pattern.
    this.sendRaw({
      seq: this.seq++,
      type: "request",
      command: "attach",
      arguments: { justMyCode: false },
    });

    // 3. Set breakpoints (if any) — sent while attach is in-flight.
    if (this.filepath && this.breakpoints.size > 0) {
      await this.request("setBreakpoints", {
        source: { path: this.filepath },
        breakpoints: Array.from(this.breakpoints).sort((a, b) => a - b).map((line) => ({ line })),
        sourceModified: false,
      });
    }

    // 4. configurationDone arrives at debugpy while attach is still being handled
    //    → debugpy completes init, sends configurationDone response + attach response,
    //    and wait_for_client() returns, letting Python execute the cell.
    await this.request("configurationDone", {});
  }

  /** Resume execution after pausing at a breakpoint. */
  continue(threadId = 1) {
    this.sendRaw({ seq: this.seq++, type: "request", command: "continue", arguments: { threadId } });
    this.onContinued?.();
  }

  /** Step over the current line (next). */
  stepOver(threadId = 1) {
    this.sendRaw({ seq: this.seq++, type: "request", command: "next", arguments: { threadId } });
    this.onContinued?.();
  }

  /** Step into the function call on the current line. */
  stepInto(threadId = 1) {
    this.sendRaw({ seq: this.seq++, type: "request", command: "stepIn", arguments: { threadId } });
    this.onContinued?.();
  }

  /** Step out of the current function. */
  stepOut(threadId = 1) {
    this.sendRaw({ seq: this.seq++, type: "request", command: "stepOut", arguments: { threadId } });
    this.onContinued?.();
  }

  /** Disconnect the debugger (letting the code run to completion without pausing). */
  disconnect() {
    this.sendRaw({ seq: this.seq++, type: "request", command: "disconnect", arguments: { terminateDebuggee: false } });
    this.onContinued?.();
  }
}