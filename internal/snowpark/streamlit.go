// SPDX-License-Identifier: GPL-3.0-or-later

package snowpark

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Events emitted by the local Streamlit preview, consumed by the deploy modal:
//   - streamlitPreviewOutput: one stdout/stderr line at a time (live log).
//   - streamlitPreviewReady:  the local URL, once the server accepts connections.
//   - streamlitPreviewStopped: emitted when the process exits on its own.
//   - streamlitPreviewError:  the preview started but never became reachable, so
//     no ready event is coming — without this the UI would sit on "Starting…"
//     forever with the reason buried in the output log.
const (
	streamlitPreviewOutput  = "snowpark:streamlit-output"
	streamlitPreviewReady   = "snowpark:streamlit-ready"
	streamlitPreviewStopped = "snowpark:streamlit-stopped"
	streamlitPreviewError   = "snowpark:streamlit-error"
)

// readinessAttempts × readinessInterval bound how long the port is polled before
// the preview is declared unreachable (~20s, enough for a cold Streamlit start).
const (
	readinessAttempts = 40
	readinessInterval = 500 * time.Millisecond
)

// At most one local preview runs at a time; streamlitCmd is the current process.
// streamlitMu guards it AND serializes the whole stop→start→record sequence in
// StartStreamlitPreview: taking it only for the final assignment let two
// concurrent starts (a double-click ahead of the first IPC round-trip) both
// launch a server, with the loser left running and unreferenced — unkillable
// until the app quit.
var (
	streamlitMu  sync.Mutex
	streamlitCmd *exec.Cmd
)

// streamlitCommand builds the command that runs the app. It is a var so tests can
// substitute a stub process instead of resolving a real Python environment.
var streamlitCommand = defaultStreamlitCommand

// defaultStreamlitCommand resolves the active Snowpark environment's Python (the
// same one the notebook kernel uses), verifies the streamlit package is installed
// there, and returns the not-yet-started command.
func defaultStreamlitCommand(appDir, mainFile string, port int) (*exec.Cmd, error) {
	python, err := snowparkPython()
	if err != nil {
		return nil, err
	}
	if err := exec.Command(python, "-c", moduleAvailableScript("streamlit")).Run(); err != nil {
		return nil, fmt.Errorf(
			"the 'streamlit' package isn't installed in the Snowpark environment — add it under Tools → Snowpark → Packages, then try again")
	}
	cmd := exec.Command(python, "-m", "streamlit", "run", filepath.FromSlash(mainFile),
		"--server.headless=true",
		"--server.port", strconv.Itoa(port),
		"--browser.gatherUsageStats=false")
	cmd.Dir = appDir
	return cmd, nil
}

// emitPreview emits a preview event. It tolerates a Service with no Wails context
// (unit tests construct one directly), where there is nothing to emit to.
func (s *Service) emitPreview(event string, data any) {
	if s.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(s.ctx, event, data)
}

// StreamlitPreviewResult reports where a started preview is served.
type StreamlitPreviewResult struct {
	URL  string `json:"url"`  // e.g. http://localhost:8501
	Port int    `json:"port"`
}

