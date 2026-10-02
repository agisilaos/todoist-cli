package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func scratchOAuthAddress(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func requireOAuthCode(t *testing.T, err error, code string, exit int) {
	t.Helper()
	var failure *oauthError
	if !errors.As(err, &failure) || failure.Code != code || toExitCode(err) != exit {
		t.Fatalf("expected %s exit %d, got %v", code, exit, err)
	}
}

func TestOAuthClientOverrideAndSetupErrors(t *testing.T) {
	t.Setenv("TODOIST_OAUTH_CLIENT_ID", "environment-client")
	cfg, err := buildOAuthConfig(&Context{}, "explicit-client", "", "", "", "", "", true)
	if err != nil || cfg.ClientID != "explicit-client" {
		t.Fatalf("explicit client did not override environment: %v", err)
	}
	t.Setenv("TODOIST_OAUTH_CLIENT_ID", "")
	_, err = buildOAuthConfig(&Context{}, "", "", "", "", "", "", true)
	for _, text := range []string{"public PKCE", "exact loopback redirect", "confidential clients", "manual auth login"} {
		if err == nil || !strings.Contains(err.Error(), text) {
			t.Fatalf("missing actionable setup guidance %q", text)
		}
	}
}

func TestOAuthCallbackConfigurationRequiresExactLoopbackAddress(t *testing.T) {
	for _, tc := range []struct{ listen, redirect string }{
		{"0.0.0.0:8765", "http://127.0.0.1:8765/callback"},
		{"example.com:8765", "http://example.com:8765/callback"},
		{"127.0.0.1:0", "http://127.0.0.1:0/callback"},
		{"127.0.0.1:8765", "http://localhost:8765/callback"},
		{"127.0.0.1:8765", "http://127.0.0.1:9876/callback"},
		{"127.0.0.1:8765", "https://127.0.0.1:8765/callback"},
		{"127.0.0.1:8765", "http://user:secret@127.0.0.1:8765/callback"},
		{"127.0.0.1:8765", "http://127.0.0.1:8765/callback?secret=value"},
	} {
		_, err := buildOAuthConfig(&Context{}, "client", "", "", "", tc.redirect, tc.listen, true)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("invalid callback accepted or unsafe error: %v", err)
		}
	}
	for _, tc := range []struct{ listen, redirect string }{
		{"127.0.0.1:8765", "http://127.0.0.1:8765/callback"},
		{"localhost:8765", "http://localhost:8765/callback"},
		{"[::1]:8765", "http://[::1]:8765/callback"},
	} {
		if err := validateOAuthCallback(oauthConfig{ListenAddr: tc.listen, RedirectURI: tc.redirect}); err != nil {
			t.Fatalf("matching loopback callback rejected: %v", err)
		}
	}
}

func TestOAuthDefaultDeviceFlowRequiresConfiguredProvider(t *testing.T) {
	t.Setenv("TODOIST_OAUTH_DEVICE_URL", "")
	err := authLogin(newAuthTestContext(t), []string{"--oauth-device", "--client-id", "client"})
	assertUsageErrorContains(t, err, "Todoist does not advertise OAuth device authorization")
}

func TestOAuthDeviceEndpointRequiresConfigurationWithoutRejectingExplicitURL(t *testing.T) {
	t.Setenv("TODOIST_OAUTH_DEVICE_URL", "")
	cfg, err := buildOAuthConfig(&Context{}, "client", "", "", "", "", "", true)
	if err != nil || cfg.DeviceURL != "" {
		t.Fatal("unconfigured device flow must not invent a provider endpoint")
	}
	const explicit = "https://todoist.com/oauth/device/code"
	cfg, err = buildOAuthConfig(&Context{}, "client", "", "", explicit, "", "", true)
	if err != nil || cfg.DeviceURL != explicit {
		t.Fatal("explicit device endpoint was not preserved")
	}
}

func TestOAuthCallbackReadyBeforeBrowserLaunch(t *testing.T) {
	ctx := newAuthTestContext(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code_verifier") == "" {
			t.Error("missing explicit PKCE token exchange fields")
		}
		fmt.Fprint(w, `{"access_token":"synthetic-new-token"}`)
	}))
	defer server.Close()
	previous := ctx.oauth.openBrowser
	defer func() { ctx.oauth.openBrowser = previous }()
	opened := false
	ctx.oauth.openBrowser = func(value string) error {
		opened = true
		u, _ := url.Parse(value)
		if u.Query().Get("response_type") != "code" {
			t.Error("missing explicit authorization code response type")
		}
		client := &http.Client{Timeout: time.Second}
		response, err := client.Get(u.Query().Get("redirect_uri") + "?state=" + url.QueryEscape(u.Query().Get("state")) + "&code=synthetic-code")
		if err != nil {
			return err
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Errorf("browser callback status %d", response.StatusCode)
		}
		return nil
	}
	address := scratchOAuthAddress(t)
	token, err := authOAuthLogin(ctx, oauthConfig{ClientID: "client", AuthorizeURL: server.URL + "/authorize", TokenURL: server.URL, ListenAddr: address, RedirectURI: "http://" + address + "/callback"})
	if err != nil || token.AccessToken != "synthetic-new-token" || !opened {
		t.Fatalf("ready callback failed: %v", err)
	}
}

