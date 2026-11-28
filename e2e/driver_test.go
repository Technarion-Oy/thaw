// SPDX-License-Identifier: GPL-3.0-or-later

//go:build wails_e2e

package e2e_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// wailsAppControlURLRe matches the streamable HTTP endpoint logged by Wails v3's
// built-in app-control driver when compiled with `-tags mcp` and
// `WAILS_MCP_PORT=0`.
var wailsAppControlURLRe = regexp.MustCompile(`http://127\.0\.0\.1:\d+/mcp`)

// syncBuffer is a goroutine-safe bounded buffer used to capture child stdout
// and stderr so we can scan for the driver endpoint while the app is starting
// and dump logs on test failure without unbounded memory growth.
type syncBuffer struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	overflow bool
}

const maxCapturedLogBytes = 256 * 1024

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buf.Len() < maxCapturedLogBytes {
		rem := maxCapturedLogBytes - b.buf.Len()
		if len(p) <= rem {
			b.buf.Write(p)
		} else {
			b.buf.Write(p[:rem])
			b.overflow = true
		}
	} else {
		b.overflow = true
	}
	return len(p), nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.overflow {
		return b.buf.String() + "\n... [output truncated at 256 KiB]"
	}
	return b.buf.String()
}

// buildOrLocateE2EBinary returns the path to a Thaw binary compiled with
// `-tags mcp` (non-production, non-dev). If THAW_E2E_BINARY is set (e.g. in CI),
// that prebuilt binary is used directly; otherwise a fresh binary is built into
// a temporary directory using the already-built frontend/dist bundle.
func buildOrLocateE2EBinary(t *testing.T) string {
	t.Helper()
	if prebuilt := strings.TrimSpace(os.Getenv("THAW_E2E_BINARY")); prebuilt != "" {
		if _, err := os.Stat(prebuilt); err != nil {
			t.Fatalf("THAW_E2E_BINARY=%q is not accessible: %v", prebuilt, err)
		}
		return prebuilt
	}

	root := repoRoot(t)
	indexHTML := filepath.Join(root, "frontend", "dist", "index.html")
	if _, err := os.Stat(indexHTML); err != nil {
		t.Fatalf("frontend/dist/index.html not found (%v) — run `cd frontend && npm run build` first (or set THAW_E2E_BINARY)", err)
	}

	binName := "thaw-wails-e2e"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	outPath := filepath.Join(t.TempDir(), binName)
	cmd := exec.Command("go", "build", "-tags", "mcp", "-buildvcs=false", "-o", outPath, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building Wails E2E binary (`go build -tags mcp`): %v\n%s", err, string(out))
	}
	return outPath
}

// isolatedEnv constructs an environment for the child Thaw process that redirects
// all OS config, data, state, and cache paths under stateDir so E2E tests never
// touch a developer's real Thaw config or session files.
func isolatedEnv(stateDir, authToken string) []string {
	overrides := map[string]string{
		"HOME":                  stateDir,
		"USERPROFILE":           stateDir,
		"XDG_CONFIG_HOME":       filepath.Join(stateDir, "xdg-config"),
		"XDG_DATA_HOME":         filepath.Join(stateDir, "xdg-data"),
		"XDG_STATE_HOME":        filepath.Join(stateDir, "xdg-state"),
		"XDG_CACHE_HOME":        filepath.Join(stateDir, "xdg-cache"),
		"APPDATA":               filepath.Join(stateDir, "appdata"),
		"LOCALAPPDATA":          filepath.Join(stateDir, "localappdata"),
		"WAILS_MCP_HOST":        "127.0.0.1",
		"WAILS_MCP_PORT":        "0",
		"WAILS_MCP_TOKEN":       authToken,
		"WAILS_MCP_HIDE_CURSOR": "1",
		"WAILS_MCP_TIMEOUT":     "15000",
	}

	base := os.Environ()
	env := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		key, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if _, replaced := overrides[key]; replaced {
			continue
		}
		env = append(env, kv)
	}
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env
}

// isolatedSessionPath returns the session.json path that internal/session
// (`//go:build !dev`) uses when launched with isolatedEnv(stateDir, ...).
func isolatedSessionPath(stateDir string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(stateDir, "Library", "Application Support", "thaw", "session.json")
	case "windows":
		return filepath.Join(stateDir, "localappdata", "thaw", "session.json")
	default:
		return filepath.Join(stateDir, "xdg-data", "thaw", "session.json")
	}
}

// isolatedConfigPath returns the config.json path that internal/config uses
// when launched with isolatedEnv(stateDir, ...).
func isolatedConfigPath(stateDir string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(stateDir, "Library", "Application Support", "thaw", "config.json")
	case "windows":
		return filepath.Join(stateDir, "appdata", "thaw", "config.json")
	default:
		return filepath.Join(stateDir, "xdg-config", "thaw", "config.json")
	}
}

func randomHexToken(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return hex.EncodeToString(b)
}

