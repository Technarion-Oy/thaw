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
	"reflect"
	"testing"
)

// unionRecentDirs backs the atomic AddRecentDir merge; a regression drops another
// window's concurrently-added recent entry or breaks the newest-first cap.
func TestUnionRecentDirs(t *testing.T) {
	cases := []struct {
		name           string
		primary, extra []string
		want           []string
	}{
		{"prepend + backfill disk", []string{"/new"}, []string{"/a", "/b"}, []string{"/new", "/a", "/b"}},
		{"dedupe across lists", []string{"/a"}, []string{"/a", "/b"}, []string{"/a", "/b"}},
		{"drop empties", []string{""}, []string{"/a", ""}, []string{"/a"}},
		{"cap at 8", []string{"/1", "/2", "/3", "/4", "/5"}, []string{"/6", "/7", "/8", "/9"}, []string{"/1", "/2", "/3", "/4", "/5", "/6", "/7", "/8"}},
		{"both empty", nil, nil, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := unionRecentDirs(c.primary, c.extra); !reflect.DeepEqual(got, c.want) {
				t.Errorf("unionRecentDirs(%v, %v) = %v, want %v", c.primary, c.extra, got, c.want)
			}
		})
	}
}

// workdirOverrideArg drives the per-instance working-directory override used by
// "Open Folder in New Window"; a regression here silently reverts new windows to
// the global folder, so pin the arg + env-fallback precedence.
func TestWorkdirOverrideArg(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"present", []string{"thaw", "--workdir=/a/b"}, "/a/b"},
		{"absent", []string{"thaw"}, ""},
		{"arg with spaces", []string{"thaw", "--workdir=/My Projects/x"}, "/My Projects/x"},
		{"ignores other flags", []string{"thaw", "--foo", "--workdir=/z"}, "/z"},
		{"empty value ignored (no env fallback)", []string{"thaw", "--workdir="}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			os.Args = c.args
			if got := workdirOverrideArg(); got != c.want {
				t.Errorf("workdirOverrideArg() = %q, want %q", got, c.want)
			}
		})
	}
}
