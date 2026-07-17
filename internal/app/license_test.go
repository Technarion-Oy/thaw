// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"testing"

	"thaw/internal/config"
)

// TestLicenseAcceptance_FreshThenAccepted covers the gate's full lifecycle: a
// fresh config reports "not accepted" (so the frontend shows the gate), and
// AcceptLicense flips the persisted flag so subsequent launches skip it.
func TestLicenseAcceptance_FreshThenAccepted(t *testing.T) {
	isolateConfig(t)
	a := &App{}

	if a.IsLicenseAccepted() {
		t.Fatal("fresh config should report the license as not yet accepted")
	}

	if err := a.AcceptLicense(); err != nil {
		t.Fatalf("AcceptLicense: %v", err)
	}

	if !a.IsLicenseAccepted() {
		t.Error("IsLicenseAccepted should be true after AcceptLicense")
	}
}

// TestAcceptLicense_PersistsToDisk confirms acceptance is written to the config
// file (not merely held in memory), so a later process reading a fresh
// config.Load sees the accepted flag and does not re-prompt.
func TestAcceptLicense_PersistsToDisk(t *testing.T) {
	isolateConfig(t)
	a := &App{}

	if err := a.AcceptLicense(); err != nil {
		t.Fatalf("AcceptLicense: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !cfg.LicenseAccepted {
		t.Error("on-disk config.LicenseAccepted should be true after AcceptLicense")
	}
}

// TestGetLicenseText_ReturnsEmbedded confirms GetLicenseText serves the exact
// text handed to NewApp (the embedded LICENSE in production).
func TestGetLicenseText_ReturnsEmbedded(t *testing.T) {
	const licenseText = "GNU GENERAL PUBLIC LICENSE\nVersion 3"
	a := NewApp("", licenseText)

	if got := a.GetLicenseText(); got != licenseText {
		t.Errorf("GetLicenseText() = %q, want %q", got, licenseText)
	}
}
