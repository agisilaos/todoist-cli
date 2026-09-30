package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/agisilaos/todoist-cli/internal/authorization"
)

const (
	defaultOAuthAuthorizeURL = "https://app.todoist.com/oauth/authorize"
	defaultOAuthTokenURL     = "https://api.todoist.com/oauth/access_token"
	defaultOAuthListenAddr   = "127.0.0.1:8765"
)

type oauthToken struct {
	AccessToken   string
	Authorization authorization.Metadata
}

type oauthConfig struct {
	RequestTimeout time.Duration
	ReadOnly       bool
	ClientID       string
	AuthorizeURL   string
	TokenURL       string
	DeviceURL      string
	RedirectURI    string
	ListenAddr     string
	NoBrowser      bool
	callback       *oauthCallback
}

// OAuth failures concern the proposed grant, never the authorization evidence of
// the credential it would replace. Messages and causes contain no provider data.
type oauthError struct {
	Code    string
	Message string
	cause   error
}

func (e *oauthError) Error() string { return e.Message }
func (e *oauthError) Unwrap() error { return e.cause }

func oauthFailure(code, message string) error {
	return &CodeError{Code: exitAuth, Err: &oauthError{Code: code, Message: message}}
}

func oauthContextError(cause error) error {
	code, message := "OAUTH_CANCELLED", "OAuth login cancelled. Nothing was saved; existing credentials are unchanged."
	if errors.Is(cause, context.DeadlineExceeded) {
		code, message = "OAUTH_TIMEOUT", "OAuth login deadline exceeded. Retry login; nothing was saved and existing credentials are unchanged."
	}
	return &CodeError{Code: exitError, Err: &oauthError{Code: code, Message: message, cause: cause}}
}

func buildOAuthConfig(clientID, authorizeURL, tokenURL, deviceURL, redirectURI, listenAddr string, noBrowser bool) (oauthConfig, error) {
	if clientID == "" {
		clientID = strings.TrimSpace(os.Getenv("TODOIST_OAUTH_CLIENT_ID"))
	}
	if clientID == "" {
		return oauthConfig{}, errors.New("missing OAuth client id; set --client-id or TODOIST_OAUTH_CLIENT_ID to your public PKCE client ID or hosted client metadata URL. This CLI has no maintainer-owned registered client. Configure the client's exact loopback redirect before login; confidential clients requiring a client secret are unsupported. See https://developer.todoist.com/api/v1/#tag/Authorization/OAuth-Client-ID-Metadata-Document or use manual auth login")
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
		deviceURL = strings.TrimSpace(os.Getenv("TODOIST_OAUTH_DEVICE_URL"))
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
	cfg := oauthConfig{
		ClientID:     clientID,
		AuthorizeURL: authorizeURL,
		TokenURL:     tokenURL,
		DeviceURL:    deviceURL,
		RedirectURI:  redirectURI,
		ListenAddr:   listenAddr,
		NoBrowser:    noBrowser,
	}
	if _, err := parseOAuthEndpoint(cfg.AuthorizeURL); err != nil {
		return oauthConfig{}, errors.New("invalid OAuth authorize URL; use HTTPS or a loopback HTTP test provider, without user credentials or a fragment")
	}
	if _, err := parseOAuthEndpoint(cfg.TokenURL); err != nil {
		return oauthConfig{}, errors.New("invalid OAuth token URL; use HTTPS or a loopback HTTP test provider, without user credentials or a fragment")
	}
	if err := validateOAuthCallback(cfg); err != nil {
		return oauthConfig{}, err
	}
	return cfg, nil
}

func loopbackOAuthHost(host string) bool {
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}

func parseOAuthEndpoint(value string) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil || u == nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && (u.Scheme != "http" || !loopbackOAuthHost(u.Hostname()))) {
		return nil, errors.New("invalid OAuth endpoint")
	}
	return u, nil
}

