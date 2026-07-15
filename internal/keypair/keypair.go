// SPDX-License-Identifier: GPL-3.0-or-later

package keypair

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// KeyPairResult holds the paths and public key content produced by GenerateKeyPair.
type KeyPairResult struct {
	PrivateKeyPath string `json:"privateKeyPath"`
	PublicKeyPath  string `json:"publicKeyPath"`
	PublicKey      string `json:"publicKey"` // stripped of PEM headers, ready for ALTER USER
}

// CheckAvailableKeyTools returns the list of available key generation methods.
// "go" (Go built-in crypto) is always present. "openssl" and "ssh-keygen" are
// included only when their executables are found on PATH.
func CheckAvailableKeyTools() []string {
	tools := []string{"go"}
	if _, err := exec.LookPath("openssl"); err == nil {
		tools = append(tools, "openssl")
	}
	if _, err := exec.LookPath("ssh-keygen"); err == nil {
		tools = append(tools, "ssh-keygen")
	}
	return tools
}

// GenerateKeyPair generates an RSA-2048 key pair using the specified method.
//
//   - "go"        — pure Go crypto (PKCS#8, no passphrase support)
//   - "openssl"   — openssl CLI (PKCS#8; passphrase encrypts the private key)
//   - "ssh-keygen"— ssh-keygen CLI (OpenSSH/PKCS8 private key; passphrase
//     encrypts the private key; public key saved as PKCS8 PEM)
func GenerateKeyPair(method, privateKeyPath, passphrase string) (KeyPairResult, error) {
	if err := os.MkdirAll(filepath.Dir(privateKeyPath), 0700); err != nil {
		return KeyPairResult{}, fmt.Errorf("cannot create directory: %w", err)
	}
	switch method {
	case "go":
		return generateKeyPairGo(privateKeyPath)
	case "openssl":
		return generateKeyPairOpenSSL(privateKeyPath, passphrase)
	case "ssh-keygen":
		return generateKeyPairSSHKeygen(privateKeyPath, passphrase)
	default:
		return KeyPairResult{}, fmt.Errorf("unknown key generation method %q", method)
	}
}

// generateKeyPairGo uses the Go standard library to produce an unencrypted
// RSA-2048 PKCS#8 private key and a PKIX public key. Passphrase is not supported.
func generateKeyPairGo(privateKeyPath string) (KeyPairResult, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("key generation failed: %w", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("PKCS8 marshal failed: %w", err)
	}
	var privBuf bytes.Buffer
	if err = pem.Encode(&privBuf, &pem.Block{Type: "PRIVATE KEY", Bytes: privDER}); err != nil {
		return KeyPairResult{}, err
	}
	if err = os.WriteFile(privateKeyPath, privBuf.Bytes(), 0600); err != nil {
		return KeyPairResult{}, fmt.Errorf("cannot write private key: %w", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("public key marshal failed: %w", err)
	}
	ext := filepath.Ext(privateKeyPath)
	pubKeyPath := strings.TrimSuffix(privateKeyPath, ext) + "_pub.pem"
	var pubBuf bytes.Buffer
	if err = pem.Encode(&pubBuf, &pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}); err != nil {
		return KeyPairResult{}, err
	}
	if err = os.WriteFile(pubKeyPath, pubBuf.Bytes(), 0644); err != nil {
		return KeyPairResult{}, fmt.Errorf("cannot write public key: %w", err)
	}
	return KeyPairResult{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  pubKeyPath,
		PublicKey:      stripPEMContent(pubBuf.String()),
	}, nil
}

