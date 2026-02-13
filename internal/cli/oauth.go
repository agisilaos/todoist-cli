package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	defaultOAuthAuthorizeURL = "https://todoist.com/oauth/authorize"
	defaultOAuthTokenURL     = "https://todoist.com/oauth/access_token"
	defaultOAuthDeviceURL    = "https://todoist.com/oauth/device/code"
	defaultOAuthListenAddr   = "127.0.0.1:8765"
)

type oauthConfig struct {
	ClientID     string
	AuthorizeURL string
	TokenURL     string
	DeviceURL    string
	RedirectURI  string
	ListenAddr   string
	NoBrowser    bool
}

func buildOAuthConfig(clientID, authorizeURL, tokenURL, deviceURL, redirectURI, listenAddr string, noBrowser bool) (oauthConfig, error) {
	if clientID == "" {
		clientID = strings.TrimSpace(os.Getenv("TODOIST_OAUTH_CLIENT_ID"))
	}
	if clientID == "" {
		return oauthConfig{}, fmt.Errorf("missing OAuth client id; set --client-id or TODOIST_OAUTH_CLIENT_ID")
	}
	if authorizeURL == "" {
		if env := strings.TrimSpace(os.Getenv("TODOIST_OAUTH_AUTHORIZE_URL")); env != "" {
			authorizeURL = env
		} else {
			authorizeURL = defaultOAuthAuthorizeURL
		}
	}
	if tokenURL == "" {
		if env := strings.TrimSpace(os.Getenv("TODOIST_OAUTH_TOKEN_URL")); env != "" {
			tokenURL = env
		} else {
			tokenURL = defaultOAuthTokenURL
		}
	}
	if deviceURL == "" {
		if env := strings.TrimSpace(os.Getenv("TODOIST_OAUTH_DEVICE_URL")); env != "" {
			deviceURL = env
		} else {
			deviceURL = defaultOAuthDeviceURL
		}
	}
	if listenAddr == "" {
		if env := strings.TrimSpace(os.Getenv("TODOIST_OAUTH_LISTEN")); env != "" {
			listenAddr = env
		} else {
			listenAddr = defaultOAuthListenAddr
		}
	}
	if redirectURI == "" {
		redirectURI = "http://" + listenAddr + "/callback"
	}
	return oauthConfig{
		ClientID:     clientID,
		AuthorizeURL: authorizeURL,
		TokenURL:     tokenURL,
		DeviceURL:    deviceURL,
		RedirectURI:  redirectURI,
		ListenAddr:   listenAddr,
		NoBrowser:    noBrowser,
	}, nil
}

func generateOAuthRandom(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func oauthCodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func buildOAuthAuthorizationURL(cfg oauthConfig, codeChallenge, state string) (string, error) {
	u, err := url.Parse(cfg.AuthorizeURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", cfg.ClientID)
	q.Set("scope", "data:read_write,data:delete,project:delete")
	q.Set("state", state)
	q.Set("redirect_uri", cfg.RedirectURI)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func openOAuthBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func waitForOAuthCode(ctx context.Context, cfg oauthConfig, expectedState string, timeout time.Duration) (string, error) {
	parsedRedirect, err := url.Parse(cfg.RedirectURI)
	if err != nil {
		return "", fmt.Errorf("invalid redirect URI: %w", err)
	}
	callbackPath := parsedRedirect.Path
	if callbackPath == "" {
		callbackPath = "/callback"
	}

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return "", fmt.Errorf("listen OAuth callback: %w", err)
	}
	defer ln.Close()

	resultCh := make(chan struct {
		code string
		err  error
	}, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errMsg := q.Get("error"); errMsg != "" {
			http.Error(w, "OAuth failed: "+errMsg, http.StatusBadRequest)
			resultCh <- struct {
				code string
				err  error
			}{"", fmt.Errorf("oauth authorization failed: %s", errMsg)}
			return
		}
		state := q.Get("state")
		if state == "" || state != expectedState {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			resultCh <- struct {
				code string
				err  error
			}{"", fmt.Errorf("invalid oauth state")}
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "Missing OAuth code", http.StatusBadRequest)
			resultCh <- struct {
				code string
				err  error
			}{"", fmt.Errorf("missing oauth code in callback")}
			return
		}
		_, _ = io.WriteString(w, "Todoist CLI authorization successful. You can close this tab.")
		resultCh <- struct {
			code string
			err  error
		}{code, nil}
	})

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(ln)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case <-waitCtx.Done():
		if waitCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("oauth callback timed out")
		}
		return "", waitCtx.Err()
	case result := <-resultCh:
		return result.code, result.err
	}
}

