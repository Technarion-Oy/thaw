package gitrepo

import (
	"testing"

	gogithttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

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