// generateKeyPairOpenSSL uses the openssl CLI to produce a PKCS#8 private key
// (encrypted with AES-256 if passphrase is non-empty) and a PKIX public key.
func generateKeyPairOpenSSL(privateKeyPath, passphrase string) (KeyPairResult, error) {
	tool, err := exec.LookPath("openssl")
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("openssl not found in PATH")
	}

	// Step 1: generate raw RSA-2048 key (piped, never saved to disk).
	rawPEM, err := exec.Command(tool, "genrsa", "2048").Output()
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("openssl genrsa failed: %w", err)
	}

	// Step 2: convert to PKCS#8 and write to privateKeyPath.
	pkcs8Args := []string{"pkcs8", "-topk8", "-inform", "PEM", "-outform", "PEM", "-out", privateKeyPath}
	if passphrase != "" {
		pkcs8Args = append(pkcs8Args, "-passout", "pass:"+passphrase)
	} else {
		pkcs8Args = append(pkcs8Args, "-nocrypt")
	}
	pkcs8Cmd := exec.Command(tool, pkcs8Args...)
	pkcs8Cmd.Stdin = strings.NewReader(string(rawPEM))
	if out, err2 := pkcs8Cmd.CombinedOutput(); err2 != nil {
		return KeyPairResult{}, fmt.Errorf("openssl pkcs8 failed: %s", strings.TrimSpace(string(out)))
	}
	_ = os.Chmod(privateKeyPath, 0600)

	// Step 3: extract public key.
	ext := filepath.Ext(privateKeyPath)
	pubKeyPath := strings.TrimSuffix(privateKeyPath, ext) + "_pub.pem"
	rsaArgs := []string{"rsa", "-in", privateKeyPath, "-pubout", "-out", pubKeyPath}
	if passphrase != "" {
		rsaArgs = append(rsaArgs, "-passin", "pass:"+passphrase)
	}
	if out, err2 := exec.Command(tool, rsaArgs...).CombinedOutput(); err2 != nil {
		return KeyPairResult{}, fmt.Errorf("openssl rsa pubout failed: %s", strings.TrimSpace(string(out)))
	}

	pubPEM, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("cannot read public key: %w", err)
	}
	return KeyPairResult{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  pubKeyPath,
		PublicKey:      stripPEMContent(string(pubPEM)),
	}, nil
}

// generateKeyPairSSHKeygen uses ssh-keygen to produce an RSA-2048 private key
// (encrypted if passphrase is non-empty) and converts the public key to
// PKCS#8 PEM format suitable for Snowflake.
func generateKeyPairSSHKeygen(privateKeyPath, passphrase string) (KeyPairResult, error) {
	tool, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("ssh-keygen not found in PATH")
	}

	// Generate RSA-2048 key pair. -N sets the passphrase ("" = none).
	genArgs := []string{"-t", "rsa", "-b", "2048", "-f", privateKeyPath, "-N", passphrase}
	if out, err2 := exec.Command(tool, genArgs...).CombinedOutput(); err2 != nil {
		return KeyPairResult{}, fmt.Errorf("ssh-keygen failed: %s", strings.TrimSpace(string(out)))
	}
	_ = os.Chmod(privateKeyPath, 0600)

	// Convert the OpenSSH public key to PKCS#8 PEM format for Snowflake.
	sshPubPath := privateKeyPath + ".pub"
	pubPEMBytes, err := exec.Command(tool, "-e", "-m", "pkcs8", "-f", sshPubPath).Output()
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("ssh-keygen public key export failed: %w", err)
	}
	ext := filepath.Ext(privateKeyPath)
	pubKeyPath := strings.TrimSuffix(privateKeyPath, ext) + "_pub.pem"
	if err = os.WriteFile(pubKeyPath, pubPEMBytes, 0644); err != nil {
		return KeyPairResult{}, fmt.Errorf("cannot write public key: %w", err)
	}
	return KeyPairResult{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  pubKeyPath,
		PublicKey:      stripPEMContent(string(pubPEMBytes)),
	}, nil
}

// stripPEMContent returns the base64 payload of a PEM block with all header,
// footer, and blank lines removed — the format expected by ALTER USER SET RSA_PUBLIC_KEY.
func stripPEMContent(pemStr string) string {
	var lines []string
	for _, line := range strings.Split(pemStr, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "-----") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "")
}

// BuildSetUserPublicKeySQL builds the ALTER USER ... SET RSA_PUBLIC_KEY statement
// that registers an RSA public key with a Snowflake user.
func BuildSetUserPublicKeySQL(username, publicKey string) string {
	esc := strings.ReplaceAll(username, `"`, `""`)
	sq := strings.ReplaceAll(publicKey, "'", "''")
	return fmt.Sprintf(`ALTER USER "%s" SET RSA_PUBLIC_KEY='%s'`, esc, sq)
}
