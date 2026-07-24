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
const (
	streamlitPreviewOutput  = "snowpark:streamlit-output"
	streamlitPreviewReady   = "snowpark:streamlit-ready"
	streamlitPreviewStopped = "snowpark:streamlit-stopped"
)

// At most one local preview runs at a time; streamlitCmd is the current process.
var (
	streamlitMu  sync.Mutex
	streamlitCmd *exec.Cmd
)

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
	// Replace any running preview first.
	s.StopStreamlitPreview()

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

	python, err := snowparkPython()
	if err != nil {
		return StreamlitPreviewResult{}, err
	}
	if err := exec.Command(python, "-c", moduleAvailableScript("streamlit")).Run(); err != nil {
		return StreamlitPreviewResult{}, fmt.Errorf(
			"the 'streamlit' package isn't installed in the Snowpark environment — add it under Tools → Snowpark → Packages, then try again")
	}

	port, err := freeTCPPort()
	if err != nil {
		return StreamlitPreviewResult{}, fmt.Errorf("find a free port: %w", err)
	}

	cmd := exec.Command(python, "-m", "streamlit", "run", filepath.FromSlash(main),
		"--server.headless=true",
		"--server.port", strconv.Itoa(port),
		"--browser.gatherUsageStats=false")
	cmd.Dir = appDir

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

	streamlitMu.Lock()
	streamlitCmd = cmd
	streamlitMu.Unlock()

	emit := func(line string) { wailsruntime.EventsEmit(s.ctx, streamlitPreviewOutput, line) }
	go pumpLines(stdout, emit)
	go pumpLines(stderr, emit)

	url := fmt.Sprintf("http://localhost:%d", port)

	// Poll the port until the server is up, then announce the URL. Bail if this
	// preview is superseded/stopped in the meantime.
	go func() {
		for range 40 {
			streamlitMu.Lock()
			current := streamlitCmd == cmd
			streamlitMu.Unlock()
			if !current {
				return
			}
			if conn, derr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond); derr == nil {
				_ = conn.Close()
				wailsruntime.EventsEmit(s.ctx, streamlitPreviewReady, url)
				return
			}
			time.Sleep(500 * time.Millisecond)
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
			wailsruntime.EventsEmit(s.ctx, streamlitPreviewStopped, "")
		}
	}()

	return StreamlitPreviewResult{URL: url, Port: port}, nil
}

// StopStreamlitPreview terminates the running local preview, if any.
func (s *Service) StopStreamlitPreview() {
	streamlitMu.Lock()
	cmd := streamlitCmd
	streamlitCmd = nil
	streamlitMu.Unlock()
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

// freeTCPPort asks the OS for an unused localhost TCP port.
func freeTCPPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close() //nolint:errcheck
	return l.Addr().(*net.TCPAddr).Port, nil
}
