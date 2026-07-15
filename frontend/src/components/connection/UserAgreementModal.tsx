// SPDX-License-Identifier: GPL-3.0-or-later

import { Modal, Typography } from "antd";

const { Title, Paragraph, Text, Link } = Typography;

interface Props {
  open: boolean;
  onClose: () => void;
}

export default function UserAgreementModal({ open, onClose }: Props) {
  return (
    <Modal
      open={open}
      title="License"
      onCancel={onClose}
      footer={null}
      width={640}
      styles={{ body: { maxHeight: "60vh", overflowY: "auto" } }}
    >
      <Typography>
        <Title level={5}>Thaw — Free Software</Title>
        <Text type="secondary" style={{ fontSize: 12 }}>
          GNU General Public License, version 3 (or later)
        </Text>

        <Paragraph style={{ marginTop: 16 }}>
          Thaw is free software: you can redistribute it and/or modify it under
          the terms of the <Text strong>GNU General Public License</Text> as
          published by the Free Software Foundation, either version 3 of the
          License, or (at your option) any later version.
        </Paragraph>

        <Paragraph>
          Thaw is distributed in the hope that it will be useful, but{" "}
          <Text strong>WITHOUT ANY WARRANTY</Text>; without even the implied
          warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See
          the GNU General Public License for more details.
        </Paragraph>

        <Paragraph>
          You should have received a copy of the GNU General Public License
          along with Thaw. If not, see{" "}
          <Link href="https://www.gnu.org/licenses/" target="_blank" rel="noreferrer">
            https://www.gnu.org/licenses/
          </Link>
          . The full license text is also available in the{" "}
          <Text code>LICENSE</Text> file at the root of the source repository.
        </Paragraph>

        <Title level={5}>Your data &amp; privacy</Title>
        <Paragraph>
          Thaw connects to Snowflake using credentials you provide. It does not
          store or transmit your Snowflake credentials or query data to any
          third party. Anonymous, non-identifying usage telemetry may be
          collected to help improve the software.
        </Paragraph>

        <Title level={5}>Copyright</Title>
        <Paragraph>
          Copyright © 2026 Technarion Oy and Thaw contributors.
        </Paragraph>
      </Typography>
    </Modal>
  );
}
