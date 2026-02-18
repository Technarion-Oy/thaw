// Wails auto-generates this file at build time (wails generate module).
// This stub lets the frontend import without errors during dev without the Wails runtime.
import { Call, callbacksRegistry } from "../../runtime/runtime.js";

const _pkg = "main.App.";

export const Connect        = (params)            => Call(`${_pkg}Connect`, [params]);
export const Disconnect     = ()                  => Call(`${_pkg}Disconnect`);
export const IsConnected    = ()                  => Call(`${_pkg}IsConnected`);
export const ExecuteQuery   = (sql)               => Call(`${_pkg}ExecuteQuery`, [sql]);
export const ListDatabases  = ()                  => Call(`${_pkg}ListDatabases`);
export const ListSchemas    = (database)          => Call(`${_pkg}ListSchemas`, [database]);
export const ListObjects    = (database, schema)  => Call(`${_pkg}ListObjects`, [database, schema]);