func TestOAuthCallbackStatePrecedesErrorsAndRepeatedCallbacksDoNotBlock(t *testing.T) {
	address := scratchOAuthAddress(t)
	cfg, err := prepareOAuthCallback(oauthConfig{ListenAddr: address, RedirectURI: "http://" + address + "/callback"}, "expected-state")
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.callback.close()
	client := &http.Client{Timeout: time.Second}
	request := func(query string) int {
		response, err := client.Get(cfg.RedirectURI + query)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var data bytes.Buffer
		_, _ = data.ReadFrom(response.Body)
		if strings.Contains(data.String(), "synthetic-secret") {
			t.Fatal("callback echoed provider input")
		}
		return response.StatusCode
	}
	if request("?state=wrong&error=synthetic-secret") != 400 {
		t.Fatal("untrusted error callback accepted")
	}
	select {
	case <-cfg.callback.result:
		t.Fatal("untrusted callback ended authorization")
	default:
	}
	if request("?state=expected-state&code=synthetic-secret") != 200 || request("?state=expected-state&code=another") != 409 {
		t.Fatal("repeated callback was not bounded")
	}
	code, err := waitForOAuthCode(context.Background(), cfg, "expected-state", time.Second)
	if err != nil || code != "synthetic-secret" {
		t.Fatalf("valid callback not retained: %v", err)
	}
}

func TestOAuthExchangeErrorsNeverEchoBodiesOrFollowRedirects(t *testing.T) {
	var redirected bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected = true }))
	defer target.Close()
	for _, tc := range []struct {
		status int
		body   string
	}{
		{400, `{"error":"synthetic-secret","access_token":"synthetic-secret"}`},
		{200, `{"access_token":"synthetic-secret","scope":`},
		{307, `{"access_token":"synthetic-secret"}`},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", target.URL)
			w.WriteHeader(tc.status)
			fmt.Fprint(w, tc.body)
		}))
		_, err := exchangeOAuthToken(context.Background(), oauthConfig{ClientID: "client", TokenURL: server.URL}, "synthetic-secret", "synthetic-secret")
		server.Close()
		requireOAuthCode(t, err, "OAUTH_EXCHANGE_FAILED", exitAuth)
		if strings.Contains(err.Error(), "synthetic-secret") || redirected {
			t.Fatal("exchange exposed provider data or followed redirect")
		}
	}
}