func validateOAuthCallback(cfg oauthConfig) error {
	host, port, err := net.SplitHostPort(cfg.ListenAddr)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || !loopbackOAuthHost(host) || portErr != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("--oauth-listen must be a loopback host with a fixed port between 1 and 65535")
	}
	u, err := url.Parse(cfg.RedirectURI)
	if err != nil || u == nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !loopbackOAuthHost(u.Hostname()) || !strings.EqualFold(u.Hostname(), host) || u.Port() != port || u.Path == "" {
		return errors.New("--oauth-redirect-uri must be an HTTP loopback URL with a callback path and the same host and port as --oauth-listen; register this exact redirect with your public client")
	}
	return nil
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
	u, err := parseOAuthEndpoint(cfg.AuthorizeURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", cfg.ClientID)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(authorization.RequestedScopes(cfg.ReadOnly), ","))
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

type oauthCallbackResult struct {
	code string
	err  error
}

type oauthCallback struct {
	server *http.Server
	result chan oauthCallbackResult
}

func (c *oauthCallback) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if c.server.Shutdown(ctx) != nil {
		_ = c.server.Close()
	}
}

// Start serving before publishing the URL, so even an immediate browser callback
// reaches a ready listener. Host aliases must match exactly; no DNS guessing.
func prepareOAuthCallback(cfg oauthConfig, expectedState string) (oauthConfig, error) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultOAuthListenAddr
	}
	if cfg.RedirectURI == "" {
		cfg.RedirectURI = "http://" + cfg.ListenAddr + "/callback"
	}
	if err := validateOAuthCallback(cfg); err != nil {
		return oauthConfig{}, &CodeError{Code: exitUsage, Err: err}
	}
	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return oauthConfig{}, oauthFailure("OAUTH_CALLBACK_INVALID", "Could not listen for the OAuth callback. Choose a free loopback port and register its exact redirect with your public client. Nothing was saved; existing credentials are unchanged.")
	}
	callback := &oauthCallback{result: make(chan oauthCallbackResult, 1)}
	var completed sync.Once
	publish := func(result oauthCallbackResult) bool {
		accepted := false
		completed.Do(func() {
			callback.result <- result
			accepted = true
		})
		return accepted
	}
	redirect, _ := url.Parse(cfg.RedirectURI)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != redirect.Path {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "OAuth callback requires GET", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		if len(q["state"]) != 1 || expectedState == "" || q.Get("state") != expectedState {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			return
		}
		result := oauthCallbackResult{code: q.Get("code")}
		if q.Get("error") != "" {
			result = oauthCallbackResult{err: oauthFailure("OAUTH_AUTHORIZATION_DENIED", "OAuth authorization was rejected or cancelled by the provider. Nothing was saved; existing credentials are unchanged.")}
		} else if len(q["code"]) != 1 || result.code == "" {
			result.err = oauthFailure("OAUTH_CALLBACK_INVALID", "OAuth callback did not contain a usable authorization code. Retry login; nothing was saved and existing credentials are unchanged.")
		}
		if !publish(result) {
			http.Error(w, "OAuth callback already received", http.StatusConflict)
			return
		}
		if result.err != nil {
			http.Error(w, "OAuth authorization could not complete. Return to the terminal.", http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, "OAuth response received. Return to the terminal to check whether login completed.")
	})
	callback.server = &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := callback.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			publish(oauthCallbackResult{err: oauthFailure("OAUTH_CALLBACK_INVALID", "OAuth callback server stopped. Retry login; nothing was saved and existing credentials are unchanged.")})
		}
	}()
	cfg.callback = callback
	return cfg, nil
}

func waitForOAuthCode(ctx context.Context, cfg oauthConfig, expectedState string, timeout time.Duration) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", oauthContextError(err)
	}
	if cfg.callback == nil {
		var err error
		cfg, err = prepareOAuthCallback(cfg, expectedState)
		if err != nil {
			return "", err
		}
		defer cfg.callback.close()
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case <-waitCtx.Done():
		return "", oauthContextError(waitCtx.Err())
	case result := <-cfg.callback.result:
		if err := waitCtx.Err(); err != nil {
			return "", oauthContextError(err)
		}
		return result.code, result.err
	}
}

