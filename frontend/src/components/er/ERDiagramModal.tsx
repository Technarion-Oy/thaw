// Copyright (c) 2026 Technarion Oy. All rights reserved.

import { useState, useId, useEffect, useRef, useMemo } from "react";
import { Modal, Button, Checkbox, Spin } from "antd";
import { ZoomInOutlined, ZoomOutOutlined, CopyOutlined } from "@ant-design/icons";
import mermaid from "mermaid";
import type { snowflake } from "../../../wailsjs/go/models";
import { buildMermaid } from "./buildMermaid";

mermaid.initialize({
  startOnLoad: false,
  securityLevel: "loose",
  theme: "dark",
});

/**
 * Scale an SVG string to the requested zoom level by rewriting its
 * width / max-width attributes.  Mermaid outputs SVGs with
 * `width="100%"` and `style="max-width: Npx;"`.  We replace those
 * with explicit pixel values so the browser lays the element out at
 * the correct size and the scroll container shows scrollbars when
 * the diagram is larger than the viewport.
 */
function applyZoom(svg: string, zoom: number): string {
  if (!svg) return svg;
  // Extract the natural pixel width from the max-width style
  const maxWidthMatch = svg.match(/max-width:\s*([\d.]+)px/);
  if (maxWidthMatch) {
    const naturalPx = parseFloat(maxWidthMatch[1]);
    const zoomedPx = Math.round(naturalPx * zoom);
    return svg
      .replace(/max-width:\s*[\d.]+px;?\s*/, `max-width: ${zoomedPx}px; `)
      .replace(/\bwidth="100%"/, `width="${zoomedPx}"`);
  }
  // Fallback: SVG has explicit width/height attributes
  const wMatch = svg.match(/\bwidth="([\d.]+)"/);
  const hMatch = svg.match(/\bheight="([\d.]+)"/);
  if (wMatch && hMatch) {
    const w = Math.round(parseFloat(wMatch[1]) * zoom);
    const h = Math.round(parseFloat(hMatch[1]) * zoom);
    return svg
      .replace(/\bwidth="[\d.]+"/, `width="${w}"`)
      .replace(/\bheight="[\d.]+"/, `height="${h}"`);
  }
  return svg;
}

interface Props {
  database: string;
  data: snowflake.ERDiagramData;
  onClose: () => void;
}

