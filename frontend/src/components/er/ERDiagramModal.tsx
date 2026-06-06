// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import { useState, useEffect, useMemo } from "react";
import { Modal, Button, Checkbox } from "antd";
import { CopyOutlined, EditOutlined } from "@ant-design/icons";
import { ListSchemas } from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { buildMermaid } from "./buildMermaid";
import ERDesigner from "./ERDesigner";
import ERCanvas from "./ERCanvas";
import { initFromERData } from "./erCanvasLayout";

interface Props {
  database: string;
  data: snowflake.ERDiagramData;
  onClose: () => void;
  onDesignerSuccess?: () => void;
}

export default function ERDiagramModal({ database, data, onClose, onDesignerSuccess }: Props) {
  const dataSchemas = useMemo(
    () => [...new Set(data.tables.map((t) => t.schema))],
    [data],
  );

  const [dbSchemas, setDbSchemas] = useState<string[]>([]);
  useEffect(() => {
    ListSchemas(database).then(setDbSchemas).catch(() => {});
  }, [database]);

  // Merge schemas from ER data with all database schemas, excluding INFORMATION_SCHEMA
  const allSchemas = useMemo(
    () => [...new Set([...dataSchemas, ...dbSchemas])]
      .filter((s) => s.toUpperCase() !== "INFORMATION_SCHEMA")
      .sort(),
    [dataSchemas, dbSchemas],
  );

  const [visibleSchemas, setVisibleSchemas] = useState<Set<string>>(new Set(dataSchemas));
  const [designerOpen, setDesignerOpen] = useState(false);

  const designerTables = useMemo(() => initFromERData(data), [data]);

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
    <>
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

        {/* Copy + design controls */}
        <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
          <Button size="small" icon={<CopyOutlined />} onClick={copyMermaid}>
            Copy Mermaid
          </Button>
          <Button size="small" icon={<EditOutlined />} onClick={() => setDesignerOpen(true)}>
            Design Tables…
          </Button>
        </div>
      </div>

      {/* Canvas area */}
      <div style={{ height: "70vh" }}>
        <ERCanvas
          tables={designerTables}
          mode="readonly"
          database={database}
          visibleSchemas={visibleSchemas}
        />
      </div>
    </Modal>

    {designerOpen && (
      <ERDesigner
        database={database}
        initialData={data}
        onClose={() => setDesignerOpen(false)}
        onSuccess={() => {
          setDesignerOpen(false);
          onDesignerSuccess?.();
        }}
      />
    )}
    </>
  );
}
