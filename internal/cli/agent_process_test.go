package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAgentPlannerCancellationStopsOwnedSubprocesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("external planners currently require /bin/sh")
	}
	for _, cause := range []string{"timeout", "caller cancellation", "timeout after shell exit", "pipe drain after shell exit"} {
		t.Run(cause, func(t *testing.T) {
			dir := t.TempDir()
			heartbeat := filepath.Join(dir, "child-heartbeat")
			child := filepath.Join(dir, "planner-child.sh")
			// A finite child makes failure bounded while exposing whether cancellation
			// stops child work as well as returning control to the caller.
			script := "i=0; while [ $i -lt 60 ]; do printf x >> " + shellEscape(heartbeat) + "; i=$((i+1)); sleep 0.05; done\n"
			if err := os.WriteFile(child, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					http.Error(w, "no mutations allowed", http.StatusMethodNotAllowed)
					return
				}
				fmt.Fprint(w, `{"results":[]}`)
			}))
			defer server.Close()
			operation, cancel := context.WithCancel(context.Background())
			defer cancel()
			stopWatching := make(chan struct{})
			defer close(stopWatching)
			if cause == "caller cancellation" {
				go func() {
					ticker := time.NewTicker(10 * time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-stopWatching:
							return
						case <-ticker.C:
							if data, _ := os.ReadFile(heartbeat); len(data) > 0 {
								cancel()
								return
							}
						}
					}
				}()
			}
			planner := "/bin/sh " + shellEscape(child) + `; printf '{}'`
			if strings.HasSuffix(cause, "after shell exit") {
				planner = "/bin/sh " + shellEscape(child) + ` & printf '{}'`
			}
			args := []string{"--config", filepath.Join(dir, "config.json"), "--base-url", server.URL, "--json", "agent", "plan", "Cancel planner", "--planner", planner}
			if strings.HasPrefix(cause, "timeout") {
				args = append(args, "--timeout", "1")
			}
			if cause == "pipe drain after shell exit" {
				args = append(args, "--timeout", "5")
			}
			var out, diagnostic bytes.Buffer
			started := time.Now()
			code := executeTestWithEnvironment(args, &out, &diagnostic, Environment{
				OperationContext: operation,
				Getenv: func(key string) string {
					if key == "TODOIST_TOKEN" {
						return "synthetic"
					}
					return ""
				},
			})
			elapsed := time.Since(started)
			wantError := "context deadline exceeded"
			if cause == "caller cancellation" {
				wantError = "context canceled"
			}
			if cause == "pipe drain after shell exit" {
				wantError = "WaitDelay"
			}
			if code != 1 || !strings.Contains(diagnostic.String(), wantError) || out.Len() != 0 {
				t.Fatalf("cancelled planner returned exit %d, stdout=%s stderr=%s", code, out.String(), diagnostic.String())
			}
			before, err := os.ReadFile(heartbeat)
			if err != nil || len(before) == 0 {
				t.Fatalf("child did not start before cancellation: %v", err)
			}
			if elapsed >= 2500*time.Millisecond {
				t.Errorf("planner cancellation waited for child completion: elapsed %s", elapsed)
			}
			time.Sleep(250 * time.Millisecond)
			after, err := os.ReadFile(heartbeat)
			if err != nil || !bytes.Equal(before, after) {
				t.Errorf("planner child continued work after cancellation: before=%d after=%d err=%v", len(before), len(after), err)
			}
		})
	}
}
