// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Collapse,
  Input,
  Modal,
  Space,
  Spin,
  Steps,
  Tooltip,
  Typography,
} from "antd";
import { FolderOpenOutlined, WarningOutlined } from "@ant-design/icons";
import {
  CreateDbtProject,
  GetGitConfig,
  ListDatabases,
  ListDirectory,
  ListSchemas,
  PickDirectory,
} from "../../../wailsjs/go/main/App";
import type { dbt } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  onClose: () => void;
}

// ─── DbtProjectModal ─────────────────────────────────────────────────────────

export default function DbtProjectModal({ onClose }: Props) {
  const [step, setStep] = useState(0);

  // Step 0 — Configure
  const [projectName, setProjectName] = useState("my_dbt_project");
  const [profileName, setProfileName] = useState("my_dbt_project");
  const [profileManuallyEdited, setProfileManuallyEdited] = useState(false);
  const [outputDir, setOutputDir] = useState("");
  const [dirExists, setDirExists] = useState(false);

  // Step 1 — Select Sources
  const [databases, setDatabases] = useState<string[]>([]);
  const [loadingDbs, setLoadingDbs] = useState(false);
  const [schemasByDb, setSchemasByDb] = useState<Record<string, string[]>>({});
  const [loadingSchemas, setLoadingSchemas] = useState<Record<string, boolean>>({});
  const [selectedSchemas, setSelectedSchemas] = useState<Record<string, Set<string>>>({});

  // Step 2 — Generate
  const [generating, setGenerating] = useState(false);
  const [result, setResult] = useState<dbt.CreateResult | null>(null);
  const [generateError, setGenerateError] = useState<string | null>(null);
  const [filesExpanded, setFilesExpanded] = useState(false);

  // Pre-fill output dir from git config on mount
  useEffect(() => {
    GetGitConfig()
      .then((cfg) => {
        if (cfg.exportDir) setOutputDir(cfg.exportDir);
      })
      .catch(() => {});
  }, []);

  // Check if target directory already exists whenever name or dir changes
  useEffect(() => {
    if (!outputDir || !projectName) {
      setDirExists(false);
      return;
    }
    const target = outputDir.replace(/\/$/, "") + "/" + projectName;
    ListDirectory(target)
      .then(() => setDirExists(true))
      .catch(() => setDirExists(false));
  }, [outputDir, projectName]);

  // ── Step 0 helpers ─────────────────────────────────────────────────────────

  function handleProjectNameChange(value: string) {
    setProjectName(value);
    if (!profileManuallyEdited) {
      setProfileName(value);
    }
  }

  function handleProfileNameChange(value: string) {
    setProfileName(value);
    setProfileManuallyEdited(true);
  }

  async function handlePickDir() {
    const dir = await PickDirectory().catch(() => "");
    if (dir) setOutputDir(dir);
  }

  function handleStep0Next() {
    setStep(1);
    if (databases.length === 0) {
      setLoadingDbs(true);
      ListDatabases()
        .then((dbs) => setDatabases(dbs ?? []))
        .catch(() => setDatabases([]))
        .finally(() => setLoadingDbs(false));
    }
  }

  // ── Step 1 helpers ─────────────────────────────────────────────────────────

  function handlePanelExpand(db: string) {
    if (schemasByDb[db] !== undefined || loadingSchemas[db]) return;
    setLoadingSchemas((prev) => ({ ...prev, [db]: true }));
    ListSchemas(db)
      .then((schemas) => {
        setSchemasByDb((prev) => ({ ...prev, [db]: schemas ?? [] }));
      })
      .catch(() => {
        setSchemasByDb((prev) => ({ ...prev, [db]: [] }));
      })
      .finally(() => {
        setLoadingSchemas((prev) => ({ ...prev, [db]: false }));
      });
  }

  function isSchemaSelected(db: string, schema: string): boolean {
    return selectedSchemas[db]?.has(schema) ?? false;
  }

  function toggleSchema(db: string, schema: string, checked: boolean) {
    setSelectedSchemas((prev) => {
      const next = { ...prev };
      const set = new Set(next[db] ?? []);
      if (checked) {
        set.add(schema);
      } else {
        set.delete(schema);
      }
      next[db] = set;
      return next;
    });
  }

  const isSystemSchema = (schema: string) =>
    schema.toUpperCase() === "INFORMATION_SCHEMA";

  function selectAllSchemas(db: string) {
    const schemas = (schemasByDb[db] ?? []).filter((s) => !isSystemSchema(s));
    setSelectedSchemas((prev) => ({ ...prev, [db]: new Set(schemas) }));
  }

  function deselectAllSchemas(db: string) {
    setSelectedSchemas((prev) => ({ ...prev, [db]: new Set() }));
  }

  function totalSelectedSchemas(): number {
    return Object.values(selectedSchemas).reduce((sum, s) => sum + s.size, 0);
  }

  // Build schemasMap: Record<db, string[]>
  function buildSchemasMap(): Record<string, string[]> {
    const map: Record<string, string[]> = {};
    for (const [db, set] of Object.entries(selectedSchemas)) {
      if (set.size > 0) {
        map[db] = [...set];
      }
    }
    return map;
  }

  // ── Step 2 helpers ─────────────────────────────────────────────────────────

  async function handleGenerate() {
    setGenerating(true);
    setGenerateError(null);
    setResult(null);

    const schemasMap = buildSchemasMap();
    const req = {
      projectName,
      outputDir,
      profileName,
    };

    try {
      const res = await CreateDbtProject(req as any, schemasMap);
      setResult(res as unknown as dbt.CreateResult);
    } catch (e: any) {
      setGenerateError(e?.message ?? String(e));
    } finally {
      setGenerating(false);
    }
  }

  // ── Summary helpers ────────────────────────────────────────────────────────

  function selectedDbCount(): number {
    return Object.values(selectedSchemas).filter((s) => s.size > 0).length;
  }

  function estimatedFileCount(): number {
    // profiles.yml + dbt_project.yml + _sources.yml + 3 gitkeeps + stub models
    // We don't know table count yet; show a minimum of the static files.
    return 6;
  }

  function projectPath(): string {
    return (outputDir.replace(/\/$/, "") + "/" + projectName).replace(/\/\//g, "/");
  }

  // ── Render steps ──────────────────────────────────────────────────────────

  function renderStep0() {
    const canNext = projectName.trim() !== "" && profileName.trim() !== "" && outputDir.trim() !== "";

    return (
      <Space direction="vertical" style={{ width: "100%", gap: 16 }}>
        <div>
          <Text strong style={{ display: "block", marginBottom: 4 }}>
            Project Name
          </Text>
          <Input
            value={projectName}
            onChange={(e) => handleProjectNameChange(e.target.value)}
            placeholder="my_dbt_project"
          />
        </div>

        <div>
          <Text strong style={{ display: "block", marginBottom: 4 }}>
            Profile Name
          </Text>
          <Input
            value={profileName}
            onChange={(e) => handleProfileNameChange(e.target.value)}
            placeholder="my_dbt_project"
          />
          <Text type="secondary" style={{ fontSize: 12 }}>
            Mirrors the project name by default. Used in <code>profiles.yml</code> and{" "}
            <code>dbt_project.yml</code>.
          </Text>
        </div>

        <div>
          <Text strong style={{ display: "block", marginBottom: 4 }}>
            Output Directory
          </Text>
          <Space.Compact style={{ width: "100%" }}>
            <Input
              value={outputDir}
              onChange={(e) => setOutputDir(e.target.value)}
              placeholder="/path/to/output"
              style={{ fontFamily: "monospace" }}
              readOnly
            />
            <Button icon={<FolderOpenOutlined />} onClick={handlePickDir}>
              Browse…
            </Button>
          </Space.Compact>
          {outputDir && projectName && (
            <Text type="secondary" style={{ fontSize: 12, display: "block", marginTop: 4 }}>
              Project will be created at: <code>{projectPath()}</code>
            </Text>
          )}
        </div>

        {dirExists && (
          <Alert
            type="warning"
            showIcon
            message="Directory already exists"
            description={`A directory named "${projectName}" already exists in the selected output folder. Files will be overwritten.`}
          />
        )}

        <div style={{ display: "flex", justifyContent: "flex-end" }}>
          <Button type="primary" onClick={handleStep0Next} disabled={!canNext}>
            Next →
          </Button>
        </div>
      </Space>
    );
  }

  function renderStep1() {
    const hasSelection = totalSelectedSchemas() > 0;

    return (
      <Space direction="vertical" style={{ width: "100%", gap: 16 }}>
        <Text type="secondary">
          Select the schemas to include as dbt sources. Expand a database panel to load its
          schemas.
        </Text>

        {loadingDbs ? (
          <div style={{ textAlign: "center", padding: 24 }}>
            <Spin />
          </div>
        ) : (
          <Collapse
            accordion={false}
            onChange={(keys) => {
              const opened = Array.isArray(keys) ? keys : [keys];
              opened.forEach((db) => handlePanelExpand(db as string));
            }}
            items={databases.map((db) => {
              const schemas = schemasByDb[db] ?? [];
              const loading = loadingSchemas[db] ?? false;
              const selected = selectedSchemas[db] ?? new Set<string>();
              const regularSchemas = schemas.filter((s) => !isSystemSchema(s));
              const allSelected =
                regularSchemas.length > 0 &&
                regularSchemas.every((s) => selected.has(s));

              return {
                key: db,
                label: (
                  <span>
                    <strong>{db}</strong>
                    {selected.size > 0 && (
                      <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>
                        ({selected.size} schema{selected.size !== 1 ? "s" : ""} selected)
                      </Text>
                    )}
                  </span>
                ),
                children: loading ? (
                  <div style={{ textAlign: "center", padding: 16 }}>
                    <Spin size="small" />
                  </div>
                ) : schemas.length === 0 ? (
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    No schemas found.
                  </Text>
                ) : (
                  <Space direction="vertical" style={{ width: "100%" }}>
                    <Space size="small">
                      <Button
                        size="small"
                        type="link"
                        style={{ padding: 0 }}
                        onClick={() => (allSelected ? deselectAllSchemas(db) : selectAllSchemas(db))}
                      >
                        {allSelected ? "Deselect all" : "Select all"}
                      </Button>
                    </Space>
                    <Space wrap>
                      {schemas.map((schema) => {
                        const system = isSystemSchema(schema);
                        return system ? (
                          <Tooltip
                            key={schema}
                            title="System schema — contains Snowflake metadata. Include only if your models reference it directly. No staging stubs will be generated."
                          >
                            <Checkbox
                              checked={isSchemaSelected(db, schema)}
                              onChange={(e) => toggleSchema(db, schema, e.target.checked)}
                            >
                              <Text type="secondary" italic>
                                <WarningOutlined style={{ marginRight: 4, color: "var(--ant-color-warning)" }} />
                                {schema}
                              </Text>
                            </Checkbox>
                          </Tooltip>
                        ) : (
                          <Checkbox
                            key={schema}
                            checked={isSchemaSelected(db, schema)}
                            onChange={(e) => toggleSchema(db, schema, e.target.checked)}
                          >
                            {schema}
                          </Checkbox>
                        );
                      })}
                    </Space>
                  </Space>
                ),
              };
            })}
          />
        )}

        <div style={{ display: "flex", justifyContent: "space-between" }}>
          <Button onClick={() => setStep(0)}>← Back</Button>
          <Button
            type="primary"
            disabled={!hasSelection}
            onClick={() => setStep(2)}
          >
            Preview →
          </Button>
        </div>
      </Space>
    );
  }

  function renderStep2() {
    const dbCount = selectedDbCount();
    const schemaCount = totalSelectedSchemas();

    if (result) {
      // Group files by directory prefix
      const grouped: Record<string, string[]> = {};
      for (const f of result.filesCreated) {
        const parts = f.split("/");
        const dir = parts.length > 1 ? parts.slice(0, -1).join("/") : ".";
        grouped[dir] = grouped[dir] ?? [];
        grouped[dir].push(parts[parts.length - 1]);
      }

      return (
        <Space direction="vertical" style={{ width: "100%", gap: 16 }}>
          <Alert
            type="success"
            showIcon
            message="Project created successfully"
            description={`${result.filesCreated.length} file${result.filesCreated.length !== 1 ? "s" : ""} written to ${result.projectDir}`}
          />

          {result.warnings.length > 0 && (
            <Alert
              type="warning"
              showIcon
              message={`${result.warnings.length} warning${result.warnings.length !== 1 ? "s" : ""}`}
              description={
                <ul style={{ margin: 0, paddingLeft: 16 }}>
                  {result.warnings.map((w, i) => (
                    <li key={i}>{w}</li>
                  ))}
                </ul>
              }
            />
          )}

          <div>
            <Button
              type="link"
              style={{ padding: 0, marginBottom: 8 }}
              onClick={() => setFilesExpanded((v) => !v)}
            >
              {filesExpanded ? "Hide file list" : "Show file list"}
            </Button>
            {filesExpanded && (
              <div
                style={{
                  fontFamily: "monospace",
                  fontSize: 12,
                  background: "var(--ant-color-fill-quaternary, #f5f5f5)",
                  borderRadius: 4,
                  padding: "8px 12px",
                  maxHeight: 240,
                  overflowY: "auto",
                }}
              >
                {Object.entries(grouped)
                  .sort(([a], [b]) => a.localeCompare(b))
                  .map(([dir, files]) => (
                    <div key={dir} style={{ marginBottom: 4 }}>
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        {dir}/
                      </Text>
                      {files.map((f) => (
                        <div key={f} style={{ paddingLeft: 16 }}>
                          {f}
                        </div>
                      ))}
                    </div>
                  ))}
              </div>
            )}
          </div>

          <Alert
            type="info"
            showIcon
            message="Next steps"
            description={
              <>
                <code>profiles.yml</code> has been written to the project root. Copy it to{" "}
                <code>~/.dbt/profiles.yml</code> when you're ready to run dbt commands.
              </>
            }
          />

          <div style={{ display: "flex", justifyContent: "flex-end" }}>
            <Button type="primary" onClick={onClose}>
              Done
            </Button>
          </div>
        </Space>
      );
    }

    if (generateError) {
      return (
        <Space direction="vertical" style={{ width: "100%", gap: 16 }}>
          <Alert
            type="error"
            showIcon
            message="Generation failed"
            description={generateError}
          />
          <div style={{ display: "flex", justifyContent: "flex-start" }}>
            <Button onClick={() => { setGenerateError(null); setStep(0); }}>
              ← Back
            </Button>
          </div>
        </Space>
      );
    }

    return (
      <Space direction="vertical" style={{ width: "100%", gap: 16 }}>
        <div>
          <Text strong style={{ display: "block", marginBottom: 8 }}>
            Summary
          </Text>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
            <tbody>
              <tr>
                <td style={{ padding: "4px 0", color: "var(--ant-color-text-secondary)" }}>
                  Project path
                </td>
                <td style={{ padding: "4px 0", fontFamily: "monospace", fontSize: 12 }}>
                  {projectPath()}
                </td>
              </tr>
              <tr>
                <td style={{ padding: "4px 0", color: "var(--ant-color-text-secondary)" }}>
                  Profile name
                </td>
                <td style={{ padding: "4px 0" }}>{profileName}</td>
              </tr>
              <tr>
                <td style={{ padding: "4px 0", color: "var(--ant-color-text-secondary)" }}>
                  Databases
                </td>
                <td style={{ padding: "4px 0" }}>{dbCount}</td>
              </tr>
              <tr>
                <td style={{ padding: "4px 0", color: "var(--ant-color-text-secondary)" }}>
                  Schemas
                </td>
                <td style={{ padding: "4px 0" }}>{schemaCount}</td>
              </tr>
              <tr>
                <td style={{ padding: "4px 0", color: "var(--ant-color-text-secondary)" }}>
                  Min. files
                </td>
                <td style={{ padding: "4px 0" }}>
                  {estimatedFileCount()} + one stub per table/view
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <Button onClick={() => setStep(1)}>← Back</Button>
          <Button type="primary" loading={generating} onClick={handleGenerate}>
            {generating ? "Creating project files…" : "Generate Project"}
          </Button>
        </div>
      </Space>
    );
  }

  // ── Modal render ──────────────────────────────────────────────────────────

  const stepTitles = ["Configure", "Select Sources", "Generate"];

  return (
    <Modal
      open
      title="Create dbt Project"
      width={640}
      onCancel={onClose}
      footer={null}
      destroyOnClose
    >
      <Steps
        current={step}
        size="small"
        style={{ marginBottom: 24 }}
        items={stepTitles.map((title) => ({ title }))}
      />

      {step === 0 && renderStep0()}
      {step === 1 && renderStep1()}
      {step === 2 && renderStep2()}
    </Modal>
  );
}
