// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useCallback, useEffect, useState } from "react";
import { App as AntApp, Button, Dropdown, Table, Tooltip, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { CopyOutlined, MoreOutlined } from "@ant-design/icons";
import { DropImageRepository, ListImageRepositories, ListImagesInRepository } from "../../../wailsjs/go/app/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { friendlyError } from "../common/errors";
import CreateImageRepositoryModal from "../imagerepository/CreateImageRepositoryModal";
import ImageRepositoryPropertiesModal from "../imagerepository/ImageRepositoryPropertiesModal";
import RegistryCommands from "../imagerepository/RegistryCommands";
import ContainerTabLayout from "./ContainerTabLayout";
import ScopedCreate from "./ScopedCreate";
import { rowsOf, type ResultRow } from "./computePools";

const { Text } = Typography;
const fqn = (r: ResultRow) => `${r.database_name}.${r.schema_name}.${r.name}`;

const copyBtn = (text: string, tip: string) => (
  <Tooltip title={tip}>
    <Button size="small" type="text" icon={<CopyOutlined style={{ fontSize: 12 }} />}
      onClick={(e) => { e.stopPropagation(); ClipboardSetText(text); }} />
  </Tooltip>
);

/** Images of one repository, each with a copyable pull command, plus the registry commands. */
function RepoDetail({ repo }: { repo: ResultRow }) {
  const [images, setImages] = useState<ResultRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let live = true;
    setLoading(true);
    setError(null);
    ListImagesInRepository(repo.database_name, repo.schema_name, repo.name)
      .then((res) => live && setImages(rowsOf(res)))
      .catch((e) => live && (setImages([]), setError(friendlyError(e))))
      .finally(() => live && setLoading(false));
    return () => { live = false; };
  }, [repo.database_name, repo.schema_name, repo.name]);

  const host = repo.repository_url?.split("/")[0];
  const columns: ColumnsType<ResultRow> = [
    { title: "Image", dataIndex: "image_name", ellipsis: true },
    { title: "Tags", dataIndex: "tags", ellipsis: true },
    { title: "Digest", dataIndex: "digest", ellipsis: true },
    { title: "Created", dataIndex: "created_on", width: 180, ellipsis: true },
    {
      key: "pull", width: 40,
      render: (_, r) => r.image_path && host ? copyBtn(`docker pull ${host}/${r.image_path}`, "Copy pull command") : null,
    },
  ];
  return (
    <div>
      <Text strong style={{ fontSize: 12 }}>Images in {repo.name}</Text>
      <Table<ResultRow> size="small" rowKey={(r) => r.digest || r.image_path || r.image_name}
        columns={columns} dataSource={images} loading={loading} pagination={false} scroll={{ y: 200 }}
        locale={{ emptyText: error ?? "No images in this repository." }} style={{ margin: "6px 0 12px" }} />
      {repo.repository_url && <RegistryCommands repositoryUrl={repo.repository_url} />}
    </div>
  );
}

/** Images tab of the Container Services dialog (issue #943). */
export default function ImagesTab() {
  const { modal, message } = AntApp.useApp();
  const [rows, setRows] = useState<ResultRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [props, setProps] = useState<ResultRow | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setRows(rowsOf(await ListImageRepositories()).map((r) => ({ ...r, fqn: fqn(r) })));
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { load(); }, [load]);

  const confirmDrop = (r: ResultRow) => modal.confirm({
    title: `Drop image repository ${r.name}?`,
    content: "The repository and every image in it are deleted.",
    okText: "Drop",
    okButtonProps: { danger: true },
    onOk: async () => {
      try {
        await DropImageRepository(r.database_name, r.schema_name, r.name);
      } catch (e) {
        message.error(friendlyError(e));
        throw e;
      }
      message.success(`Dropped ${r.name}`);
      load();
    },
  });

  const columns: ColumnsType<ResultRow> = [
    { title: "Name", dataIndex: "fqn", ellipsis: true },
    {
      title: "Repository URL", dataIndex: "repository_url", ellipsis: true,
      render: (u: string) => u && <>{u}{copyBtn(u, "Copy URL")}</>,
    },
    { title: "Owner", dataIndex: "owner", width: 140, ellipsis: true },
    { title: "Comment", dataIndex: "comment", ellipsis: true },
    {
      key: "menu", width: 40,
      render: (_, r) => (
        <Dropdown trigger={["click"]} menu={{
          items: [
            { key: "props", label: "Properties…", onClick: () => setProps(r) },
            { type: "divider" },
            { key: "drop", label: "Drop", danger: true, onClick: () => confirmDrop(r) },
          ],
        }}>
          <Button size="small" type="text" icon={<MoreOutlined />} onClick={(e) => e.stopPropagation()} />
        </Dropdown>
      ),
    },
  ];

  return (
    <>
      <ContainerTabLayout<ResultRow>
        objectLabel="image repositories" rowKey="fqn" columns={columns} rows={rows}
        loading={loading} error={error} onRefresh={load}
        newLabel="New repository…" onNew={() => setCreating(true)}
        filter={(r, q) => [r.fqn, r.repository_url, r.owner, r.comment].some((v) => v?.toLowerCase().includes(q))}
        detail={(r) => <RepoDetail repo={r} />}
      />
      {creating && (
        <ScopedCreate title="New image repository" onClose={() => setCreating(false)}>
          {(db, schema) => (
            <CreateImageRepositoryModal db={db} schema={schema} onClose={() => setCreating(false)} onSuccess={load} />
          )}
        </ScopedCreate>
      )}
      {props && (
        <ImageRepositoryPropertiesModal
          db={props.database_name} schema={props.schema_name} name={props.name}
          onClose={() => { setProps(null); load(); }}
        />
      )}
    </>
  );
}