// StartStreamlitPreview launches `python -m streamlit run <mainFile>` from appDir
// using the active Snowpark environment's Python (the same conda/venv the notebook
// kernel uses), for a quick pre-deploy look. Output lines stream as
// streamlitPreviewOutput events; once the server accepts connections a
// streamlitPreviewReady event carries the local URL. Only one preview runs at a
// time — starting a new one replaces any existing preview.
//
// Caveat surfaced in the UI: Snowflake's Streamlit runtime pins specific
// Python/Streamlit versions and an allow-listed Anaconda package set, so a local
// preview is a convenience, not a guarantee that the app behaves identically in
// Snowflake.
func (s *Service) StartStreamlitPreview(appDir, mainFile string) (StreamlitPreviewResult, error) {
	if strings.TrimSpace(appDir) == "" {
		return StreamlitPreviewResult{}, fmt.Errorf("app folder is required")
	}
	main := strings.TrimSpace(mainFile)
	if main == "" {
		main = "streamlit_app.py"
	}
	if _, err := os.Stat(filepath.Join(appDir, filepath.FromSlash(main))); err != nil {
		return StreamlitPreviewResult{}, fmt.Errorf("main file %q not found in the app folder", main)
	}

	// Everything from "replace the running preview" to "record the new one" runs
	// under the lock, so two concurrent starts can never both end up with a live
	// process: the second waits, then stops whatever the first recorded.
	streamlitMu.Lock()
	defer streamlitMu.Unlock()

	stopPreviewLocked()

	port, err := freeTCPPort()
	if err != nil {
		return StreamlitPreviewResult{}, fmt.Errorf("find a free port: %w", err)
	}

	cmd, err := streamlitCommand(appDir, main, port)
	if err != nil {
		return StreamlitPreviewResult{}, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return StreamlitPreviewResult{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return StreamlitPreviewResult{}, err
	}
	if err := cmd.Start(); err != nil {
		return StreamlitPreviewResult{}, fmt.Errorf("start streamlit: %w", err)
	}
	streamlitCmd = cmd

	// isCurrent reports whether cmd is still the preview the app knows about. The
	// goroutines below block on the mutex until this function returns.
	isCurrent := func() bool {
		streamlitMu.Lock()
		defer streamlitMu.Unlock()
		return streamlitCmd == cmd
	}

	// Trailing output from a superseded process is dropped rather than interleaved
	// into the live log of the one that replaced it.
	emit := func(line string) {
		if isCurrent() {
			s.emitPreview(streamlitPreviewOutput, line)
		}
	}
	go pumpLines(stdout, emit)
	go pumpLines(stderr, emit)

	url := fmt.Sprintf("http://localhost:%d", port)

	// Poll the port until the server is up, then announce the URL. Bail if this
	// preview is superseded/stopped in the meantime; if it never answers, say so
	// instead of leaving the UI waiting for a ready event that isn't coming.
	go func() {
		for range readinessAttempts {
			if !isCurrent() {
				return
			}
			if conn, derr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond); derr == nil {
				_ = conn.Close()
				s.emitPreview(streamlitPreviewReady, url)
				return
			}
			time.Sleep(readinessInterval)
		}
		if isCurrent() {
			s.emitPreview(streamlitPreviewError, fmt.Sprintf(
				"the preview didn't start serving on port %d within %s — see the output above for the reason",
				port, readinessAttempts*readinessInterval))
		}
	}()

	// Reap the process. Only announce "stopped" if this cmd was still the current
	// preview (i.e. it exited on its own rather than being superseded/stopped).
	go func() {
		_ = cmd.Wait()
		streamlitMu.Lock()
		wasCurrent := streamlitCmd == cmd
		if wasCurrent {
			streamlitCmd = nil
		}
		streamlitMu.Unlock()
		if wasCurrent {
			s.emitPreview(streamlitPreviewStopped, "")
		}
	}()

	return StreamlitPreviewResult{URL: url, Port: port}, nil
}

// StopStreamlitPreview terminates the running local preview, if any.
func (s *Service) StopStreamlitPreview() {
	streamlitMu.Lock()
	defer streamlitMu.Unlock()
	stopPreviewLocked()
}

// stopPreviewLocked kills the recorded preview and clears it. The caller must
// hold streamlitMu — StartStreamlitPreview keeps it across the whole replace
// sequence, so the kill cannot be interleaved with another start. Killing under
// the lock is safe: Process.Kill does not wait, and the reaper goroutine takes
// the mutex only after cmd.Wait returns.
func stopPreviewLocked() {
	cmd := streamlitCmd
	streamlitCmd = nil
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// pumpLines scans r line by line and hands each to emit, tolerating long lines.
func pumpLines(r io.Reader, emit func(string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		emit(sc.Text())
	}
}

// freeTCPPort asks the OS for an unused localhost TCP port. The listener is
// closed before Streamlit binds it, so another local process could in principle
// take the port in between; that window isn't worth engineering around for a
// local dev preview, and the readiness poll now reports the failure
// (streamlitPreviewError) instead of hanging if it ever happens.
func freeTCPPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close() //nolint:errcheck
	return l.Addr().(*net.TCPAddr).Port, nil
}
