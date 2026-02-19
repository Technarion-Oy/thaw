import { useState } from "react";
import { Form, Input, Button, Alert, Space, Typography } from "antd";
import { CloudServerOutlined } from "@ant-design/icons";
import { Connect } from "../../../wailsjs/go/main/App";
import { useConnectionStore, type ConnectionParams } from "../../store/connectionStore";

const { Title } = Typography;

export default function ConnectModal() {
  const [form] = Form.useForm<ConnectionParams>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const setConnected = useConnectionStore((s) => s.setConnected);

  const onFinish = async (values: ConnectionParams) => {
    setLoading(true);
    setError(null);
    try {
      await Connect(values);
      setConnected(values);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        height: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "#0d1117",
      }}
    >
      <div style={{ width: 420 }}>
        <Space direction="vertical" size={24} style={{ width: "100%" }}>
          <Space align="center">
            <CloudServerOutlined style={{ fontSize: 28, color: "#29B6F6" }} />
            <Title level={3} style={{ margin: 0, color: "#e6edf3" }}>
              Connect to Snowflake
            </Title>
          </Space>

          {error && <Alert type="error" message={error} showIcon />}

          <Form form={form} layout="vertical" onFinish={onFinish} requiredMark={false}>
            <Form.Item name="account" label="Account" rules={[{ required: true }]}>
              <Input placeholder="myorg-myaccount" />
            </Form.Item>
            <Form.Item name="user" label="Username" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Form.Item name="password" label="Password" rules={[{ required: true }]}>
              <Input.Password />
            </Form.Item>

            <Space style={{ width: "100%" }}>
              <Form.Item name="role" label="Role">
                <Input placeholder="SYSADMIN" />
              </Form.Item>
              <Form.Item name="warehouse" label="Warehouse">
                <Input placeholder="COMPUTE_WH" />
              </Form.Item>
            </Space>

            <Space style={{ width: "100%" }}>
              <Form.Item name="database" label="Database">
                <Input placeholder="optional" />
              </Form.Item>
              <Form.Item name="schema" label="Schema">
                <Input placeholder="optional" />
              </Form.Item>
            </Space>

            <Form.Item style={{ marginBottom: 0 }}>
              <Button type="primary" htmlType="submit" loading={loading} block>
                Connect
              </Button>
            </Form.Item>
          </Form>
        </Space>
      </div>
    </div>
  );
}
