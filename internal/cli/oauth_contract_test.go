package cli

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type oauthURLWriter struct {
	bytes.Buffer
	urls chan string
}

func (w *oauthURLWriter) Write(p []byte) (int, error) {
	text := string(p)
	if strings.HasPrefix(text, "OAuth authorization URL:\n") {
		w.urls <- strings.TrimSpace(strings.TrimPrefix(text, "OAuth authorization URL:\n"))
	}
	return w.Buffer.Write(p)
}

// Exercise the actual CLI and HTTP exchange; the browser is simulated at the callback boundary.
func TestAuthorizationOAuthScopesAcrossPKCEAndDevice(t *testing.T) {
	for _, flow := range []string{"pkce", "device"} {
		for _, tc := range []struct {
			name           string
			readOnly       bool
			response       string
			mode, evidence string
			code           int
		}{
			{"read-only", true, `,"scope":"data:read"`, "read-only", "token-response", 0},
			{"read-write", false, "", "read-write", "oauth-request", 0},
			{"read-only-omitted", true, "", "read-only", "oauth-request", 0},
			{"reduced-grant", false, `,"scope":"data:read"`, "read-only", "token-response", 0},
			{"broader-grant", true, `,"scope":"data:read_write"`, "", "", 3},
			{"empty-grant", false, `,"scope":""`, "", "", 3},
			{"unsupported-grant", false, `,"scope":"task:add"`, "", "", 3},
			{"null-grant", false, `,"scope":null`, "", "", 3},
		} {
			t.Run(flow+"/"+tc.name, func(t *testing.T) {
				path := authorizationFixture(t, readOnlyMetadata)
				before, _ := os.ReadFile(filepath.Join(filepath.Dir(path), "credentials.json"))
				requested := make(chan string, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					r.ParseForm()
					if r.URL.Path == "/device" {
						requested <- r.Form.Get("scope")
						fmt.Fprint(w, `{"device_code":"d","user_code":"u","verification_uri":"https://example.test","interval":1,"expires_in":60}`)
						return
					}
					if flow == "pkce" && (r.Form.Get("code_verifier") == "" || r.Form.Get("code") != "approved") {
						t.Error("missing PKCE exchange fields")
					}
					fmt.Fprint(w, `{"access_token":"new-secret"`+tc.response+`}`)
				}))
				defer server.Close()
				args := []string{"--config", path, "auth", "login", "--client-id", "client", "--oauth-token-url", server.URL + "/token", "--json"}
				if tc.readOnly {
					args = append(args, "--read-only")
				}
				var out bytes.Buffer
				diagnostic := &oauthURLWriter{urls: make(chan string, 1)}
				var callbackDone chan error
				if flow == "device" {
					args = append(args, "--oauth-device", "--oauth-device-url", server.URL+"/device")
				} else {
					listener, err := net.Listen("tcp", "127.0.0.1:0")
					if err != nil {
						t.Fatal(err)
					}
					addr := listener.Addr().String()
					listener.Close()
					args = append(args, "--oauth", "--no-browser", "--oauth-listen", addr, "--oauth-authorize-url", server.URL+"/authorize")
					callbackDone = make(chan error, 1)
					go func() {
						authURL := <-diagnostic.urls
						parsed, err := url.Parse(authURL)
						if err != nil {
							callbackDone <- err
							return
						}
						query := parsed.Query()
						requested <- query.Get("scope")
						if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
							t.Error("missing PKCE challenge")
						}
						callback := query.Get("redirect_uri") + "?code=approved&state=" + url.QueryEscape(query.Get("state"))
						client := &http.Client{Timeout: time.Second}
						for attempts := 0; attempts < 100; attempts++ {
							response, e := client.Get(callback)
							if e == nil {
								response.Body.Close()
								callbackDone <- nil
								return
							}
							err = e
							time.Sleep(5 * time.Millisecond)
						}
						callbackDone <- err
					}()
				}
				code := Execute(args, &out, diagnostic)
				if code != tc.code {
					t.Fatalf("login %d want %d: %s %s", code, tc.code, out.String(), diagnostic.String())
				}
				wantScope := "data:read_write,data:delete,project:delete"
				if tc.readOnly {
					wantScope = "data:read"
				}
				if got := <-requested; got != wantScope {
					t.Fatalf("scope %q want %q", got, wantScope)
				}
				if callbackDone != nil {
					if err := <-callbackDone; err != nil {
						t.Fatal(err)
					}
				}
				if code != 0 {
					after, _ := os.ReadFile(filepath.Join(filepath.Dir(path), "credentials.json"))
					if !bytes.Equal(before, after) {
						t.Fatal("rejected grant replaced credential")
					}
					if tc.name == "broader-grant" && !strings.Contains(diagnostic.String(), "broader-than-requested") {
						t.Fatalf("missing scope refusal reason: %s", diagnostic.String())
					}
					if !strings.Contains(diagnostic.String(), "OAUTH_SCOPE_INVALID") {
						t.Fatalf("missing stable error: %s", diagnostic.String())
					}
				} else {
					_, status, _ := executeAuthorization(t, path, "auth", "status", "--json")
					if !strings.Contains(status, `"mode": "`+tc.mode+`"`) || !strings.Contains(status, `"scope_evidence": "`+tc.evidence+`"`) || !strings.Contains(status, `"origin": "oauth-`+flow+`"`) {
						t.Fatalf("metadata: %s", status)
					}
				}
				if strings.Contains(out.String()+diagnostic.String(), "new-secret") {
					t.Fatal("OAuth login leaked credential")
				}
			})
		}
	}
}
