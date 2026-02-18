import { Layout } from "antd";
import Sidebar from "./Sidebar";
import QueryPage from "../../pages/QueryPage";

const { Sider, Content } = Layout;

export default function AppLayout() {
  return (
    <Layout style={{ height: "100vh" }}>
      {/* macOS traffic-light drag area */}
      <div
        className="titlebar-drag"
        style={{ height: 28, background: "#161b22", position: "fixed", top: 0, left: 0, right: 0, zIndex: 100 }}
      />

      <Sider
        width={240}
        style={{
          background: "#161b22",
          borderRight: "1px solid #30363d",
          paddingTop: 28,
          overflow: "auto",
        }}
      >
        <Sidebar />
      </Sider>

      <Content style={{ paddingTop: 28, overflow: "hidden", display: "flex", flexDirection: "column" }}>
        <QueryPage />
      </Content>
    </Layout>
  );
}