// Never follow redirects carrying an authorization code, verifier or device
// credential, and never include endpoint URLs or response bodies in errors.
func requestOAuth(ctx context.Context, endpoint string, form url.Values, timeout time.Duration, action string) ([]byte, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, oauthContextError(err)
	}
	if _, err := parseOAuthEndpoint(endpoint); err != nil {
		return nil, 0, oauthFailure("OAUTH_EXCHANGE_FAILED", "Invalid OAuth provider endpoint. Use HTTPS or a loopback HTTP test provider; nothing was saved and existing credentials are unchanged.")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, oauthFailure("OAUTH_EXCHANGE_FAILED", "Could not prepare the OAuth request. Nothing was saved; existing credentials are unchanged.")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			cause := context.Canceled
			if errors.Is(err, context.DeadlineExceeded) {
				cause = context.DeadlineExceeded
			}
			return nil, 0, oauthContextError(cause)
		}
		return nil, 0, oauthFailure("OAUTH_EXCHANGE_FAILED", "OAuth "+action+" request failed. Check your connection and configured provider endpoint. Nothing was saved; existing credentials are unchanged.")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024+1))
	if err != nil || len(data) > 8*1024 {
		if ctx.Err() != nil {
			return nil, 0, oauthContextError(ctx.Err())
		}
		return nil, 0, oauthFailure("OAUTH_EXCHANGE_FAILED", "Could not read a bounded OAuth response. Nothing was saved; existing credentials are unchanged.")
	}
	return data, resp.StatusCode, nil
}

type oauthTokenResponse struct {
	AccessToken  string          `json:"access_token"`
	Scope        json.RawMessage `json:"scope"`
	RefreshToken json.RawMessage `json:"refresh_token"`
	ExpiresIn    json.RawMessage `json:"expires_in"`
	Error        string          `json:"error"`
}

func validateOAuthLifecycle(payload oauthTokenResponse) error {
	unsupported := func() error {
		return oauthFailure("OAUTH_LIFECYCLE_UNSUPPORTED", "OAuth returned a refresh-bearing grant or an unsupported token lifetime that this CLI cannot safely retain. Token refresh is not implemented. Use manual auth login or a verified long-lived public client grant. Nothing was saved; existing credentials are unchanged.")
	}
	if len(payload.RefreshToken) > 0 && string(payload.RefreshToken) != "null" {
		var refresh string
		if json.Unmarshal(payload.RefreshToken, &refresh) != nil || refresh != "" {
			return unsupported()
		}
	}
	if len(payload.ExpiresIn) > 0 {
		var expires int64
		// Todoist documents exactly this legacy compatibility value. No other
		// finite lifetime is assumed safe without refresh/expiry persistence.
		if string(payload.ExpiresIn) == "null" || json.Unmarshal(payload.ExpiresIn, &expires) != nil || expires != 315360000 {
			return unsupported()
		}
	}
	return nil
}

func tokenFromOAuthResponse(payload oauthTokenResponse, readOnly bool, origin string) (oauthToken, error) {
	if payload.Error != "" || payload.AccessToken == "" || strings.ContainsFunc(payload.AccessToken, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return oauthToken{}, oauthFailure("OAUTH_EXCHANGE_FAILED", "OAuth token exchange returned no usable access token. Nothing was saved; existing credentials are unchanged.")
	}
	if err := validateOAuthLifecycle(payload); err != nil {
		return oauthToken{}, err
	}
	metadata, err := authorization.OAuthMetadata(readOnly, origin, payload.Scope)
	if err != nil {
		return oauthToken{}, err
	}
	return oauthToken{AccessToken: payload.AccessToken, Authorization: metadata}, nil
}

func exchangeOAuthToken(ctx context.Context, cfg oauthConfig, code, codeVerifier string) (oauthToken, error) {
	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", codeVerifier)
	form.Set("redirect_uri", cfg.RedirectURI)
	data, status, err := requestOAuth(ctx, cfg.TokenURL, form, cfg.RequestTimeout, "token exchange")
	if err != nil {
		return oauthToken{}, err
	}
	if status < 200 || status >= 300 {
		return oauthToken{}, oauthFailure("OAUTH_EXCHANGE_FAILED", fmt.Sprintf("oauth token exchange failed: status %d. Check that the public PKCE client and exact redirect are configured; confidential clients requiring a secret are unsupported. Nothing was saved; existing credentials are unchanged.", status))
	}
	var payload oauthTokenResponse
	if json.Unmarshal(data, &payload) != nil {
		return oauthToken{}, oauthFailure("OAUTH_EXCHANGE_FAILED", "Could not decode the OAuth token response. Nothing was saved; existing credentials are unchanged.")
	}
	return tokenFromOAuthResponse(payload, cfg.ReadOnly, "oauth-pkce")
}

