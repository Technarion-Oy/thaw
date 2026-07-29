// SPDX-License-Identifier: GPL-3.0-or-later

package snowpark

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

// stubPreviewCommand replaces the real `python -m streamlit run` launch with a
// long-sleeping process, so the preview lifecycle can be exercised without a
// Snowpark environment. It records every command it hands out.
func stubPreviewCommand(t *testing.T) *[]*exec.Cmd {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub uses a POSIX sleep binary")
	}
	var mu sync.Mutex
	var started []*exec.Cmd

	orig := streamlitCommand
	streamlitCommand = func(appDir, mainFile string, port int) (*exec.Cmd, error) {
		cmd := exec.Command("sleep", "60")
		cmd.Dir = appDir
		mu.Lock()
		started = append(started, cmd)
		mu.Unlock()
		return cmd, nil
	}
	t.Cleanup(func() {
		streamlitCommand = orig
		(&Service{}).StopStreamlitPreview()
	})
	return &started
}

// appDirWithMain creates a minimal app folder that passes the entrypoint check.
func appDirWithMain(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "streamlit_app.py"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write main file: %v", err)
	}
	return dir
}

// TestStartStreamlitPreview_ConcurrentStartsLeaveOneProcess is the regression
// test for the start race: the stop→start→record sequence used to run outside
// the mutex, so two starts racing (a double-click before the first IPC round-trip
// returns) could both leave a live server behind, the loser unreferenced and
// therefore unkillable. Run with -race, which also covers the shared state.
func TestStartStreamlitPreview_ConcurrentStartsLeaveOneProcess(t *testing.T) {
	started := stubPreviewCommand(t)
	dir := appDirWithMain(t)
	s := &Service{} // no Wails ctx: events are no-ops (see emitPreview)

	const n = 4
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.StartStreamlitPreview(dir, "streamlit_app.py"); err != nil {
				t.Errorf("StartStreamlitPreview: %v", err)
			}
		}()
	}
	wg.Wait()

	if len(*started) != n {
		t.Fatalf("started %d commands, want %d", len(*started), n)
	}

	// Exactly one process is recorded as the current preview...
	streamlitMu.Lock()
	current := streamlitCmd
	streamlitMu.Unlock()
	if current == nil {
		t.Fatal("no preview recorded after concurrent starts")
	}

	// ...and every other one was killed, i.e. nothing is left running that the
	// app can no longer reach.
	for i, cmd := range *started {
		if cmd == current {
			continue
		}
		if !waitExited(cmd) {
			t.Errorf("superseded preview %d is still running (orphan)", i)
		}
	}

	// Stopping clears the last one too.
	s.StopStreamlitPreview()
	if !waitExited(current) {
		t.Error("StopStreamlitPreview left the current preview running")
	}
	streamlitMu.Lock()
	left := streamlitCmd
	streamlitMu.Unlock()
	if left != nil {
		t.Error("StopStreamlitPreview did not clear the recorded preview")
	}
}

// TestStartStreamlitPreview_ReplacesPrevious covers the ordinary sequential case:
// starting a second preview kills the first and records the second.
func TestStartStreamlitPreview_ReplacesPrevious(t *testing.T) {
	started := stubPreviewCommand(t)
	dir := appDirWithMain(t)
	s := &Service{}

	if _, err := s.StartStreamlitPreview(dir, "streamlit_app.py"); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if _, err := s.StartStreamlitPreview(dir, "streamlit_app.py"); err != nil {
		t.Fatalf("second start: %v", err)
	}
	if len(*started) != 2 {
		t.Fatalf("started %d commands, want 2", len(*started))
	}
	if !waitExited((*started)[0]) {
		t.Error("the replaced preview is still running")
	}
	streamlitMu.Lock()
	current := streamlitCmd
	streamlitMu.Unlock()
	if current != (*started)[1] {
		t.Error("the second preview was not recorded as current")
	}
}

// TestStartStreamlitPreview_MissingMainFile rejects before touching the process
// machinery, so a bad entrypoint never disturbs a running preview.
func TestStartStreamlitPreview_MissingMainFile(t *testing.T) {
	started := stubPreviewCommand(t)
	s := &Service{}

	if _, err := s.StartStreamlitPreview(t.TempDir(), "nope.py"); err == nil {
		t.Fatal("expected an error for a missing main file")
	}
	if _, err := s.StartStreamlitPreview("", "streamlit_app.py"); err == nil {
		t.Fatal("expected an error for an empty app folder")
	}
	if len(*started) != 0 {
		t.Errorf("started %d commands, want 0", len(*started))
	}
}

// waitExited reports whether cmd's process is gone within a short grace period.
// It probes with signal 0 rather than reading cmd.ProcessState, which the reaper
// goroutine writes concurrently — os.Process guards its own state, so this stays
// clean under -race.
func waitExited(cmd *exec.Cmd) bool {
	if cmd.Process == nil {
		return true
	}
	for range 100 {
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			return true // os.ErrProcessDone once the reaper has waited on it
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
