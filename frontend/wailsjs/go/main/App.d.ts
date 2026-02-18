// Wails auto-generates this file at build time (wails generate module).
// This stub keeps TypeScript happy during development.
// Do not edit manually — run `wails generate module` to regenerate.

import { snowflake } from "../models";

export function Connect(params: snowflake.ConnectParams): Promise<void>;
export function Disconnect(): Promise<void>;
export function IsConnected(): Promise<boolean>;
export function ExecuteQuery(sql: string): Promise<snowflake.QueryResult>;
export function ListDatabases(): Promise<string[]>;
export function ListSchemas(database: string): Promise<string[]>;
export function ListObjects(database: string, schema: string): Promise<snowflake.SnowflakeObject[]>;
