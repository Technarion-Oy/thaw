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
	"bufio"
	"encoding/base64"
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetAvailableShells reads /etc/shells and returns the list of valid shells.
// Lines starting with '#' are skipped, as are paths that do not exist on disk.
// Falls back to ["/bin/zsh", "/bin/bash", "/bin/sh"] when the file cannot be read.
func (a *App) GetAvailableShells() []string {
	f, err := os.Open("/etc/shells")
	if err != nil {
		return []string{"/bin/zsh", "/bin/bash", "/bin/sh"}
	}
	defer f.Close() //nolint:errcheck

	var shells []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, err := os.Stat(line); err == nil {
			shells = append(shells, line)
		}
	}
	if len(shells) == 0 {
		return []string{"/bin/zsh", "/bin/bash", "/bin/sh"}
	}
	return shells
}

// StartShell launches the given shell in a pseudo-terminal.
// If a shell is already running it is stopped first.
// dir sets the working directory; when empty the shell inherits the process cwd.
// Output from the shell is emitted as base64-encoded "terminal:data" events;
// process exit is signaled by a "terminal:exit" event.
func (a *App) StartShell(shell, dir string) error {
	a.ptyMu.Lock()
	defer a.ptyMu.Unlock()

	// Stop any previously running shell (already locked, so call internals directly).
	if a.ptmx != nil {
		a.ptmx.Close() //nolint:errcheck
		if a.ptyCmd != nil && a.ptyCmd.Process != nil {
			a.ptyCmd.Process.Kill() //nolint:errcheck
		}
		a.ptmx = nil
		a.ptyCmd = nil
	}

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if dir != "" {
		cmd.Dir = dir
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	a.ptmx = ptmx
	a.ptyCmd = cmd

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				encoded := base64.StdEncoding.EncodeToString(buf[:n])
				wailsruntime.EventsEmit(a.ctx, "terminal:data", encoded)
			}
			if err != nil {
				// EOF or closed — shell exited.
				a.ptyMu.Lock()
				a.ptmx = nil
				a.ptyCmd = nil
				a.ptyMu.Unlock()
				wailsruntime.EventsEmit(a.ctx, "terminal:exit")
				return
			}
		}
	}()

	return nil
}

// WriteShell sends data (keystrokes) to the running shell's stdin.
func (a *App) WriteShell(data string) error {
	a.ptyMu.Lock()
	defer a.ptyMu.Unlock()
	if a.ptmx == nil {
		return nil
	}
	_, err := a.ptmx.Write([]byte(data))
	return err
}

// ResizeShell updates the terminal window size of the running pseudo-terminal.
func (a *App) ResizeShell(cols, rows int) error {
	a.ptyMu.Lock()
	defer a.ptyMu.Unlock()
	if a.ptmx == nil {
		return nil
	}
	return pty.Setsize(a.ptmx, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// StopShell kills the running shell and closes the pseudo-terminal.
// It is a no-op when no shell is running.
func (a *App) StopShell() error {
	a.ptyMu.Lock()
	defer a.ptyMu.Unlock()
	if a.ptmx == nil {
		return nil
	}
	a.ptmx.Close() //nolint:errcheck
	if a.ptyCmd != nil && a.ptyCmd.Process != nil {
		a.ptyCmd.Process.Kill() //nolint:errcheck
	}
	a.ptmx = nil
	a.ptyCmd = nil
	return nil
}
