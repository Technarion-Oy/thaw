// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package filesystem

import (
	"os/exec"
	"syscall"
)

// revealInFileManager opens Windows Explorer with abs selected.
//
// explorer.exe has a non-standard command-line parser: it splits arguments on
// commas and treats quotes as plain grouping with no `\"` escape. Go's os/exec
// builds the child command line through syscall.EscapeArg, which backslash-
// escapes every quote — so any quotes we embed in an Args entry reach explorer
// as literal `\"`, corrupting the path. explorer then falls back to the deepest
// existing element (the file's containing folder) and `/select` on a folder
// opens its *parent* with the folder highlighted — the "always one level up"
// bug reported in issue #294.
//
// Setting SysProcAttr.CmdLine bypasses EscapeArg entirely: the string is passed
// to CreateProcess verbatim, so explorer's own parser sees the quotes literally
// and handles spaces, commas, parentheses, and Unicode inside the quoted path
// correctly. Windows paths cannot contain a literal quote, so abs never needs
// escaping and the closing quote is unambiguous.
//
// exec.Command("explorer") still resolves explorer.exe (via PATH) for the
// executable image; only the command line comes from CmdLine. explorer.exe
// always exits with code 1, but we only Start() (never Wait), so that is moot.
//
// This path only runs on Windows and cannot be exercised in CI (cross-compiled,
// never executed); manual-verification checklist lives in issue #840.
func revealInFileManager(abs string) error {
	cmd := exec.Command("explorer") // #nosec G204 — fixed argv0; the path is passed via CmdLine, not shell
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer /select,"` + abs + `"`}
	return cmd.Start()
}
