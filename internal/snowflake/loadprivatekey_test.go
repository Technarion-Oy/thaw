// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/youmark/pkcs8"
)

func writePEM(t *testing.T, dir, name, blockType string, der []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// TestLoadPrivateKey covers the key formats loadPrivateKey must accept. The
// encrypted-PKCS#8 case is the issue #804 F2 regression: keys Thaw's own
// generator produces (openssl -topk8 -passout / ssh-keygen) are "BEGIN
// ENCRYPTED PRIVATE KEY", which the old x509.DecryptPEMBlock path could not
// load.
func TestLoadPrivateKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pkcs8DER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	const pass = "s3cr3t-pass"
	encDER, err := pkcs8.MarshalPrivateKey(key, []byte(pass), nil)
	if err != nil {
		t.Fatalf("marshal encrypted pkcs8: %v", err)
	}

	dir := t.TempDir()
	unencPKCS8 := writePEM(t, dir, "unenc_pkcs8.pem", "PRIVATE KEY", pkcs8DER)
	pkcs1 := writePEM(t, dir, "pkcs1.pem", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))
	encPKCS8 := writePEM(t, dir, "enc_pkcs8.pem", "ENCRYPTED PRIVATE KEY", encDER)

	tests := []struct {
		name       string
		path       string
		passphrase string
		wantErr    bool
	}{
		{"unencrypted PKCS#8 (Thaw \"go\" method)", unencPKCS8, "", false},
		{"unencrypted PKCS#8, spurious passphrase ignored", unencPKCS8, "unused", false},
		{"PKCS#1", pkcs1, "", false},
		{"encrypted PKCS#8 (Thaw openssl/ssh-keygen)", encPKCS8, pass, false},
		{"encrypted PKCS#8, wrong passphrase", encPKCS8, "wrong", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadPrivateKey(tt.path, tt.passphrase)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("loadPrivateKey: %v", err)
			}
			if !got.Equal(key) {
				t.Fatalf("loaded key does not match original")
			}
		})
	}
}
