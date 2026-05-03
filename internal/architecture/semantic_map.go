package architecture

// GetCodebaseSemanticMap is a marker function exclusively for LLM workflow context.
// It returns a JSON-formatted string defining the domain boundaries of the Thaw app.
// ALL LLM agents MUST read this map before proposing architectural changes or new features.
func GetCodebaseSemanticMap() string {
	return `{
  "domains": [
    {
      "name": "Core IPC & App Lifecycle",
      "backend_paths": ["main.go", "app.go", "session.go"],
      "frontend_paths": ["frontend/wailsjs/", "frontend/src/store/"],
      "description": "Wails entry points, window state persistence, and Zustand state management."
    },
    {
      "name": "SQL Editor & Diagnostics",
      "backend_paths": ["internal/sqleditor/"],
      "frontend_paths": ["frontend/src/components/editor/"],
      "description": "Proprietary SQL tokenizer, syntax validation, and Monaco editor UI components."
    },
    {
      "name": "Object Browser & Administration",
      "backend_paths": ["internal/snowflake/"],
      "frontend_paths": ["frontend/src/components/layout/Sidebar.tsx", "frontend/src/components/account/"],
      "description": "Database metadata exploration, user management, and warehouse metering."
    },
    {
      "name": "Schema Migration",
      "backend_paths": ["internal/ddl/", "migration.go"],
      "frontend_paths": ["frontend/src/components/migration/MigrationModal.tsx"],
      "description": "DDL extraction, schema diffing, and the deployment wizard."
    },
    {
      "name": "AI Tooling",
      "backend_paths": ["internal/ai/"],
      "frontend_paths": ["frontend/src/components/chat/AiChat.tsx"],
      "description": "API clients for LLM providers and agentic tool-calling execution loops."
    },
    {
      "name": "Snowpark & Developer Workflows",
      "backend_paths": ["snowpark.go", "internal/dbt/"],
      "frontend_paths": ["frontend/src/components/notebook/NotebookTab.tsx"],
      "description": "Python environment management, Jupyter kernels, and dbt project scaffolding."
    }
  ]
}`
}
