// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package keypair

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBuildSetUserPublicKeySQL(t *testing.T) {
	sql := BuildSetUserPublicKeySQL(`O"Brien`, "MIIB'key")
	want := `ALTER USER "O""Brien" SET RSA_PUBLIC_KEY='MIIB''key'`
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
}

func TestCheckAvailableKeyToolsAlwaysHasGo(t *testing.T) {
	tools := CheckAvailableKeyTools()
	if !slices.Contains(tools, "go") {
		t.Errorf("expected 'go' to always be available, got %v", tools)
	}
}

func TestGenerateKeyPairGo(t *testing.T) {
	dir := t.TempDir()
	privPath := filepath.Join(dir, "key.p8")

	res, err := GenerateKeyPair("go", privPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.PrivateKeyPath != privPath {
		t.Errorf("unexpected private key path %q", res.PrivateKeyPath)
	}
	wantPub := filepath.Join(dir, "key_pub.pem")
	if res.PublicKeyPath != wantPub {
		t.Errorf("got public key path %q, want %q", res.PublicKeyPath, wantPub)
	}

	priv, err := os.ReadFile(privPath)
	if err != nil {
		t.Fatalf("private key not written: %v", err)
	}
	if !strings.Contains(string(priv), "BEGIN PRIVATE KEY") {
		t.Errorf("private key not in PKCS#8 PEM form:\n%s", priv)
	}

	// PublicKey is the stripped base64 payload — no PEM headers or whitespace.
	if strings.Contains(res.PublicKey, "-----") || strings.ContainsAny(res.PublicKey, " \n\t") {
		t.Errorf("public key should be stripped of PEM headers/whitespace, got %q", res.PublicKey)
	}
	if res.PublicKey == "" {
		t.Error("expected non-empty stripped public key")
	}
}

func TestGenerateKeyPairUnknownMethod(t *testing.T) {
	if _, err := GenerateKeyPair("bogus", filepath.Join(t.TempDir(), "k.p8"), ""); err == nil {
		t.Error("expected error for unknown method")
	}
}
