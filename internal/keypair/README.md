# internal/keypair

> RSA-2048 key-pair generation (Go crypto, openssl, or ssh-keygen) and ALTER USER RSA_PUBLIC_KEY SQL builder for Snowflake key-pair authentication.

## Responsibility

Generates an RSA-2048 key pair using one of three selectable methods, writes the
private and public key files to disk, and returns a `KeyPairResult` containing
the file paths plus the public key payload stripped of PEM headers — the exact
format required by Snowflake's `ALTER USER ... SET RSA_PUBLIC_KEY = '...'`.

Also provides `BuildSetUserPublicKeySQL` to construct that `ALTER USER` statement
and `CheckAvailableKeyTools` to discover which generation methods are available on
the current system.

## Key files

| File | Purpose |
|------|---------|
| `doc.go` | Package doc + `thaw:domain` annotation (Object Browser & Administration) |
| `keypair.go` | `KeyPairResult`, `CheckAvailableKeyTools`, `GenerateKeyPair`, three private generators, `BuildSetUserPublicKeySQL`, `stripPEMContent` |
| `keypair_test.go` | Unit tests for key generation and SQL builder |

## Key types & functions

### `KeyPairResult`
```go
type KeyPairResult struct {
    PrivateKeyPath string `json:"privateKeyPath"`
    PublicKeyPath  string `json:"publicKeyPath"`
    PublicKey      string `json:"publicKey"` // base64 only, no PEM headers
}
```
`PublicKey` is ready to paste into `ALTER USER ... SET RSA_PUBLIC_KEY = '...'`
or copy from the UI.

### `CheckAvailableKeyTools() []string`
Returns `["go"]` always, plus `"openssl"` and/or `"ssh-keygen"` when those
executables are found on `PATH` via `exec.LookPath`. Called on startup to populate
the key-generation method selector in the UI.

### `GenerateKeyPair(method, privateKeyPath, passphrase string) (KeyPairResult, error)`
Dispatches to one of three generators:

| `method` | Implementation | Passphrase support | Private key format |
|----------|---------------|-------------------|--------------------|
| `"go"` | `crypto/rsa` + `x509.MarshalPKCS8PrivateKey` | No | PKCS#8 PEM |
| `"openssl"` | `openssl genrsa` piped to `openssl pkcs8` | Yes (AES-256) | PKCS#8 PEM |
| `"ssh-keygen"` | `ssh-keygen -t rsa -b 2048` + PEM export | Yes | OpenSSH/PKCS8 |

All generators:
- Create the parent directory with `0700` permissions if absent.
- Write the private key with `0600` permissions.
- Write the public key as `<privateKeyPath_without_ext>_pub.pem` with `0644`.

### `BuildSetUserPublicKeySQL(username, publicKey string) string`
Returns `ALTER USER "<username>" SET RSA_PUBLIC_KEY='<publicKey>'`.
- Username is double-quote-escaped.
- `publicKey` should be the bare base64 string from `KeyPairResult.PublicKey`
  (PEM headers already stripped).

### `stripPEMContent(pemStr string) string`
Removes `-----BEGIN/END ...-----` header/footer lines and blank lines from a PEM
string, returning the concatenated base64 payload as a single string.

## Patterns & integration (thin-delegator)

```go
// internal/app/keypair.go (illustrative)
func (a *App) GenerateKeyPair(method, path, passphrase string) (keypair.KeyPairResult, error) {
    return keypair.GenerateKeyPair(method, path, passphrase)
}

func (a *App) BuildSetUserPublicKeySQL(username, publicKey string) string {
    return keypair.BuildSetUserPublicKeySQL(username, publicKey)
}
```

`GenerateKeyPair` does not require a live Snowflake connection. It only touches
the local filesystem and (for `openssl`/`ssh-keygen`) spawns subprocess. The
`*App` delegator does not nil-check `a.client` before calling it.

## Gotchas

- The `"go"` method does not support passphrases — the passphrase parameter is
  silently ignored. Callers should communicate this limitation to the user in the
  UI (the method selector should hide the passphrase field when `"go"` is selected).
- The `"openssl"` method generates a raw RSA key with `openssl genrsa` and pipes
  it in-memory to `openssl pkcs8`. The raw key is never saved to disk, only the
  PKCS#8 output. The passphrase is passed on the command line via `-passout
  pass:<passphrase>`, which may be visible in process listings on some systems.
- The `"ssh-keygen"` method saves a `.pub` file alongside the private key (the
  OpenSSH format), then converts it to PKCS#8 PEM and saves that as `_pub.pem`.
  The original `.pub` file is left on disk.
- `stripPEMContent` joins all non-header lines without any separator, producing
  a single unbroken base64 string. Snowflake's `RSA_PUBLIC_KEY` parameter accepts
  this format, but some PEM validators expect 64-character line breaks.
