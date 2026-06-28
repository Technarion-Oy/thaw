package gitrepo

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	gogithttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// initRepoWithCommit creates a repo in a temp dir with a single committed file
// (a.sql) and returns the repo directory.
func initRepoWithCommit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.sql"), []byte("create table a;\n"), 0o644); err != nil {
		t.Fatalf("write a.sql: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if err := wt.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := wt.Commit("init", &gogit.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return dir
}

// statusSets returns the set of paths on the staged and unstaged sides.
func statusSets(t *testing.T, dir string) (staged, unstaged map[string]string) {
	t.Helper()
	s, err := GetStatus(dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	staged, unstaged = map[string]string{}, map[string]string{}
	for _, f := range s.Staged {
		staged[f.Path] = f.Status
	}
	for _, f := range s.Unstaged {
		unstaged[f.Path] = f.Status
	}
	return staged, unstaged
}

func TestStagingFlow(t *testing.T) {
	dir := initRepoWithCommit(t)

	// Modify the tracked file and add an untracked one.
	if err := os.WriteFile(filepath.Join(dir, "a.sql"), []byte("create table a2;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.sql"), []byte("create table b;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Initially everything is unstaged.
	staged, unstaged := statusSets(t, dir)
	if len(staged) != 0 {
		t.Fatalf("expected nothing staged, got %v", staged)
	}
	if unstaged["a.sql"] != "M" || unstaged["b.sql"] != "U" {
		t.Fatalf("unexpected unstaged set: %v", unstaged)
	}

	// Stage a.sql → moves to the staged side; b.sql remains unstaged.
	if err := StageFile(dir, "a.sql"); err != nil {
		t.Fatalf("StageFile: %v", err)
	}
	staged, unstaged = statusSets(t, dir)
	if staged["a.sql"] != "M" {
		t.Fatalf("expected a.sql staged M, got %v", staged)
	}
	if _, ok := unstaged["b.sql"]; !ok {
		t.Fatalf("expected b.sql still unstaged, got %v", unstaged)
	}

	// Unstage a.sql → back to unstaged.
	if err := UnstageFile(dir, "a.sql"); err != nil {
		t.Fatalf("UnstageFile: %v", err)
	}
	staged, unstaged = statusSets(t, dir)
	if len(staged) != 0 {
		t.Fatalf("expected nothing staged after unstage, got %v", staged)
	}
	if unstaged["a.sql"] != "M" {
		t.Fatalf("expected a.sql unstaged M, got %v", unstaged)
	}

	// StageAll → both staged (a modified, b added).
	if err := StageAll(dir); err != nil {
		t.Fatalf("StageAll: %v", err)
	}
	staged, _ = statusSets(t, dir)
	if staged["a.sql"] != "M" || staged["b.sql"] != "A" {
		t.Fatalf("unexpected staged set after StageAll: %v", staged)
	}

	// UnstageAll → nothing staged.
	if err := UnstageAll(dir); err != nil {
		t.Fatalf("UnstageAll: %v", err)
	}
	staged, _ = statusSets(t, dir)
	if len(staged) != 0 {
		t.Fatalf("expected nothing staged after UnstageAll, got %v", staged)
	}
}

func TestDiscardFile(t *testing.T) {
	dir := initRepoWithCommit(t)

	// Discard a tracked modification → file restored to HEAD content.
	if err := os.WriteFile(filepath.Join(dir, "a.sql"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := DiscardFile(dir, "a.sql"); err != nil {
		t.Fatalf("DiscardFile tracked: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "a.sql"))
	if string(got) != "create table a;\n" {
		t.Fatalf("expected a.sql restored to HEAD, got %q", string(got))
	}

	// Discard an untracked file → deleted from disk.
	if err := os.WriteFile(filepath.Join(dir, "c.sql"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := DiscardFile(dir, "c.sql"); err != nil {
		t.Fatalf("DiscardFile untracked: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "c.sql")); !os.IsNotExist(err) {
		t.Fatalf("expected c.sql removed, stat err = %v", err)
	}

	staged, unstaged := statusSets(t, dir)
	if len(staged) != 0 || len(unstaged) != 0 {
		t.Fatalf("expected clean tree after discards, got staged=%v unstaged=%v", staged, unstaged)
	}
}

// Discarding a tracked file must restore its original mode (executable bit) and
// recreate its parent directory if it was deleted — otherwise the tree isn't
// actually clean afterwards.
func TestDiscardFileRestoresModeAndRecreatesDir(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	sub := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(sub, "run.sh")
	const orig = "#!/bin/sh\necho hi\n"
	if err := os.WriteFile(script, []byte(orig), 0o755); err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	if err := wt.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("init", &gogit.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}

	// Modify the file, then delete its whole parent directory.
	if err := os.WriteFile(script, []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(sub); err != nil {
		t.Fatal(err)
	}

	if err := DiscardFile(dir, "scripts/run.sh"); err != nil {
		t.Fatalf("DiscardFile: %v", err)
	}

	got, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("read restored file (parent dir not recreated?): %v", err)
	}
	if string(got) != orig {
		t.Fatalf("content not restored: %q", string(got))
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(script)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o100 == 0 {
			t.Fatalf("executable bit not restored, mode = %v", info.Mode())
		}
	}

	staged, unstaged := statusSets(t, dir)
	if len(staged) != 0 || len(unstaged) != 0 {
		t.Fatalf("expected clean tree after discard, got staged=%v unstaged=%v", staged, unstaged)
	}
}

// A file that was staged (git add) and then edited again appears on BOTH the
// staged side ("A") and the unstaged side ("M"), and has no committed version —
// so DiscardFile deletes it. The UI must classify it as "new" from the staged
// "A" (not the unstaged "M" display letter), or it would warn "discard changes"
// while permanently deleting the file. This test pins that backend contract.
func TestStagedThenModifiedIsNewAndDiscarded(t *testing.T) {
	dir := initRepoWithCommit(t)
	target := filepath.Join(dir, "new.sql")
	if err := os.WriteFile(target, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := StageFile(dir, "new.sql"); err != nil {
		t.Fatalf("StageFile: %v", err)
	}
	if err := os.WriteFile(target, []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	staged, unstaged := statusSets(t, dir)
	if staged["new.sql"] != "A" {
		t.Fatalf("expected staged side A, got %v", staged)
	}
	if unstaged["new.sql"] != "M" {
		t.Fatalf("expected unstaged side M, got %v", unstaged)
	}

	if err := DiscardFile(dir, "new.sql"); err != nil {
		t.Fatalf("DiscardFile: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected new.sql deleted (no committed version), stat err = %v", err)
	}
}

// ChangedPaths.IsNew must be true exactly for files with no committed version
// (so the UI knows discard deletes rather than reverts), including a
// staged-new-then-modified file whose display letter is "M".
func TestChangedPathsIsNew(t *testing.T) {
	dir := initRepoWithCommit(t) // commits a.sql

	// A second committed file we'll delete (tracked delete → not new).
	if err := os.WriteFile(filepath.Join(dir, "keep.sql"), []byte("k\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo, _ := gogit.PlainOpen(dir)
	wt, _ := repo.Worktree()
	if err := wt.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("c2", &gogit.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.sql", "modified\n")               // tracked modified → not new
	write("new.sql", "n\n")                    // untracked → new
	write("staged.sql", "s\n")                 // staged-new → new
	if err := StageFile(dir, "staged.sql"); err != nil {
		t.Fatal(err)
	}
	write("both.sql", "b1\n")                  // staged-new then modified → new (displays M)
	if err := StageFile(dir, "both.sql"); err != nil {
		t.Fatal(err)
	}
	write("both.sql", "b2\n")
	if err := os.Remove(filepath.Join(dir, "keep.sql")); err != nil { // tracked deleted → not new
		t.Fatal(err)
	}

	s, err := GetStatus(dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	cases := map[string]bool{
		"a.sql": false, "new.sql": true, "staged.sql": true, "both.sql": true, "keep.sql": false,
	}
	for path, want := range cases {
		cf, ok := s.ChangedPaths[path]
		if !ok {
			t.Errorf("%s missing from ChangedPaths", path)
			continue
		}
		if cf.IsNew != want {
			t.Errorf("%s IsNew=%v, want %v (status=%q)", path, cf.IsNew, want, cf.Status)
		}
	}
}

func TestNormaliseHTTPS(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"", ""},
		{"https://github.com/org/repo.git", "https://github.com/org/repo.git"},
		{"git@github.com:org/repo.git", "https://github.com/org/repo.git"},
		{"git@gitlab.com:group/project.git", "https://gitlab.com/group/project.git"},
		{"ssh://git@github.com/org/repo.git", "https://github.com/org/repo.git"},
		{"git@myhost.local:repo.git", "https://myhost.local/repo.git"},
	}

	for _, tt := range tests {
		got := normaliseHTTPS(tt.url)
		if got != tt.want {
			t.Errorf("normaliseHTTPS(%q) = %q; want %q", tt.url, got, tt.want)
		}
	}
}

func TestResolveAuth(t *testing.T) {
	token := "test-token"

	// Test GitHub with bearer method
	auth := resolveAuth("https://github.com/org/repo.git", "bearer", token)
	if _, ok := auth.(*gogithttp.BasicAuth); !ok {
		t.Errorf("resolveAuth(github, bearer) should return BasicAuth for GitHub compatibility")
	}

	// Test GitHub with oauth method
	auth = resolveAuth("https://github.com/org/repo.git", "oauth", token)
	if _, ok := auth.(*gogithttp.BasicAuth); !ok {
		t.Errorf("resolveAuth(github, oauth) should return BasicAuth for GitHub compatibility")
	}

	// Test Azure DevOps with bearer method
	auth = resolveAuth("https://dev.azure.com/org/proj/_git/repo", "bearer", token)
	if _, ok := auth.(*gogithttp.TokenAuth); !ok {
		t.Errorf("resolveAuth(azure, bearer) should return TokenAuth")
	}

	// Test GitLab with oauth method
	auth = resolveAuth("https://gitlab.com/group/repo.git", "oauth", token)
	if _, ok := auth.(*gogithttp.TokenAuth); !ok {
		t.Errorf("resolveAuth(gitlab, oauth) should return TokenAuth")
	}

	// Test PAT method
	auth = resolveAuth("https://github.com/org/repo.git", "pat", token)
	if ba, ok := auth.(*gogithttp.BasicAuth); !ok || ba.Username != "x-access-token" {
		t.Errorf("resolveAuth(pat) should return BasicAuth with x-access-token")
	}
}
