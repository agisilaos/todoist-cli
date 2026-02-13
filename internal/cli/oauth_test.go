package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBuildOAuthConfigRequiresClientID(t *testing.T) {
	t.Setenv("TODOIST_OAUTH_CLIENT_ID", "")

	_, err := buildOAuthConfig("", "", "", "", "", "", false)
	if err == nil {
		t.Fatalf("expected missing client id error")
	}
	if !strings.Contains(err.Error(), "missing OAuth client id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildOAuthConfigDefaultsAndEnv(t *testing.T) {
	t.Setenv("TODOIST_OAUTH_CLIENT_ID", "env-client")
	t.Setenv("TODOIST_OAUTH_AUTHORIZE_URL", "https://auth.example.com/authorize")
	t.Setenv("TODOIST_OAUTH_TOKEN_URL", "https://auth.example.com/token")
	t.Setenv("TODOIST_OAUTH_DEVICE_URL", "https://auth.example.com/device")
	t.Setenv("TODOIST_OAUTH_LISTEN", "127.0.0.1:9999")

	cfg, err := buildOAuthConfig("", "", "", "", "", "", true)
	if err != nil {
		t.Fatalf("buildOAuthConfig: %v", err)
	}
	if cfg.ClientID != "env-client" {
		t.Fatalf("expected env client id, got %q", cfg.ClientID)
	}
	if cfg.AuthorizeURL != "https://auth.example.com/authorize" {
		t.Fatalf("unexpected authorize url: %q", cfg.AuthorizeURL)
	}
	if cfg.TokenURL != "https://auth.example.com/token" {
		t.Fatalf("unexpected token url: %q", cfg.TokenURL)
	}
	if cfg.DeviceURL != "https://auth.example.com/device" {
		t.Fatalf("unexpected device url: %q", cfg.DeviceURL)
	}
	if cfg.ListenAddr != "127.0.0.1:9999" {
		t.Fatalf("unexpected listen addr: %q", cfg.ListenAddr)
	}
	if cfg.RedirectURI != "http://127.0.0.1:9999/callback" {
		t.Fatalf("unexpected redirect uri: %q", cfg.RedirectURI)
	}
	if !cfg.NoBrowser {
		t.Fatalf("expected no-browser true")
	}
}

func TestStartOAuthDeviceFlowSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		data, _ := io.ReadAll(r.Body)
		values, err := url.ParseQuery(string(data))
		if err != nil {
			t.Fatalf("parse body: %v", err)
		}
		if values.Get("client_id") != "client-1" {
			t.Fatalf("unexpected client_id: %q", values.Get("client_id"))
		}
		_, _ = w.Write([]byte(`{"device_code":"dev-1","user_code":"ABCD","verification_uri":"https://verify","verification_uri_complete":"https://verify?user_code=ABCD","interval":2,"expires_in":300}`))
	}))
	defer ts.Close()

	cfg := oauthConfig{ClientID: "client-1", DeviceURL: ts.URL}
	deviceCode, userCode, verifyURL, verifyURLComplete, intervalSec, expiresInSec, err := startOAuthDeviceFlow(context.Background(), cfg)
	if err != nil {
		t.Fatalf("startOAuthDeviceFlow: %v", err)
	}
	if deviceCode != "dev-1" || userCode != "ABCD" || verifyURL != "https://verify" || verifyURLComplete == "" || intervalSec != 2 || expiresInSec != 300 {
		t.Fatalf("unexpected device flow response: %q %q %q %q %d %d", deviceCode, userCode, verifyURL, verifyURLComplete, intervalSec, expiresInSec)
	}
}

func TestPollOAuthDeviceTokenSuccessAfterPending(t *testing.T) {
	prevWait := waitForOAuthPollFn
	waitForOAuthPollFn = func(ctx context.Context, delay time.Duration) error { return nil }
	defer func() { waitForOAuthPollFn = prevWait }()

	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"token-device-1"}`))
	}))
	defer ts.Close()

	cfg := oauthConfig{ClientID: "client-1", TokenURL: ts.URL}
	token, err := pollOAuthDeviceToken(context.Background(), cfg, "dev-1", 1, 10)
	if err != nil {
		t.Fatalf("pollOAuthDeviceToken: %v", err)
	}
	if token != "token-device-1" {
		t.Fatalf("unexpected token: %q", token)
	}
}

func TestOAuthCodeChallenge(t *testing.T) {
	got := oauthCodeChallenge("abc")
	want := "ungWv48Bz-pBQUDeXa4iI7ADYaOWF3qctBD_YfIAFa0"
	if got != want {
		t.Fatalf("unexpected code challenge: got %q want %q", got, want)
	}
}

func TestBuildOAuthAuthorizationURL(t *testing.T) {
	cfg := oauthConfig{
		ClientID:     "cid-1",
		AuthorizeURL: "https://todoist.example/oauth/authorize",
		RedirectURI:  "http://127.0.0.1:8765/callback",
	}
	u, err := buildOAuthAuthorizationURL(cfg, "challenge-1", "state-1")
	if err != nil {
		t.Fatalf("buildOAuthAuthorizationURL: %v", err)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	q := parsed.Query()
	if q.Get("client_id") != "cid-1" {
		t.Fatalf("unexpected client_id: %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != cfg.RedirectURI {
		t.Fatalf("unexpected redirect_uri: %q", q.Get("redirect_uri"))
	}
	if q.Get("state") != "state-1" {
		t.Fatalf("unexpected state: %q", q.Get("state"))
	}
	if q.Get("code_challenge") != "challenge-1" {
		t.Fatalf("unexpected code_challenge: %q", q.Get("code_challenge"))
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Fatalf("unexpected code_challenge_method: %q", q.Get("code_challenge_method"))
	}
}

func TestExchangeOAuthTokenSuccess(t *testing.T) {
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Fatalf("unexpected content type: %q", ct)
		}
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token-123"}`))
	}))
	defer ts.Close()

	cfg := oauthConfig{
		ClientID:    "client-1",
		TokenURL:    ts.URL,
		RedirectURI: "http://127.0.0.1:8765/callback",
	}
	token, err := exchangeOAuthToken(context.Background(), cfg, "code-1", "verifier-1")
	if err != nil {
		t.Fatalf("exchangeOAuthToken: %v", err)
	}
	if token != "token-123" {
		t.Fatalf("unexpected token: %q", token)
	}
	values, err := url.ParseQuery(gotBody)
	if err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if values.Get("client_id") != "client-1" || values.Get("code") != "code-1" || values.Get("code_verifier") != "verifier-1" {
		t.Fatalf("unexpected form body: %q", gotBody)
	}
}

func TestExchangeOAuthTokenErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid code", http.StatusBadRequest)
	}))
	defer ts.Close()

	cfg := oauthConfig{
		ClientID:    "client-1",
		TokenURL:    ts.URL,
		RedirectURI: "http://127.0.0.1:8765/callback",
	}
	_, err := exchangeOAuthToken(context.Background(), cfg, "bad-code", "verifier-1")
	if err == nil {
		t.Fatalf("expected exchange error")
	}
	if !strings.Contains(err.Error(), "oauth token exchange failed: status 400") {
		t.Fatalf("unexpected error: %v", err)
	}
}
