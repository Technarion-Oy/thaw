// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"thaw/internal/config"
	"thaw/internal/snowpark"
)

func (a *App) IsAppleSilicon() bool { return snowpark.IsAppleSilicon() }

func (a *App) GetSnowparkConfig() snowpark.SnowparkConfigResult {
	return a.snowparkSvc.GetSnowparkConfig()
}

func (a *App) SaveSnowparkConfig(backend string) error {
	return a.snowparkSvc.SaveSnowparkConfig(backend)
}

func (a *App) SaveSnowparkVenvPath(path string) error {
	return a.snowparkSvc.SaveSnowparkVenvPath(path)
}

func (a *App) VenvFolderExists() bool { return a.snowparkSvc.VenvFolderExists() }

func (a *App) SaveSnowparkPythonPath(pythonPath string) error {
	return a.snowparkSvc.SaveSnowparkPythonPath(pythonPath)
}

func (a *App) GetPipRegistryConfig() (config.PipRegistryConfig, error) {
	return a.snowparkSvc.GetPipRegistryConfig()
}

func (a *App) SavePipRegistryConfig(cfg config.PipRegistryConfig) error {
	return a.snowparkSvc.SavePipRegistryConfig(cfg)
}

func (a *App) ResetPipRegistryConfig() error { return a.snowparkSvc.ResetPipRegistryConfig() }

func (a *App) PickCACertFile() (string, error) { return a.snowparkSvc.PickCACertFile() }

func (a *App) ListSystemPythons() []snowpark.PythonInfo { return a.snowparkSvc.ListSystemPythons() }

func (a *App) CheckSnowparkEnv() snowpark.SnowparkCheckResult {
	return a.snowparkSvc.CheckSnowparkEnv()
}

func (a *App) ListEnvPackages() ([]snowpark.PackageInfo, error) {
	return a.snowparkSvc.ListEnvPackages()
}

func (a *App) InstallEnvPackage(pkg string) error { return a.snowparkSvc.InstallEnvPackage(pkg) }

func (a *App) UninstallEnvPackage(pkg string) error { return a.snowparkSvc.UninstallEnvPackage(pkg) }

func (a *App) InstallCondaEnv() error { return a.snowparkSvc.InstallCondaEnv() }

func (a *App) InstallSnowparkPackage() error { return a.snowparkSvc.InstallSnowparkPackage() }

func (a *App) InstallJupyterNotebook() error { return a.snowparkSvc.InstallJupyterNotebook() }

func (a *App) InstallVenvEnv() error { return a.snowparkSvc.InstallVenvEnv() }

func (a *App) InstallSnowparkVenv(withPandas bool) error {
	return a.snowparkSvc.InstallSnowparkVenv(withPandas)
}

func (a *App) DeleteVenvFolder() error { return a.snowparkSvc.DeleteVenvFolder() }

func (a *App) InstallJupyterVenv() error { return a.snowparkSvc.InstallJupyterVenv() }

func (a *App) NewNotebook() (string, error) { return a.snowparkSvc.NewNotebook() }

func (a *App) PickNotebookFile() (string, error) { return a.snowparkSvc.PickNotebookFile() }

func (a *App) ReadNotebook(path string) (string, error) { return a.snowparkSvc.ReadNotebook(path) }

func (a *App) SaveNotebook(path string, content string) error {
	return a.snowparkSvc.SaveNotebook(path, content)
}

func (a *App) SaveNotebookBreakpoints(notebookPath string, bps map[string][]int) error {
	return a.snowparkSvc.SaveNotebookBreakpoints(notebookPath, bps)
}

func (a *App) LoadNotebookBreakpoints(notebookPath string) (map[string][]int, error) {
	return a.snowparkSvc.LoadNotebookBreakpoints(notebookPath)
}

func (a *App) StopDapProxy() { a.snowparkSvc.StopDapProxy() }

func (a *App) StopNotebookSession(tabId string) error {
	return a.snowparkSvc.StopNotebookSession(tabId)
}

func (a *App) RunNotebookSql(sql string) (snowpark.NotebookSqlResult, error) {
	return a.snowparkSvc.RunNotebookSql(a.client, sql)
}

func (a *App) StartNotebookSession(tabId string) error {
	return a.snowparkSvc.StartNotebookSession(a.client, a.connectParams, tabId)
}

func (a *App) GetKernelPythonVersion(tabId string) string {
	return a.snowparkSvc.GetKernelPythonVersion(tabId)
}

func (a *App) RunNotebookCell(tabId string, cellId string, code string) (snowpark.NotebookCellOutput, error) {
	return a.snowparkSvc.RunNotebookCell(tabId, cellId, code)
}

func (a *App) StartDapProxy() error {
	return a.snowparkSvc.StartDapProxy()
}

func (a *App) DebugNotebookCell(tabId string, cellId string, code string) (snowpark.NotebookCellOutput, error) {
	return a.snowparkSvc.DebugNotebookCell(tabId, cellId, code)
}

func (a *App) RunNotebookCellSql(tabId, sql string) (snowpark.NotebookSqlResult, error) {
	return a.snowparkSvc.RunNotebookCellSql(a.client, tabId, sql)
}

func (a *App) NotebookUseContext(tabId, role, warehouse, database, schema string) error {
	return a.snowparkSvc.NotebookUseContext(tabId, role, warehouse, database, schema)
}

func (a *App) GetNotebookCompletions(tabId, code string, line, col int) ([]snowpark.NotebookCompletion, error) {
	return a.snowparkSvc.GetNotebookCompletions(tabId, code, line, col)
}

func (a *App) GetNotebookHover(tabId, code string, line, col int) (string, error) {
	return a.snowparkSvc.GetNotebookHover(tabId, code, line, col)
}

func (a *App) CheckPythonSyntax(tabId, code, mode string) ([]snowpark.NotebookSyntaxError, error) {
	return a.snowparkSvc.CheckPythonSyntax(tabId, code, mode)
}