func exchangeOAuthToken(ctx context.Context, cfg oauthConfig, code, codeVerifier string) (string, error) {
	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("code", code)
	form.Set("code_verifier", codeVerifier)
	form.Set("redirect_uri", cfg.RedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("oauth token exchange failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("decode oauth token response: %w", err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", fmt.Errorf("oauth token exchange returned empty access_token")
	}
	return payload.AccessToken, nil
}

func startOAuthDeviceFlow(ctx context.Context, cfg oauthConfig) (deviceCode, userCode, verifyURL, verifyURLComplete string, intervalSec int, expiresInSec int, err error) {
	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("scope", "data:read_write,data:delete,project:delete")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.DeviceURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", "", "", 0, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", "", 0, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	if resp.StatusCode >= 400 {
		return "", "", "", "", 0, 0, fmt.Errorf("oauth device code request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var payload struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		Interval                int    `json:"interval"`
		ExpiresIn               int    `json:"expires_in"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", "", "", "", 0, 0, fmt.Errorf("decode oauth device code response: %w", err)
	}
	if strings.TrimSpace(payload.DeviceCode) == "" || strings.TrimSpace(payload.UserCode) == "" {
		return "", "", "", "", 0, 0, fmt.Errorf("oauth device code response missing required fields")
	}
	if payload.Interval <= 0 {
		payload.Interval = 5
	}
	if payload.ExpiresIn <= 0 {
		payload.ExpiresIn = 600
	}
	return payload.DeviceCode, payload.UserCode, payload.VerificationURI, payload.VerificationURIComplete, payload.Interval, payload.ExpiresIn, nil
}

func pollOAuthDeviceToken(ctx context.Context, cfg oauthConfig, deviceCode string, intervalSec, expiresInSec int) (string, error) {
	if intervalSec <= 0 {
		intervalSec = 5
	}
	if expiresInSec <= 0 {
		expiresInSec = 600
	}
	deadline := time.Now().Add(time.Duration(expiresInSec) * time.Second)

	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("oauth device flow timed out")
		}
		form := url.Values{}
		form.Set("client_id", cfg.ClientID)
		form.Set("device_code", deviceCode)
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		resp.Body.Close()

		var payload struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}
		_ = json.Unmarshal(data, &payload)

		if resp.StatusCode < 400 && strings.TrimSpace(payload.AccessToken) != "" {
			return payload.AccessToken, nil
		}

		switch payload.Error {
		case "authorization_pending":
			if err := waitForOAuthPollFn(ctx, time.Duration(intervalSec)*time.Second); err != nil {
				return "", err
			}
			continue
		case "slow_down":
			intervalSec += 5
			if err := waitForOAuthPollFn(ctx, time.Duration(intervalSec)*time.Second); err != nil {
				return "", err
			}
			continue
		case "access_denied":
			return "", fmt.Errorf("oauth device authorization denied")
		case "expired_token":
			return "", fmt.Errorf("oauth device code expired")
		}

		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("oauth device token polling failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
		}
		return "", fmt.Errorf("oauth device token polling failed: empty access_token")
	}
}

var waitForOAuthPollFn = waitForOAuthPoll

func waitForOAuthPoll(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