// wailsE2EInstance represents a single running Thaw process driven via Wails
// v3's app-control HTTP JSON-RPC endpoint.
type wailsE2EInstance struct {
	t          *testing.T
	cmd        *exec.Cmd
	logs       *syncBuffer
	endpoint   string
	statusURL  string
	authToken  string
	httpClient *http.Client
	reqID      atomic.Int64

	waitOnce sync.Once
	waitCh   chan error
}

// launchThaw starts binaryPath using stateDir as its isolated profile root,
// discovers its ephemeral Wails app-control endpoint, completes the protocol
// handshake, and waits until the main window and React root have mounted.
func launchThaw(t *testing.T, binaryPath, stateDir string) *wailsE2EInstance {
	t.Helper()
	for _, sub := range []string{"xdg-config", "xdg-data", "xdg-state", "xdg-cache", "appdata", "localappdata"} {
		if err := os.MkdirAll(filepath.Join(stateDir, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}

	token := randomHexToken(t)
	logs := &syncBuffer{}
	cmd := exec.Command(binaryPath)
	cmd.Dir = stateDir
	cmd.Env = isolatedEnv(stateDir, token)
	cmd.Stdout = logs
	cmd.Stderr = logs

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start %s: %v", binaryPath, err)
	}

	inst := &wailsE2EInstance{
		t:          t,
		cmd:        cmd,
		logs:       logs,
		authToken:  token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		waitCh:     make(chan error, 1),
	}
	go func() {
		inst.waitCh <- cmd.Wait()
		close(inst.waitCh)
	}()

	t.Cleanup(func() {
		inst.forceStop()
		if t.Failed() {
			t.Logf("--- Thaw E2E process output ---\n%s\n--- end output ---", inst.logs.String())
		}
	})

	// 1. Discover the ephemeral Wails app-control URL from stderr/stdout.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-inst.waitCh:
			t.Fatalf("Thaw exited before Wails app-control server started (err=%v):\n%s", err, inst.logs.String())
		default:
		}
		if match := wailsAppControlURLRe.FindString(inst.logs.String()); match != "" {
			inst.endpoint = match
			inst.statusURL = strings.TrimSuffix(match, "/mcp") + "/"
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if inst.endpoint == "" {
		t.Fatalf("timed out waiting for Wails app-control server URL in process output:\n%s", inst.logs.String())
	}

	// 2. Verify status endpoint & complete JSON-RPC initialize handshake.
	inst.waitForStatus(15 * time.Second)
	if err := inst.initialize(); err != nil {
		t.Fatalf("Wails app-control initialize failed: %v", err)
	}

	// 3. Wait for main window and mounted React root (#root has children).
	inst.waitForAppReady(30 * time.Second)
	return inst
}

func (w *wailsE2EInstance) waitForStatus(timeout time.Duration) {
	w.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := w.httpClient.Get(w.statusURL)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK && strings.Contains(string(body), "wails-mcp") {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	w.t.Fatalf("timed out waiting for Wails app-control status endpoint %s", w.statusURL)
}

func (w *wailsE2EInstance) waitForAppReady(timeout time.Duration) {
	w.t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		windows, err := w.windowsList()
		if err == nil && len(windows) > 0 && windows[0].Width > 0 && windows[0].Height > 0 {
			raw, evalErr := w.callToolRaw("js_eval", map[string]any{
				"js":         `return Boolean(document.querySelector('#root') && document.querySelector('#root').childElementCount > 0);`,
				"timeout_ms": 2000,
			})
			if evalErr == nil && strings.TrimSpace(raw) == "true" {
				return
			}
			lastErr = evalErr
		} else {
			lastErr = err
		}
		time.Sleep(150 * time.Millisecond)
	}
	w.t.Fatalf("timed out waiting for React root to mount (lastErr=%v)", lastErr)
}

// closeAndWait closes the main window through Wails's window_control tool and
// waits for the process to exit cleanly (exit status 0).
func (w *wailsE2EInstance) closeAndWait(timeout time.Duration) error {
	// Closing the last window terminates the app, so the HTTP response itself
	// may race with shutdown; ignore connection-closed errors on close.
	_, _ = w.callToolRaw("window_control", map[string]any{"action": "close"})

	select {
	case err := <-w.waitCh:
		return err
	case <-time.After(timeout):
		w.forceStop()
		return fmt.Errorf("timed out after %s waiting for Thaw to exit cleanly", timeout)
	}
}

func (w *wailsE2EInstance) forceStop() {
	w.waitOnce.Do(func() {
		select {
		case <-w.waitCh:
			return
		default:
		}
		if w.cmd.Process != nil {
			_ = w.cmd.Process.Kill()
		}
		select {
		case <-w.waitCh:
		case <-time.After(5 * time.Second):
		}
	})
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func (w *wailsE2EInstance) rpc(ctx context.Context, method string, params any) (json.RawMessage, error) {
	payload, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      w.reqID.Add(1),
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+w.authToken)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, w.endpoint, strings.TrimSpace(string(body)))
	}

	var decoded rpcResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decoding JSON-RPC response: %w (body=%s)", err, string(body))
	}
	if decoded.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error %d: %s", decoded.Error.Code, decoded.Error.Message)
	}
	return decoded.Result, nil
}

