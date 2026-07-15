// SPDX-License-Identifier: GPL-3.0-or-later

package gitrepo

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	gogithttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// CredentialResult is the IPC-safe credential lookup result.
// The actual secret is never included — only discovery metadata.
type CredentialResult struct {
	Found    bool   `json:"found"`
	Username string `json:"username"`
	Source   string `json:"source"` // "keychain" | "credential-manager" | "git-credentials" | "netrc"
}

// storedCreds is the internal representation that includes the secret.
// It must never leave the gitrepo package.
type storedCreds struct {
	username string
	password string
	source   string
}

// resolveAuth returns the appropriate go-git AuthMethod for the given parameters.
//
//   - "bearer" / "oauth" → http.TokenAuth (Authorization: Bearer <token>), required by Azure DevOps Entra OAuth
//   - "stored"  → http.BasicAuth with credentials from OS keychain / ~/.git-credentials / ~/.netrc
//   - "pat" / "" → http.BasicAuth{Username: "x-access-token", Password: token}
func resolveAuth(remoteURL, authMethod, token string) transport.AuthMethod {
	switch authMethod {
	case "bearer", "oauth":
		if token != "" {
			// GitHub's Git over HTTPS server does not support the Bearer header.
			// If we are on github.com, we must use Basic Auth even for OAuth tokens.
			if strings.Contains(remoteURL, "github.com") {
				return &gogithttp.BasicAuth{Username: "x-access-token", Password: token}
			}
			return &gogithttp.TokenAuth{Token: token}
		}
	case "stored":
		if c := lookupStoredCredentials(remoteURL); c != nil {
			return &gogithttp.BasicAuth{Username: c.username, Password: c.password}
		}
	default: // "pat" or ""
		if token != "" {
			return &gogithttp.BasicAuth{Username: "x-access-token", Password: token}
		}
	}
	return nil
}

// LookupCredentials probes all credential sources for remoteURL and returns
// a result safe to send to the frontend (no secrets).
func LookupCredentials(remoteURL string) CredentialResult {
	c := lookupStoredCredentials(remoteURL)
	if c == nil {
		return CredentialResult{}
	}
	return CredentialResult{Found: true, Username: c.username, Source: c.source}
}

// lookupStoredCredentials tries all credential sources in priority order.
func lookupStoredCredentials(remoteURL string) *storedCreds {
	u, err := url.Parse(remoteURL)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	host := u.Hostname()

	if c := readGitCredentialsFile(host); c != nil {
		return c
	}
	if c := readNetrc(host); c != nil {
		return c
	}
	return lookupOSCredentials(host)
}

// readGitCredentialsFile reads ~/.git-credentials and returns the first matching entry.
// Lines have the form: https://username:password@hostname
func readGitCredentialsFile(host string) *storedCreds {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	f, err := os.Open(filepath.Join(home, ".git-credentials"))
	if err != nil {
		return nil
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("failed to close file: %v\n", err)
		}
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := url.Parse(line)
		if err != nil || u.Hostname() != host || u.User == nil {
			continue
		}
		pw, hasPw := u.User.Password()
		if !hasPw || u.User.Username() == "" || pw == "" {
			continue
		}
		return &storedCreds{
			username: u.User.Username(),
			password: pw,
			source:   "git-credentials",
		}
	}
	return nil
}

// readNetrc reads ~/.netrc and returns the first matching machine entry.
// Entries have the form: machine hostname login username password thepassword
func readNetrc(host string) *storedCreds {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	f, err := os.Open(filepath.Join(home, ".netrc"))
	if err != nil {
		return nil
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("failed to close file: %v\n", err)
		}
	}()

	type parseState int
	const (
		stateIdle      parseState = iota
		stateInMachine            // currently parsing a matching machine block
	)

	var cur parseState
	var user, pass string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		tokens := strings.Fields(scanner.Text())
		for i := 0; i < len(tokens); i++ {
			switch tokens[i] {
			case "machine":
				if i+1 < len(tokens) {
					if tokens[i+1] == host {
						cur = stateInMachine
						user, pass = "", ""
					} else {
						if cur == stateInMachine {
							// left the matching block without finding complete creds
							cur = stateIdle
						} else {
							cur = stateIdle
						}
					}
					i++
				}
			case "login":
				if cur == stateInMachine && i+1 < len(tokens) {
					user = tokens[i+1]
					i++
				}
			case "password":
				if cur == stateInMachine && i+1 < len(tokens) {
					pass = tokens[i+1]
					i++
				}
			}
		}
		if cur == stateInMachine && user != "" && pass != "" {
			return &storedCreds{username: user, password: pass, source: "netrc"}
		}
	}
	return nil
}
