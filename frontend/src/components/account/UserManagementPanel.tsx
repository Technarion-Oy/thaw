// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useRef, useCallback } from "react";
import { Input, Spin, Button, Modal, Typography, message, Tag, Space } from "antd";
import {
  UserOutlined,
  UserAddOutlined,
  ReloadOutlined,
  FileOutlined,
  DeleteOutlined,
  StopOutlined,
  CheckCircleOutlined,
  SearchOutlined,
  KeyOutlined,
} from "@ant-design/icons";
import { ListUsers, ExecuteQuery } from "../../../wailsjs/go/app/App";
import { useSessionStore } from "../../store/sessionStore";
import type { snowflake } from "../../../wailsjs/go/models";
import UserPropertiesModal from "./UserPropertiesModal";
import CreateUserModal from "./CreateUserModal";
import KeyPairAuthModal from "./KeyPairAuthModal";

const { Text } = Typography;

interface CtxMenu {
  x: number;
  y: number;
  user: snowflake.SnowflakeUser;
}

export default function UserManagementPanel() {
  const role = useSessionStore((s) => s.role);
  const [users,          setUsers]          = useState<snowflake.SnowflakeUser[]>([]);
  const [accessible,     setAccessible]     = useState<boolean | null>(null); // null = loading
  const [loading,        setLoading]        = useState(false);
  const [search,         setSearch]         = useState("");
  const [ctxMenu,        setCtxMenu]        = useState<CtxMenu | null>(null);
  const [editUser,       setEditUser]       = useState<snowflake.SnowflakeUser | null>(null);
  const [showCreate,     setShowCreate]     = useState(false);
  const [keyPairUser,    setKeyPairUser]    = useState<string | null>(null);
  const ctxRef = useRef<HTMLDivElement>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await ListUsers();
      setUsers((result ?? []).sort((a, b) => a.name.localeCompare(b.name)));
      setAccessible(true);
    } catch {
      setAccessible(false);
    } finally {
      setLoading(false);
    }
  }, []);

  // Re-run whenever the active role changes so the list refreshes.
  useEffect(() => { load(); }, [load, role]);

  // Close context menu on outside click.
  useEffect(() => {
    if (!ctxMenu) return;
    const close = (e: MouseEvent) => {
      if (ctxRef.current && !ctxRef.current.contains(e.target as Node)) {
        setCtxMenu(null);
      }
    };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [ctxMenu]);

  if (accessible === false) return null;
  if (accessible === null)  return <Spin size="small" style={{ display: "block", margin: "8px auto" }} />;

  const filtered = search.trim()
    ? users.filter((u) => {
        const q = search.toLowerCase();
        return (
          u.name.toLowerCase().includes(q) ||
          u.loginName.toLowerCase().includes(q) ||
          u.email.toLowerCase().includes(q) ||
          u.displayName.toLowerCase().includes(q)
        );
      })
    : users;

  const esc = (s: string) => s.replace(/"/g, '""');

  const handleToggleDisable = async (user: snowflake.SnowflakeUser) => {
    setCtxMenu(null);
    const action = user.disabled ? "FALSE" : "TRUE";
    const verb   = user.disabled ? "Enabled" : "Disabled";
    try {
      await ExecuteQuery(`ALTER USER "${esc(user.name)}" SET DISABLED = ${action};`);
      message.success(`${verb} user ${user.name}`);
      load();
    } catch (e) {
      message.error(String(e));
    }
  };

  const handleDrop = (user: snowflake.SnowflakeUser) => {
    setCtxMenu(null);
    Modal.confirm({
      title: `Drop user ${user.name}?`,
      content: "This cannot be undone.",
      okText: "Drop",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await ExecuteQuery(`DROP USER "${esc(user.name)}";`);
          message.success(`Dropped user ${user.name}`);
          load();
        } catch (e) {
          message.error(String(e));
        }
      },
    });
  };

  const menuItem = (
    label: string,
    icon: React.ReactNode,
    onClick: () => void,
    color?: string,
  ) => (
    <div
      key={label}
      onClick={onClick}
      style={{
        padding: "5px 12px",
        cursor: "pointer",
        fontSize: 12,
        color: color ?? "var(--text)",
        display: "flex",
        alignItems: "center",
        gap: 7,
        borderRadius: 4,
      }}
      onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-hover)")}
      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
    >
      {icon}
      {label}
    </div>
  );

  return (
    <div style={{ borderTop: "1px solid var(--border)", padding: "4px 4px 4px" }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "0 4px 4px 8px" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 5 }}>
          <UserOutlined style={{ fontSize: 12, color: "var(--text-muted)" }} />
          <Text style={{ fontSize: 11, color: "var(--text-muted)", textTransform: "uppercase", letterSpacing: "0.08em" }}>
            Users ({users.length})
          </Text>
        </div>
        <Space size={2}>
          <Button
            size="small"
            type="text"
            icon={<UserAddOutlined style={{ fontSize: 11 }} />}
            title="Create user"
            onClick={() => setShowCreate(true)}
            style={{ height: 18, padding: "0 4px", minWidth: 0 }}
          />
          <Button
            size="small"
            type="text"
            icon={<ReloadOutlined style={{ fontSize: 11 }} />}
            loading={loading}
            onClick={load}
            style={{ height: 18, padding: "0 4px", minWidth: 0 }}
          />
        </Space>
      </div>

      {/* Search */}
      <div style={{ padding: "0 4px 6px" }}>
        <Input
          size="small"
          placeholder="Search users…"
          prefix={<SearchOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} />}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          allowClear
        />
      </div>

      {/* List */}
      <div style={{ maxHeight: 220, overflowY: "auto" }}>
        {filtered.length === 0 && !loading && (
          <Text style={{ fontSize: 11, color: "var(--text-muted)", display: "block", padding: "4px 8px" }}>
            {search ? "No matches" : "No users"}
          </Text>
        )}
        {filtered.map((u) => (
          <div
            key={u.name}
            draggable
            onDragStart={(e) => {
              e.dataTransfer.setData("thaw/user", JSON.stringify({ name: u.name }));
              e.dataTransfer.effectAllowed = "copy";
            }}
            onContextMenu={(e) => {
              e.preventDefault();
              e.stopPropagation();
              // Clamp inside viewport
              const menuW = 200, menuH = 140;
              const x = Math.min(e.clientX, window.innerWidth  - menuW - 8);
              const y = Math.min(e.clientY, window.innerHeight - menuH - 8);
              setCtxMenu({ x, y, user: u });
            }}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 6,
              padding: "3px 8px",
              borderRadius: 4,
              cursor: "grab",
              userSelect: "none",
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-hover)")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
          >
            <UserOutlined style={{ fontSize: 11, color: u.disabled ? "var(--text-muted)" : "var(--text-secondary)", flexShrink: 0 }} />
            <Text style={{ fontSize: 12, color: u.disabled ? "var(--text-muted)" : "var(--text)", flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {u.name}
            </Text>
            {u.disabled && (
              <Tag color="default" style={{ fontSize: 10, padding: "0 4px", margin: 0, lineHeight: "16px" }}>
                disabled
              </Tag>
            )}
          </div>
        ))}
      </div>

      {/* Context menu */}
      {ctxMenu && (
        <div
          ref={ctxRef}
          style={{
            position: "fixed",
            top: ctxMenu.y,
            left: ctxMenu.x,
            zIndex: 9999,
            background: "var(--bg-overlay)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            boxShadow: "0 4px 16px rgba(0,0,0,0.5)",
            minWidth: 170,
            padding: "4px 0",
          }}
          onClick={(e) => e.stopPropagation()}
        >
          {menuItem("Properties", <FileOutlined style={{ fontSize: 12 }} />, () => {
            const u = ctxMenu.user;
            setCtxMenu(null);
            setEditUser(u);
          })}
          {menuItem(
            ctxMenu.user.disabled ? "Enable" : "Disable",
            ctxMenu.user.disabled
              ? <CheckCircleOutlined style={{ fontSize: 12 }} />
              : <StopOutlined style={{ fontSize: 12 }} />,
            () => handleToggleDisable(ctxMenu.user),
          )}
          {menuItem(
            "Configure Key Pair Auth…",
            <KeyOutlined style={{ fontSize: 12 }} />,
            () => { setCtxMenu(null); setKeyPairUser(ctxMenu.user.name); },
          )}
          <div style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
          {menuItem("Drop…", <DeleteOutlined style={{ fontSize: 12, color: "#f85149" }} />, () => handleDrop(ctxMenu.user), "#f85149")}
        </div>
      )}

      {/* Properties modal — edits save per-property; refresh the list on close */}
      {editUser && (
        <UserPropertiesModal
          name={editUser.name}
          onClose={() => { setEditUser(null); load(); }}
        />
      )}

      {/* Create modal */}
      {showCreate && (
        <CreateUserModal
          onClose={() => setShowCreate(false)}
          onSuccess={() => { setShowCreate(false); load(); }}
        />
      )}

      {/* Key pair auth modal */}
      {keyPairUser && (
        <KeyPairAuthModal
          username={keyPairUser}
          onClose={() => setKeyPairUser(null)}
        />
      )}
    </div>
  );
}
