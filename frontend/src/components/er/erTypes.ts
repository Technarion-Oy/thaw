// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

export interface DesignerColumn {
  id: string;
  name: string;
  dataType: string;
  isPK: boolean;
  notNull: boolean;
  fkRef: string; // "SCHEMA.TABLE.COLUMN" or "" for none
}

export interface DesignerTable {
  id: string;
  schema: string;
  name: string;
  columns: DesignerColumn[];
}

export const SF_TYPES = [
  "NUMBER",
  "VARCHAR",
  "BOOLEAN",
  "DATE",
  "TIMESTAMP_NTZ",
  "TIMESTAMP_LTZ",
  "FLOAT",
  "VARIANT",
  "ARRAY",
  "OBJECT",
];

export const ER_NODE_WIDTH = 240;
export const ER_NODE_HEADER_HEIGHT = 32;
export const ER_NODE_ROW_HEIGHT = 24;
export const ER_NODE_PADDING = 8;
export const ER_COL_LIMIT = 30;
