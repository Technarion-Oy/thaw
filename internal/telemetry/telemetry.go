// SPDX-License-Identifier: GPL-3.0-or-later

// Package telemetry collects anonymous usage events to help understand how
// the application is used. No personally identifiable information, SQL query
// content, credentials, or account-specific identifiers are ever recorded.
//
// Events are currently written to the application log only. When an analytics
// backend is chosen, wire it in the sendRemote placeholder below.
//
// thaw:domain: Core IPC & App Lifecycle
package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"runtime"
	"sync"
	"time"

	"thaw/internal/logger"
)

// ── Event names ──────────────────────────────────────────────────────────────

// Event is a named telemetry event.
type Event string

// Application lifecycle
const (
	EventAppStarted Event = "app.started"
	EventAppStopped Event = "app.stopped"
)

// Snowflake connection
const (
	EventConnected        Event = "snowflake.connected"
	EventConnectionFailed Event = "snowflake.connection_failed"
	EventDisconnected     Event = "snowflake.disconnected"
)

// Query execution
const (
	EventQueryStarted   Event = "query.started"
	EventQueryCompleted Event = "query.completed"
	EventQueryFailed    Event = "query.failed"
	EventQueryCancelled Event = "query.cancelled"
)

// Feature usage — extend as new features are added
const (
	EventFeatureERDiagram  Event = "feature.er_diagram"
	EventFeatureERDesigner Event = "feature.er_designer"
	EventFeatureTimeTravel Event = "feature.time_travel"
	EventFeatureExportDDL  Event = "feature.export_ddl"
	EventFeatureExportData Event = "feature.export_data"
	EventFeatureImportData Event = "feature.import_data"
	EventFeatureGitCommit  Event = "feature.git_commit"
	EventFeatureUndrop     Event = "feature.undrop"
)

// ── Client ───────────────────────────────────────────────────────────────────

// Props is a map of event properties. Must never contain PII, SQL text,
// credentials, or account-specific identifiers.
type Props map[string]any

// Client is a telemetry client. Use the package-level functions for convenience.
type Client struct {
	mu        sync.Mutex
	sessionID string
	version   string
	os        string
	startedAt time.Time
}

// Default is the package-level client initialized by Init.
var Default *Client

// Init creates the default telemetry client. Call once at application startup.
func Init(version string) {
	Default = &Client{
		sessionID: newSessionID(),
		version:   version,
		os:        runtime.GOOS,
		startedAt: time.Now(),
	}
}

// Track records an event on the default client. Safe to call before Init (no-op).
func Track(event Event, props Props) {
	Default.Track(event, props)
}

// Track records an event. Safe to call on a nil receiver.
func (c *Client) Track(event Event, props Props) {
	if c == nil {
		return
	}
	c.mu.Lock()
	sid := c.sessionID
	ver := c.version
	goos := c.os
	c.mu.Unlock()

	// Merge caller props with session-level context.
	enriched := Props{
		"session": sid,
		"version": ver,
		"os":      goos,
	}
	for k, v := range props {
		enriched[k] = v
	}

	// Write to the application log at DEBUG level.
	attrs := make([]any, 0, len(enriched)*2+2)
	attrs = append(attrs, "event", string(event))
	for k, v := range enriched {
		attrs = append(attrs, k, v)
	}
	logger.L.Debug("telemetry", attrs...)

	// TODO: send to a remote analytics backend.
	//
	// Candidate services (self-hostable or SaaS):
	//   - PostHog  https://posthog.com  (go get github.com/posthog/posthog-go)
	//   - Segment  https://segment.com
	//   - Mixpanel https://mixpanel.com
	//
	// Example with PostHog:
	//   client, _ := posthog.NewWithConfig(apiKey, posthog.Config{Endpoint: "..."})
	//   client.Enqueue(posthog.Capture{
	//       DistinctId: sid,
	//       Event:      string(event),
	//       Properties: posthog.NewProperties().Set("version", ver).Set("os", goos),
	//   })
	//
	// Example with a custom HTTP endpoint:
	//   go sendRemote(event, enriched)
}

// SessionDuration returns elapsed time since Init was called.
func SessionDuration() time.Duration {
	if Default == nil {
		return 0
	}
	return time.Since(Default.startedAt)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// newSessionID returns a random 16-byte hex string used as a per-session
// anonymous identifier. It is not persisted across restarts.
func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// sendRemote is the placeholder for delivering an event to a remote backend.
// Implement this function when an analytics service is chosen.
//
//nolint:unused
func sendRemote(_ Event, _ Props) {
	// TODO: implement remote event delivery.
}