func TestOAuthLifecycleAcceptanceAcrossProviders(t *testing.T) {
	for _, flow := range []string{"pkce", "device"} {
		for _, suffix := range []string{"", `,"expires_in":315360000`, `,"refresh_token":""`} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"access_token":"synthetic-token"`+suffix+`}`)
			}))
			cfg := oauthConfig{ClientID: "client", TokenURL: server.URL}
			var token oauthToken
			var err error
			if flow == "pkce" {
				token, err = exchangeOAuthToken(context.Background(), cfg, "code", "verifier")
			} else {
				token, err = pollOAuthDeviceToken(context.Background(), cfg, "device", 1, 10)
			}
			server.Close()
			if err != nil || token.AccessToken != "synthetic-token" {
				t.Fatalf("legacy %s grant rejected: %v", flow, err)
			}
		}
	}
}

func TestOAuthRejectedLifecyclePreservesAllCredentials(t *testing.T) {
	for _, flow := range []string{"pkce", "device"} {
		for _, suffix := range []string{`,"refresh_token":"synthetic-refresh-secret"`, `,"refresh_token":null`, `,"expires_in":3600`, `,"expires_in":0`, `,"expires_in":null`, `,"expires_in":"315360000"`, `,"expires_in":315360000.5`, `,"expires_in":315360001`} {
			t.Run(flow+suffix, func(t *testing.T) {
				ctx := newAuthTestContext(t)
				ctx.Mode = output.ModeJSON
				ctx.Global.NoInput = true
				for _, name := range []string{"default", "unrelated"} {
					if err := profileStore(ctx).Save(context.Background(), name, config.Credential{Token: "synthetic-existing-secret"}, "file"); err != nil {
						t.Fatal(err)
					}
				}
				path := config.CredentialsPathFromConfig(ctx.ConfigPath)
				before, _ := os.ReadFile(path)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/device" {
						fmt.Fprint(w, `{"device_code":"device","user_code":"USER","verification_uri":"https://example.invalid","expires_in":30}`)
						return
					}
					fmt.Fprint(w, `{"access_token":"synthetic-new-secret"`+suffix+`}`)
				}))
				defer server.Close()
				args := []string{"--client-id", "client", "--oauth-token-url", server.URL + "/token"}
				var callbackDone chan error
				if flow == "device" {
					args = append(args, "--oauth-device", "--oauth-device-url", server.URL+"/device")
				} else {
					address := scratchOAuthAddress(t)
					args = append(args, "--oauth", "--no-browser", "--oauth-listen", address, "--oauth-authorize-url", server.URL+"/authorize")
					writer := &oauthURLWriter{urls: make(chan string, 1)}
					ctx.Stderr = writer
					callbackDone = make(chan error, 1)
					go func() {
						value := <-writer.urls
						u, _ := url.Parse(value)
						response, err := (&http.Client{Timeout: time.Second}).Get(u.Query().Get("redirect_uri") + "?code=approved&state=" + url.QueryEscape(u.Query().Get("state")))
						if err == nil {
							response.Body.Close()
						}
						callbackDone <- err
					}()
				}
				err := authLogin(ctx, args)
				requireOAuthCode(t, err, "OAUTH_LIFECYCLE_UNSUPPORTED", exitAuth)
				if callbackDone != nil {
					if err := <-callbackDone; err != nil {
						t.Fatal(err)
					}
				}
				after, _ := os.ReadFile(path)
				if !bytes.Equal(before, after) || ctx.Stdout.(*bytes.Buffer).Len() != 0 || strings.Contains(err.Error(), "synthetic-") {
					t.Fatal("unsupported lifecycle changed credentials, emitted success or leaked data")
				}
			})
		}
	}
}

func TestOAuthCancellationBeforePersistenceOrExport(t *testing.T) {
	for _, printEnv := range []bool{false, true} {
		ctx := newAuthTestContext(t)
		operation, cancel := context.WithCancel(context.Background())
		ctx.OperationContext = operation
		if err := profileStore(ctx).Save(context.Background(), ctx.Profile, config.Credential{Token: "synthetic-existing-secret"}, "file"); err != nil {
			t.Fatal(err)
		}
		path := config.CredentialsPathFromConfig(ctx.ConfigPath)
		before, _ := os.ReadFile(path)
		restore := stubPerformOAuthLogin(ctx, func(*Context, oauthConfig) (oauthToken, error) {
			cancel()
			return oauthToken{AccessToken: "synthetic-new-secret", Authorization: authorization.ManualMetadata()}, nil
		})
		args := []string{"--oauth", "--client-id", "client"}
		if printEnv {
			args = append(args, "--print-env")
		}
		err := authLogin(ctx, args)
		restore()
		requireOAuthCode(t, err, "OAUTH_CANCELLED", exitError)
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) || ctx.Stdout.(*bytes.Buffer).Len() != 0 || !errors.Is(err, context.Canceled) {
			t.Fatal("cancelled OAuth replaced or exported a credential")
		}
	}
}

type oauthCancellationDisk struct {
	credentials.Disk
	cancel context.CancelFunc
}

func (d *oauthCancellationDisk) Lock(ctx context.Context, path string) (func(), error) {
	unlock, err := d.Disk.Lock(ctx, path)
	if err == nil && d.cancel != nil {
		d.cancel()
	}
	return unlock, err
}

func TestOAuthCancellationInsideSaveReportsSafeErrorAndPreservesCredential(t *testing.T) {
	ctx := newAuthTestContext(t)
	path := config.CredentialsPathFromConfig(ctx.ConfigPath)
	disk := &oauthCancellationDisk{}
	ctx.Credentials = credentials.New(path, nil, disk)
	if err := ctx.Credentials.Save(context.Background(), ctx.Profile, config.Credential{Token: "synthetic-existing"}, "file"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	operation, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx.OperationContext, disk.cancel = operation, cancel
	restore := stubPerformOAuthLogin(ctx, func(*Context, oauthConfig) (oauthToken, error) {
		return oauthToken{AccessToken: "synthetic-candidate", Authorization: authorization.ManualMetadata()}, nil
	})
	defer restore()
	err := authLogin(ctx, []string{"--oauth", "--client-id", "client"})
	requireOAuthCode(t, err, "OAUTH_CANCELLED", exitError)
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) || ctx.Stdout.(*bytes.Buffer).Len() != 0 {
		t.Fatal("storage cancellation replaced a credential or reported success")
	}
}

func TestOAuthCallbackAndDevicePollingRespectCancellation(t *testing.T) {
	operation, cancel := context.WithCancel(context.Background())
	ctx := newAuthTestContext(t)
	ctx.OperationContext = operation
	cancel()
	_, err := authOAuthLogin(ctx, oauthConfig{})
	requireOAuthCode(t, err, "OAUTH_CANCELLED", exitError)
	if ctx.Stderr.(*bytes.Buffer).Len() != 0 {
		t.Fatal("cancelled login began onboarding")
	}
	_, err = pollOAuthDeviceToken(operation, oauthConfig{}, "device", 1, 30)
	requireOAuthCode(t, err, "OAUTH_CANCELLED", exitError)
	address := scratchOAuthAddress(t)
	cfg, err := prepareOAuthCallback(oauthConfig{ListenAddr: address, RedirectURI: "http://" + address + "/callback"}, "state")
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.callback.close()
	_, err = waitForOAuthCode(operation, cfg, "state", time.Second)
	requireOAuthCode(t, err, "OAUTH_CANCELLED", exitError)
}

func TestOAuthExchangeRespectsOperationCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer server.Close()
	operation, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := exchangeOAuthToken(operation, oauthConfig{TokenURL: server.URL}, "synthetic-code", "synthetic-verifier")
		done <- err
	}()
	<-started
	cancel()
	requireOAuthCode(t, <-done, "OAUTH_CANCELLED", exitError)
}

func TestOAuthRejectedExchangePreservesCredentialsAndDoesNotExport(t *testing.T) {
	for _, printEnv := range []bool{false, true} {
		ctx := newAuthTestContext(t)
		if err := profileStore(ctx).Save(context.Background(), ctx.Profile, config.Credential{Token: "synthetic-existing-secret"}, "file"); err != nil {
			t.Fatal(err)
		}
		path := config.CredentialsPathFromConfig(ctx.ConfigPath)
		before, _ := os.ReadFile(path)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/device" {
				fmt.Fprint(w, `{"device_code":"device","user_code":"USER","verification_uri":"https://example.invalid","expires_in":30}`)
				return
			}
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":"synthetic-provider-secret","access_token":"synthetic-provider-secret"}`)
		}))
		args := []string{"--oauth-device", "--client-id", "client", "--oauth-device-url", server.URL + "/device", "--oauth-token-url", server.URL + "/token"}
		if printEnv {
			args = append(args, "--print-env")
		}
		err := authLogin(ctx, args)
		server.Close()
		requireOAuthCode(t, err, "OAUTH_EXCHANGE_FAILED", exitAuth)
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) || ctx.Stdout.(*bytes.Buffer).Len() != 0 || strings.Contains(err.Error(), "synthetic-provider-secret") {
			t.Fatal("rejected exchange changed or exported a credential or leaked provider data")
		}
	}
}

