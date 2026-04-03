// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { Modal, Typography } from "antd";

const { Title, Paragraph, Text } = Typography;

interface Props {
  open: boolean;
  onClose: () => void;
}

export default function UserAgreementModal({ open, onClose }: Props) {
  return (
    <Modal
      open={open}
      title="User Agreement"
      onCancel={onClose}
      footer={null}
      width={640}
      styles={{ body: { maxHeight: "60vh", overflowY: "auto" } }}
    >
      <Typography>
        <Title level={5}>Thaw — End User License Agreement</Title>
        <Text type="secondary" style={{ fontSize: 12 }}>
          Last updated: April 2026
        </Text>

        <Paragraph style={{ marginTop: 16 }}>
          This End User License Agreement ("Agreement") is a legal agreement between you ("User") and
          Technarion Oy ("Company") for the use of the Thaw desktop application ("Software").
          By installing or using the Software, you agree to be bound by the terms of this Agreement.
        </Paragraph>

        <Title level={5}>1. License Grant</Title>
        <Paragraph>
          Subject to the terms of this Agreement, Company grants you a limited, non-exclusive,
          non-transferable, revocable license to install and use the Software solely for your
          internal business purposes in accordance with your subscription plan.
        </Paragraph>

        <Title level={5}>2. Restrictions</Title>
        <Paragraph>
          You may not: (a) copy, modify, or distribute the Software; (b) reverse engineer,
          decompile, or disassemble the Software; (c) sublicense, rent, lease, or transfer the
          Software or your rights under this Agreement; (d) use the Software to provide services
          to third parties without prior written consent from Company.
        </Paragraph>

        <Title level={5}>3. Ownership</Title>
        <Paragraph>
          The Software is licensed, not sold. Company retains all intellectual property rights in
          and to the Software, including all copies, modifications, and derivative works thereof.
        </Paragraph>

        <Title level={5}>4. Data & Privacy</Title>
        <Paragraph>
          The Software connects to Snowflake services using credentials you provide. Company does
          not store or transmit your Snowflake credentials or query data. Usage telemetry (aggregate,
          non-identifying) may be collected to improve the Software. See our Privacy Policy for details.
        </Paragraph>

        <Title level={5}>5. Disclaimer of Warranties</Title>
        <Paragraph>
          THE SOFTWARE IS PROVIDED "AS IS" WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED,
          INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A
          PARTICULAR PURPOSE, AND NON-INFRINGEMENT.
        </Paragraph>

        <Title level={5}>6. Limitation of Liability</Title>
        <Paragraph>
          TO THE MAXIMUM EXTENT PERMITTED BY APPLICABLE LAW, IN NO EVENT SHALL COMPANY BE
          LIABLE FOR ANY INDIRECT, INCIDENTAL, SPECIAL, CONSEQUENTIAL, OR PUNITIVE DAMAGES
          ARISING OUT OF OR RELATED TO YOUR USE OF THE SOFTWARE.
        </Paragraph>

        <Title level={5}>7. Termination</Title>
        <Paragraph>
          This Agreement is effective until terminated. Your rights under this Agreement will
          terminate automatically if you fail to comply with any of its terms. Upon termination,
          you must destroy all copies of the Software in your possession.
        </Paragraph>

        <Title level={5}>8. Governing Law</Title>
        <Paragraph>
          This Agreement shall be governed by and construed in accordance with the laws of Finland,
          without regard to its conflict of law provisions.
        </Paragraph>

        <Title level={5}>9. Contact</Title>
        <Paragraph>
          For questions about this Agreement, please contact Technarion Oy.
        </Paragraph>
      </Typography>
    </Modal>
  );
}
