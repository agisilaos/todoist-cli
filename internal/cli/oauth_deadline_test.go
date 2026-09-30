package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeviceApprovalOutlivesRequestTimeout(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/device" {
			fmt.Fprint(w, `{"device_code":"synthetic","user_code":"TEST","verification_uri":"https://example.invalid","interval":1,"expires_in":30}`)
			return
		}
		attempts++
		if attempts == 1 {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":"authorization_pending"}`)
			return
		}
		fmt.Fprint(w, `{"access_token":"synthetic-oauth-token"}`)
	}))
	defer server.Close()
	ctx := newAuthTestContext(t)
	ctx.Config.TimeoutSeconds = 1
	if _, err := authOAuthDeviceLogin(ctx, oauthConfig{ClientID: "test", DeviceURL: server.URL + "/device", TokenURL: server.URL + "/token"}); err != nil {
		t.Fatalf("approval wait inherited request timeout: %v", err)
	}
}

func TestPKCEApprovalGetsSeparateExchangeDeadline(t *testing.T) {
	ctx := newAuthTestContext(t)
	ctx.Config.TimeoutSeconds = 1
	previousWait, previousExchange := waitForOAuthCodeFn, exchangeOAuthTokenFn
	defer func() { waitForOAuthCodeFn = previousWait; exchangeOAuthTokenFn = previousExchange }()
	waitForOAuthCodeFn = func(wait context.Context, _ oauthConfig, _ string, _ time.Duration) (string, error) {
		select {
		case <-time.After(1100 * time.Millisecond):
			return "synthetic-code", nil
		case <-wait.Done():
			return "", wait.Err()
		}
	}
	exchangeOAuthTokenFn = func(exchange context.Context, _ oauthConfig, _, _ string) (oauthToken, error) {
		if err := exchange.Err(); err != nil {
			return oauthToken{}, err
		}
		deadline, ok := exchange.Deadline()
		if !ok || time.Until(deadline) <= 0 {
			t.Fatal("token exchange has no fresh request deadline")
		}
		return oauthToken{AccessToken: "synthetic"}, nil
	}
	address := scratchOAuthAddress(t)
	if _, err := authOAuthLogin(ctx, oauthConfig{ClientID: "test", AuthorizeURL: "https://example.invalid", ListenAddr: address, RedirectURI: "http://" + address + "/callback", NoBrowser: true}); err != nil {
		t.Fatal(err)
	}
}

func TestDevicePollBoundsIndividualRequests(t *testing.T) {
	for _, phase := range []string{"headers", "body"} {
		t.Run(phase, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if phase == "body" {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				time.Sleep(100 * time.Millisecond)
				fmt.Fprint(w, `{"access_token":"synthetic"}`)
			}))
			defer server.Close()
			_, err := pollOAuthDeviceToken(context.Background(), oauthConfig{TokenURL: server.URL, RequestTimeout: 10 * time.Millisecond}, "synthetic", 1, 30)
			requireOAuthCode(t, err, "OAUTH_TIMEOUT", exitError)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("slow token request did not retain its timeout cause")
			}
		})
	}
}
