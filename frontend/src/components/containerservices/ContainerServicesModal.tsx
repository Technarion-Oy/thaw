// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { Alert, Button, Modal, Space, Tabs } from "antd";
import { CloudServerOutlined } from "@ant-design/icons";
import ContainerTabLayout from "./ContainerTabLayout";
import ComputePoolsTab from "./ComputePoolsTab";
import JobsTab from "./JobsTab";
import {
  CONTAINER_TABS,
  useContainerServicesStore,
  type ContainerTab,
} from "../../store/containerServicesStore";

interface Props {
  onClose: () => void;
}

// Per-tab metadata. `objectLabel` and `newLabel` also drive the shared layout's
// empty state and primary action, so a tab landing later only has to add its
// columns, its loader and its row menu. `issue` is the child issue that fills it
// in; when every tab has landed these placeholders (and the field) go away.
const TABS: Record<ContainerTab, { label: string; objectLabel: string; newLabel: string; issue: number }> = {
  pools:     { label: "Compute pools",      objectLabel: "compute pools",      newLabel: "New pool…",     issue: 941 }, // landed: ComputePoolsTab
  services:  { label: "Services",           objectLabel: "services",           newLabel: "New service…",  issue: 944 },
  jobs:      { label: "Jobs",               objectLabel: "job services",       newLabel: "Run job…",      issue: 942 }, // landed: JobsTab
  images:    { label: "Images",             objectLabel: "image repositories", newLabel: "New repo…",     issue: 943 },
  snapshots: { label: "Snapshots",          objectLabel: "volume snapshots",   newLabel: "New snapshot…", issue: 945 },
  gateways:  { label: "Gateways",           objectLabel: "gateways",           newLabel: "New gateway…",  issue: 943 },
};

/**
 * Account-wide Snowpark Container Services control center (issue #939): one wide
 * modal, one tab per Snowflake SPCS command group, all six sharing
 * `ContainerTabLayout`. This is the shell — each tab's data, columns and
 * lifecycle actions arrive with its own child issue.
 */
export default function ContainerServicesModal({ onClose }: Props) {
  const tab = useContainerServicesStore((s) => s.tab);
  const setTab = useContainerServicesStore((s) => s.setTab);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <CloudServerOutlined style={{ color: "var(--link)" }} />
          <span>Container Services</span>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={1080}
      styles={{ body: { maxHeight: "76vh", overflowY: "auto", paddingTop: 8 } }}
    >
      <Tabs
        activeKey={tab}
        onChange={(key) => setTab(key as ContainerTab)}
        items={CONTAINER_TABS.map((key) => {
          const t = TABS[key];
          return {
            key,
            label: t.label,
            children: key === "pools" ? <ComputePoolsTab /> : key === "jobs" ? <JobsTab onCloseDialog={onClose} /> : (
              <ContainerTabLayout
                objectLabel={t.objectLabel}
                rowKey="name"
                columns={[]}
                rows={[] as { name: string }[]}
                newLabel={t.newLabel}
                notice={
                  <Alert
                    type="info" showIcon banner style={{ marginBottom: 10, fontSize: 12 }}
                    message={`${t.label} are not wired up yet — this tab lands with issue #${t.issue}.`}
                  />
                }
              />
            ),
          };
        })}
      />
    </Modal>
  );
}