export default function ERDiagramModal({ database, data, onClose }: Props) {
  const baseId = useId().replace(/:/g, "_");
  const renderCount = useRef(0);

  const allSchemas = [...new Set(data.tables.map((t) => t.schema))].sort();

  const [visibleSchemas, setVisibleSchemas] = useState<Set<string>>(new Set(allSchemas));
  const [zoom, setZoom] = useState(1);
  const [rawSvg, setRawSvg] = useState<string>("");
  const [rendering, setRendering] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Panning state
  const containerRef = useRef<HTMLDivElement>(null);
  const [panning, setPanning] = useState(false);
  const panningRef = useRef(false); // ref avoids stale closure in mousemove
  const panOrigin = useRef({ x: 0, y: 0, scrollLeft: 0, scrollTop: 0 });

  const startPan = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!containerRef.current) return;
    panningRef.current = true;
    setPanning(true);
    panOrigin.current = {
      x: e.clientX,
      y: e.clientY,
      scrollLeft: containerRef.current.scrollLeft,
      scrollTop: containerRef.current.scrollTop,
    };
  };

  const doPan = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!panningRef.current || !containerRef.current) return;
    containerRef.current.scrollLeft = panOrigin.current.scrollLeft - (e.clientX - panOrigin.current.x);
    containerRef.current.scrollTop  = panOrigin.current.scrollTop  - (e.clientY - panOrigin.current.y);
  };

  const stopPan = () => {
    panningRef.current = false;
    setPanning(false);
  };

  // Re-render the diagram whenever the visible schema selection changes
  useEffect(() => {
    if (visibleSchemas.size === 0) {
      setRawSvg("");
      return;
    }

    let cancelled = false;
    const renderId = `${baseId}_${++renderCount.current}`;
    const src = buildMermaid(data, visibleSchemas);

    setRendering(true);
    setError(null);

    mermaid
      .render(renderId, src)
      .then(({ svg: rendered }) => {
        if (!cancelled) setRawSvg(rendered);
      })
      .catch((e) => {
        if (!cancelled) setError(String(e));
      })
      .finally(() => {
        if (!cancelled) setRendering(false);
      });

    return () => {
      cancelled = true;
    };
  }, [visibleSchemas, data, baseId]);

  // Derive the display SVG by rewriting width attributes — no CSS transform needed
  const displaySvg = useMemo(() => applyZoom(rawSvg, zoom), [rawSvg, zoom]);

  const toggleSchema = (schema: string) => {
    setVisibleSchemas((prev) => {
      const next = new Set(prev);
      if (next.has(schema)) {
        next.delete(schema);
      } else {
        next.add(schema);
      }
      return next;
    });
  };

  const copyMermaid = () => {
    navigator.clipboard.writeText(buildMermaid(data, visibleSchemas));
  };

  return (
    <Modal
      open
      title={`ER Diagram — ${database}`}
      onCancel={onClose}
      footer={null}
      width="90vw"
      styles={{ body: { padding: 0 } }}
    >
      {/* Toolbar */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          padding: "8px 16px",
          borderBottom: "1px solid var(--border)",
          flexWrap: "wrap",
        }}
      >
        {/* Schema filter checkboxes */}
        <div style={{ display: "flex", gap: 12, flexWrap: "wrap", flex: 1, alignItems: "center" }}>
          {allSchemas.map((schema) => (
            <Checkbox
              key={schema}
              checked={visibleSchemas.has(schema)}
              onChange={() => toggleSchema(schema)}
            >
              <span style={{ fontSize: 12 }}>{schema}</span>
            </Checkbox>
          ))}
        </div>

        {/* Zoom + copy controls */}
        <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
          <Button
            size="small"
            icon={<ZoomOutOutlined />}
            onClick={() => setZoom((z) => Math.max(0.25, +(z - 0.25).toFixed(2)))}
          />
          <Button size="small" onClick={() => setZoom(1)} style={{ minWidth: 50, fontSize: 12 }}>
            {Math.round(zoom * 100)}%
          </Button>
          <Button
            size="small"
            icon={<ZoomInOutlined />}
            onClick={() => setZoom((z) => Math.min(4, +(z + 0.25).toFixed(2)))}
          />
          <Button size="small" icon={<CopyOutlined />} onClick={copyMermaid}>
            Copy Mermaid
          </Button>
        </div>
      </div>

      {/* Diagram area — drag to pan */}
      <div
        ref={containerRef}
        onMouseDown={startPan}
        onMouseMove={doPan}
        onMouseUp={stopPan}
        onMouseLeave={stopPan}
        style={{
          height: "70vh",
          overflow: "auto",
          background: "var(--bg)",
          padding: 16,
          cursor: panning ? "grabbing" : "grab",
          userSelect: panning ? "none" : "auto",
        }}
      >
        {rendering && (
          <div style={{ textAlign: "center", padding: "80px 0" }}>
            <Spin />
            <div style={{ marginTop: 12, fontSize: 12, color: "var(--text-muted)" }}>
              Rendering diagram…
            </div>
          </div>
        )}

        {!rendering && error && (
          <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12, padding: 8 }}>
            {error}
          </div>
        )}

        {!rendering && visibleSchemas.size === 0 && (
          <div
            style={{
              textAlign: "center",
              padding: "80px 0",
              color: "var(--text-muted)",
              fontSize: 13,
            }}
          >
            Select at least one schema to show the diagram.
          </div>
        )}

        {!rendering && !error && displaySvg && (
          // eslint-disable-next-line react/no-danger
          <div dangerouslySetInnerHTML={{ __html: displaySvg }} />
        )}
      </div>
    </Modal>
  );
}
