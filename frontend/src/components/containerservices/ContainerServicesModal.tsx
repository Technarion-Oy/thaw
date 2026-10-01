// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { Button, Modal, Space, Tabs } from "antd";
import { CloudServerOutlined } from "@ant-design/icons";
import ComputePoolsTab from "./ComputePoolsTab";
import ServicesTab from "./ServicesTab";
import JobsTab from "./JobsTab";
import ImagesTab from "./ImagesTab";
import GatewaysTab from "./GatewaysTab";
import SnapshotsTab from "./SnapshotsTab";
import {
  CONTAINER_TABS,
  useContainerServicesStore,
  type ContainerTab,
} from "../../store/containerServicesStore";

interface Props {
  onClose: () => void;
}

const LABELS: Record<ContainerTab, string> = {
  pools: "Compute pools",
  services: "Services",
  jobs: "Jobs",
  images: "Images",
  snapshots: "Snapshots",
  gateways: "Gateways",
};

/**
 * Account-wide Snowpark Container Services control center (issue #939): one wide
 * modal, one tab per Snowflake SPCS command group, all six sharing
 * `ContainerTabLayout`.
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
        items={CONTAINER_TABS.map((key) => ({
          key,
          label: LABELS[key],
          children: {
            pools: <ComputePoolsTab />,
            services: <ServicesTab />,
            jobs: <JobsTab onCloseDialog={onClose} />,
            images: <ImagesTab />,
            snapshots: <SnapshotsTab />,
            gateways: <GatewaysTab />,
          }[key],
        }))}
      />
    </Modal>
  );
}
