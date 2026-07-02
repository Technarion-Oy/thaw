// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"os"
	"testing"
)

// workdirOverrideArg drives the per-instance working-directory override used by
// "Open Folder in New Window"; a regression here silently reverts new windows to
// the global folder, so pin the arg + env-fallback precedence.
func TestWorkdirOverrideArg(t *testing.T) {
	origArgs := os.Args
	origEnv, hadEnv := os.LookupEnv("THAW_WORKDIR")
	t.Cleanup(func() {
		os.Args = origArgs
		if hadEnv {
			os.Setenv("THAW_WORKDIR", origEnv)
		} else {
			os.Unsetenv("THAW_WORKDIR")
		}
	})

	cases := []struct {
		name string
		args []string
		env  string
		want string
	}{
		{"arg wins", []string{"thaw", "--workdir=/a/b"}, "/env/dir", "/a/b"},
		{"env fallback", []string{"thaw"}, "/env/dir", "/env/dir"},
		{"neither", []string{"thaw"}, "", ""},
		{"arg with spaces", []string{"thaw", "--workdir=/My Projects/x"}, "", "/My Projects/x"},
		{"ignores other flags", []string{"thaw", "--foo", "--workdir=/z"}, "", "/z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			os.Args = c.args
			if c.env == "" {
				os.Unsetenv("THAW_WORKDIR")
			} else {
				os.Setenv("THAW_WORKDIR", c.env)
			}
			if got := workdirOverrideArg(); got != c.want {
				t.Errorf("workdirOverrideArg() = %q, want %q", got, c.want)
			}
		})
	}
}