func startOAuthDeviceFlow(ctx context.Context, cfg oauthConfig) (deviceCode, userCode, verifyURL, verifyURLComplete string, intervalSec int, expiresInSec int, err error) {
	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("scope", strings.Join(authorization.RequestedScopes(cfg.ReadOnly), ","))
	data, status, err := requestOAuth(ctx, cfg.DeviceURL, form, cfg.RequestTimeout, "device authorization")
	if err != nil {
		return "", "", "", "", 0, 0, err
	}
	failure := func(message string) (string, string, string, string, int, int, error) {
		return "", "", "", "", 0, 0, oauthFailure("OAUTH_EXCHANGE_FAILED", message+" Nothing was saved; existing credentials are unchanged.")
	}
	if status < 200 || status >= 300 {
		return failure(fmt.Sprintf("OAuth device code request failed: status %d.", status))
	}
	var payload struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		Interval                int    `json:"interval"`
		ExpiresIn               int    `json:"expires_in"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return failure("Could not decode the OAuth device response.")
	}
	if payload.DeviceCode == "" || payload.UserCode == "" || len(payload.UserCode) > 128 || strings.ContainsFunc(payload.UserCode, unicode.IsControl) {
		return failure("OAuth device response is missing usable required fields.")
	}
	if _, err := parseOAuthEndpoint(payload.VerificationURI); err != nil {
		return failure("OAuth device response contains an invalid verification URL.")
	}
	if payload.VerificationURIComplete != "" {
		if _, err := parseOAuthEndpoint(payload.VerificationURIComplete); err != nil {
			return failure("OAuth device response contains an invalid complete verification URL.")
		}
	}
	if payload.Interval <= 0 {
		payload.Interval = 5
	}
	if payload.ExpiresIn <= 0 {
		payload.ExpiresIn = 600
	}
	return payload.DeviceCode, payload.UserCode, payload.VerificationURI, payload.VerificationURIComplete, payload.Interval, payload.ExpiresIn, nil
}

func pollOAuthDeviceToken(ctx context.Context, cfg oauthConfig, deviceCode string, intervalSec, expiresInSec int) (oauthToken, error) {
	if intervalSec <= 0 {
		intervalSec = 5
	}
	if expiresInSec <= 0 {
		expiresInSec = 600
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(expiresInSec)*time.Second)
	defer cancel()
	requestTimeout := cfg.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = 10 * time.Second
	}
	for {
		if err := ctx.Err(); err != nil {
			return oauthToken{}, oauthContextError(err)
		}
		form := url.Values{}
		form.Set("client_id", cfg.ClientID)
		form.Set("device_code", deviceCode)
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		data, status, err := requestOAuth(ctx, cfg.TokenURL, form, requestTimeout, "device polling")
		if err != nil {
			return oauthToken{}, err
		}
		var payload oauthTokenResponse
		if json.Unmarshal(data, &payload) != nil {
			return oauthToken{}, oauthFailure("OAUTH_EXCHANGE_FAILED", "Could not decode the OAuth device token response. Nothing was saved; existing credentials are unchanged.")
		}
		if status >= 200 && status < 300 && payload.AccessToken != "" {
			return tokenFromOAuthResponse(payload, cfg.ReadOnly, "oauth-device")
		}
		if status == http.StatusBadRequest {
			switch payload.Error {
			case "authorization_pending", "slow_down":
				if payload.Error == "slow_down" {
					intervalSec += 5
				}
				if err := waitForOAuthPollFn(ctx, time.Duration(intervalSec)*time.Second); err != nil {
					return oauthToken{}, oauthContextError(err)
				}
				continue
			case "access_denied":
				return oauthToken{}, oauthFailure("OAUTH_AUTHORIZATION_DENIED", "OAuth device authorization was denied. Nothing was saved; existing credentials are unchanged.")
			case "expired_token":
				return oauthToken{}, oauthFailure("OAUTH_AUTHORIZATION_DENIED", "OAuth device code expired. Retry login; nothing was saved and existing credentials are unchanged.")
			}
		}
		return oauthToken{}, oauthFailure("OAUTH_EXCHANGE_FAILED", fmt.Sprintf("OAuth device token polling failed: status %d. Nothing was saved; existing credentials are unchanged.", status))
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