func (w *wailsE2EInstance) initialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := w.rpc(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "thaw-wails-e2e",
			"version": "1.0.0",
		},
	})
	return err
}

func (w *wailsE2EInstance) listTools() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := w.rpc(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var out struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Tools))
	for _, t := range out.Tools {
		names = append(names, t.Name)
	}
	return names, nil
}

func (w *wailsE2EInstance) callToolRaw(name string, args map[string]any) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if args == nil {
		args = map[string]any{}
	}
	raw, err := w.rpc(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}
	var res struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", fmt.Errorf("decoding tool result for %s: %w", name, err)
	}
	var text string
	if len(res.Content) > 0 {
		text = res.Content[0].Text
	}
	if res.IsError {
		return text, errors.New(text)
	}
	return text, nil
}

func (w *wailsE2EInstance) callToolJSON(name string, args map[string]any, dst any) error {
	text, err := w.callToolRaw(name, args)
	if err != nil {
		return err
	}
	if dst == nil {
		return nil
	}
	// Wails v3's mcpToolResult returns string values verbatim (not JSON-quoted)
	// in content[0].text, whereas booleans, numbers, arrays, and objects are
	// JSON-encoded. Handle *string directly so callers get the raw string.
	if s, ok := dst.(*string); ok {
		*s = text
		return nil
	}
	return json.Unmarshal([]byte(text), dst)
}

type windowDescriptor struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Focused   bool   `json:"focused"`
	Visible   bool   `json:"visible"`
	Maximized bool   `json:"maximised"` //nolint:misspell // matches upstream Wails v3 JSON field
}

func (w *wailsE2EInstance) windowsList() ([]windowDescriptor, error) {
	var list []windowDescriptor
	if err := w.callToolJSON("windows_list", nil, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (w *wailsE2EInstance) evalJS(js string, dst any) error {
	return w.callToolJSON("js_eval", map[string]any{"js": js}, dst)
}

func (w *wailsE2EInstance) callBoundMethod(method string, dst any, args ...any) error {
	if args == nil {
		args = []any{}
	}
	return w.callToolJSON("call_bound_method", map[string]any{
		"name": method,
		"args": args,
	}, dst)
}

func (w *wailsE2EInstance) emitEvent(name string, data any) error {
	args := map[string]any{"name": name}
	if data != nil {
		args["data"] = data
	}
	_, err := w.callToolRaw("emit_event", args)
	return err
}

func (w *wailsE2EInstance) waitForEvent(name string, timeout time.Duration, dst any) error {
	return w.callToolJSON("wait_for_event", map[string]any{
		"name":       name,
		"timeout_ms": timeout.Milliseconds(),
	}, dst)
}

type triggerMenuOutcome struct {
	Label     string `json:"label"`
	Found     bool   `json:"found"`
	Enabled   bool   `json:"enabled"`
	Triggered bool   `json:"triggered"`
}

// triggerNativeMenu asks the in-app E2E menu driver (compiled only under
// `mcp && !production`) to look up label on the live application menu, verify
// its enabled state, and invoke its registered OnClick callback.
//
// Because wait_for_event and emit_event travel over separate HTTP calls, a busy
// runner can occasionally deliver emit_event before wait_for_event registers
// its listener. Retrying on a short per-attempt timeout avoids timing flakes.
func (w *wailsE2EInstance) triggerNativeMenu(label string) (triggerMenuOutcome, error) {
	type waitResult struct {
		Name string             `json:"name"`
		Data triggerMenuOutcome `json:"data"`
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		done := make(chan struct {
			res waitResult
			err error
		}, 1)

		go func() {
			var r waitResult
			err := w.waitForEvent("wails-e2e:trigger-menu-result", 3*time.Second, &r)
			done <- struct {
				res waitResult
				err error
			}{res: r, err: err}
		}()

		// Give wait_for_event a head start to register its listener on Wails's
		// event bus before emitting the trigger event.
		time.Sleep(200 * time.Millisecond)
		if err := w.emitEvent("wails-e2e:trigger-menu", label); err != nil {
			return triggerMenuOutcome{}, fmt.Errorf("emit trigger-menu(%q): %w", label, err)
		}

		got := <-done
		if got.err == nil {
			return got.res.Data, nil
		}
		lastErr = got.err
	}
	return triggerMenuOutcome{}, fmt.Errorf("wait trigger-menu-result(%q) after %d attempts: %w", label, maxAttempts, lastErr)
}

// pollUntil retries fn until it returns (true, nil) or timeout expires.
func pollUntil(t *testing.T, timeout time.Duration, desc string, fn func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		ok, err := fn()
		if err == nil && ok {
			return
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s (lastErr=%v)", timeout, desc, lastErr)
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
