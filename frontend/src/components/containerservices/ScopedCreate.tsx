// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useEffect, useState } from "react";
import type { ReactNode } from "react";
import { Button, Form, Modal, Select } from "antd";
import { ListDatabases, ListUserSchemas } from "../../../wailsjs/go/app/App";
import { useSessionStore } from "../../store/sessionStore";

interface Props {
  title: string;
  onClose: () => void;
  /** Renders the schema-scoped create modal once a database and schema are chosen. */
  children: (db: string, schema: string) => ReactNode;
}

/**
 * Account-scope entry for the schema-scoped create modals: pick a database and
 * schema (defaulting to the session's), then hand over to the real modal.
 */
export default function ScopedCreate({ title, onClose, children }: Props) {
  const sess = useSessionStore();
  const [dbs, setDbs] = useState<string[]>([]);
  const [schemas, setSchemas] = useState<string[]>([]);
  const [db, setDb] = useState(sess.database);
  const [schema, setSchema] = useState(sess.schema);
  const [chosen, setChosen] = useState(false);

  useEffect(() => { ListDatabases().then(setDbs).catch(() => setDbs([])); }, []);
  useEffect(() => {
    if (!db) return;
    let live = true;
    setSchemas([]);
    // ListUserSchemas omits INFORMATION_SCHEMA, which cannot host user objects.
    ListUserSchemas(db)
      .then((xs) => {
        if (!live) return;
        setSchemas(xs);
        setSchema((cur) => (xs.includes(cur) ? cur : ""));
      })
      .catch(() => live && setSchemas([]));
    return () => { live = false; };
  }, [db]);

  if (chosen) return <>{children(db, schema)}</>;
  const opts = (xs: string[]) => xs.map((x) => ({ value: x, label: x }));
  return (
    <Modal open title={title} onCancel={onClose} width={420}
      footer={<>
        <Button onClick={onClose}>Cancel</Button>
        <Button type="primary" disabled={!db || !schema} onClick={() => setChosen(true)}>Next</Button>
      </>}>
      <Form layout="vertical" size="small">
        <Form.Item label="Database">
          <Select showSearch value={db || undefined} options={opts(dbs)}
            onChange={(v) => { setDb(v); setSchema(""); }} />
        </Form.Item>
        <Form.Item label="Schema">
          <Select showSearch value={schema || undefined} options={opts(schemas)} onChange={setSchema} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
