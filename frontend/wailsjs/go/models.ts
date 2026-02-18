// Wails auto-generates this file from Go structs (wails generate module).
// Keep in sync with internal/snowflake/client.go

export namespace snowflake {
  export interface ConnectParams {
    account: string;
    user: string;
    password: string;
    role: string;
    warehouse: string;
    database: string;
    schema: string;
  }

  export interface SnowflakeObject {
    name: string;
    kind: string;
    schema: string;
  }

  export interface QueryResult {
    columns: string[];
    rows: unknown[][];
    rowsAffected: number;
  }
}