func TestOAuthFailureDoesNotBorrowStoredAuthorizationEvidence(t *testing.T) {
	ctx := newAuthTestContext(t)
	ctx.Mode = output.ModeJSON
	ctx.TokenSource = "credentials"
	report := authorization.Resolve([]byte(readOnlyMetadata), "credentials", true)
	ctx.Authorization = &report
	for _, err := range []error{
		oauthFailure("OAUTH_LIFECYCLE_UNSUPPORTED", "Unsupported proposed grant."),
		&authorization.Error{Code: "OAUTH_SCOPE_INVALID", Message: "Invalid proposed grant.", Reason: "broader-than-requested"},
	} {
		ctx.Stderr.(*bytes.Buffer).Reset()
		writeError(ctx, err)
		if strings.Contains(ctx.Stderr.(*bytes.Buffer).String(), `"authorization"`) || strings.Contains(ctx.Stderr.(*bytes.Buffer).String(), `"effective_scopes"`) {
			t.Fatal("OAuth failure attached an existing credential's evidence")
		}
	}
}

func TestCancelledCredentialPersistenceDoesNotReplaceStoredProfile(t *testing.T) {
	ctx := newAuthTestContext(t)
	if err := profileStore(ctx).Save(context.Background(), ctx.Profile, config.Credential{Token: "synthetic-existing-secret"}, "file"); err != nil {
		t.Fatal(err)
	}
	path := config.CredentialsPathFromConfig(ctx.ConfigPath)
	before, _ := os.ReadFile(path)
	operation, cancel := context.WithCancel(context.Background())
	ctx.OperationContext = operation
	cancel()
	err := storeProfileCredential(ctx, "synthetic-new-secret", authorization.ManualMetadata())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("storage ignored operation cancellation: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) || ctx.Stdout.(*bytes.Buffer).Len() != 0 {
		t.Fatal("cancelled persistence changed credentials or emitted success")
	}
}
