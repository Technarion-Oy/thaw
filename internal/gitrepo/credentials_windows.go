// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package gitrepo

import (
	"github.com/danieljoos/wincred"
)

// lookupOSCredentials queries the Windows Credential Manager.
// Git for Windows stores credentials with the target "git:https://hostname".
func lookupOSCredentials(host string) *storedCreds {
	// Git Credential Manager stores entries as "git:https://<host>"
	targets := []string{
		"git:https://" + host,
		"git:http://" + host,
		host,
	}
	for _, target := range targets {
		cred, err := wincred.GetGenericCredential(target)
		if err != nil || cred == nil {
			continue
		}
		password := string(cred.CredentialBlob)
		if cred.UserName != "" && password != "" {
			return &storedCreds{
				username: cred.UserName,
				password: password,
				source:   "credential-manager",
			}
		}
	}
	return nil
}
