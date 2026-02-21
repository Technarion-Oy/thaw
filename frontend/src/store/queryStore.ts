import { create } from "zustand";
import { ExecuteQuery } from "../../wailsjs/go/main/App";

export interface QueryResult {
  columns: string[];
  rows: unknown[][];
  rowsAffected: number;
}

interface QueryState {
  sql: string;
  selectedSql: string;
  currentFile: string | null;  // path of the file currently open in the editor
  result: QueryResult | null;
  isRunning: boolean;
  error: string | null;
  setSql: (sql: string) => void;
  setSelectedSql: (selected: string) => void;
  setResult: (result: QueryResult) => void;
  setRunning: (isRunning: boolean) => void;
  setError: (error: string | null) => void;
  // Loads file content into the editor and tracks the source path.
  openFile: (path: string, content: string) => void;
  // Sets the editor SQL, clears selection, and immediately runs the query.
  executeWith: (sql: string) => Promise<void>;
}

export const useQueryStore = create<QueryState>((set, get) => ({
  sql: "SELECT CURRENT_USER(), CURRENT_WAREHOUSE(), CURRENT_DATABASE();",
  selectedSql: "",
  currentFile: null,
  result: null,
  isRunning: false,
  error: null,
  setSql: (sql) => set({ sql, currentFile: null }),
  setSelectedSql: (selectedSql) => set({ selectedSql }),
  setResult: (result) => set({ result, error: null }),
  setRunning: (isRunning) => set({ isRunning }),
  setError: (error) => set({ error, isRunning: false }),
  openFile: (path, content) => set({ sql: content, currentFile: path, selectedSql: "", result: null, error: null }),
  executeWith: async (sql) => {
    set({ sql, selectedSql: "", currentFile: null, isRunning: true, error: null });
    try {
      const res = await ExecuteQuery(sql);
      get().setResult(res);
    } catch (e) {
      get().setError(String(e));
    } finally {
      set({ isRunning: false });
    }
  },
}));
