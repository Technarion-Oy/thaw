package gitrepo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"thaw/internal/config"

	"github.com/pkg/browser"
)

// The secret is scrambled before compile.
// (e.g., XORing the secret with the xorKey)
// This is the industry standard for storing oauth keys.
// For reference check src/shared/GitHub/GitHubConstants.cs
// in https://github.com/git-ecosystem/git-credential-manager
const (
	// #nosec G101 // Justification: OAuth2 public client application 'secrets' are required and permitted to be public
	internalRoutingID = "2c0c730e63161c564c7f0d47284f54600d6e2e254b755b60425e54220602555a5f4b427201760d73"
	sessionPadding    = "N4EmTpxauJotJx0VhYOFzLnSs8gG65bcnyvF8"
)

func getScrambledClientSecret() string {
	if internalRoutingID == "PLACEHOLDER_SCRAMBLED_HEX" || sessionPadding == "PLACEHOLDER_XOR_KEY" {
		return ""
	}

	scrambled, err := hex.DecodeString(internalRoutingID)
	if err != nil {
		return ""
	}

	unscrambled := make([]byte, len(scrambled))
	for i := 0; i < len(scrambled); i++ {
		unscrambled[i] = scrambled[i] ^ sessionPadding[i%len(sessionPadding)]
	}
	return string(unscrambled)
}

type OAuthConfig struct {
	ProviderName string
	AuthURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       string
}

func GetProviderConfig(provider string) OAuthConfig {
	cfg, err := config.Load()
	if err != nil {
		// Fallback to empty if we can't load config
		cfg = &config.AppConfig{}
		cfg.OAuth.GithubClientID = "Ov23liqwbGA6HHQ1za1a"
	}

	switch strings.ToLower(provider) {
	case "github":
		secret := getScrambledClientSecret()
		if secret == "" {
			secret = cfg.OAuth.GithubClientSecret
		}

		// #nosec G101 // Justification: OAuth2 public client application 'secrets' are required and permitted to be public
		return OAuthConfig{
			ProviderName: "GitHub",
			AuthURL:      "https://github.com/login/oauth/authorize",
			TokenURL:     "https://github.com/login/oauth/access_token",
			ClientID:     cfg.OAuth.GithubClientID,
			ClientSecret: secret,
			Scopes:       "repo",
		}
	case "gitlab":
		// #nosec G101 // Justification: OAuth2 public client application 'secrets' are required and permitted to be public
		return OAuthConfig{
			ProviderName: "GitLab",
			AuthURL:      "https://gitlab.com/oauth/authorize",
			TokenURL:     "https://gitlab.com/oauth/token",
			ClientID:     cfg.OAuth.GitlabClientID,
			ClientSecret: cfg.OAuth.GitlabClientSecret,
			Scopes:       "api",
		}
	default:
		return OAuthConfig{}
	}
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func PerformOAuthFlow(ctx context.Context, provider string) (string, error) {
	cfg := GetProviderConfig(provider)
	if cfg.ClientID == "" {
		return "", fmt.Errorf("unsupported or unconfigured provider: %s", provider)
	}

	state := generateState()
	codeChan := make(chan string)
	errChan := make(chan error)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              "127.0.0.1:3456",
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,  // Time allowed to read headers
		ReadTimeout:       10 * time.Second, // Maximum duration for reading the entire request
		WriteTimeout:      10 * time.Second, // Maximum duration before timing out writes of the response
		IdleTimeout:       30 * time.Second, // Maximum amount of time to wait for the next request when keep-alives are enabled
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		queryState := r.URL.Query().Get("state")
		if queryState != state {
			http.Error(w, "State mismatch", http.StatusBadRequest)
			errChan <- fmt.Errorf("state mismatch in oauth callback")
			return
		}

		errStr := r.URL.Query().Get("error")
		if errStr != "" {
			http.Error(w, "OAuth Error: "+errStr, http.StatusBadRequest)
			errChan <- fmt.Errorf("oauth error: %s", errStr)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No code provided", http.StatusBadRequest)
			errChan <- fmt.Errorf("no code provided in callback")
			return
		}

		w.Header().Set("Content-Type", "text/html")
		if _, err := fmt.Fprintf(w, "<html><body><h2>Authentication successful!</h2><p>You can close this tab and return to Thaw.</p><script>window.close()</script></body></html>"); err != nil {
			fmt.Printf("failed to write HTML response: %v", err)
		}
		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("local server error: %w", err)
		}
	}()
	defer func() {
		if err := server.Shutdown(context.Background()); err != nil {
			fmt.Printf("failed to shutdown server: %v", err)
		}
	}()
	redirectURI := "http://127.0.0.1:3456/callback"

	authURL, err := url.Parse(cfg.AuthURL)
	if err != nil {
		return "", fmt.Errorf("invalid auth url: %w", err)
	}
	q := authURL.Query()
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", cfg.Scopes)
	q.Set("state", state)
	authURL.RawQuery = q.Encode()

	if err := browser.OpenURL(authURL.String()); err != nil {
		return "", fmt.Errorf("failed to open browser: %w", err)
	}

	var code string
	select {
	case code = <-codeChan:
	case err := <-errChan:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Exchange code for token
	token, err := exchangeCodeForToken(cfg, code, redirectURI)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code for token: %w", err)
	}

	return token, nil
}

func exchangeCodeForToken(cfg OAuthConfig, code, redirectURI string) (string, error) {
	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
	data.Set("client_secret", cfg.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequest("POST", cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if token, ok := result["access_token"].(string); ok {
		return token, nil
	}

	return "", fmt.Errorf("no access_token in response: %s", string(body))
}
