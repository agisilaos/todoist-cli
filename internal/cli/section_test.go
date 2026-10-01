package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestSectionDeleteRejectsNormalizedBlankIDBeforeAuthentication(t *testing.T) {
	for _, id := range []string{" ", "id:", " Id: "} {
		t.Run(id, func(t *testing.T) {
			ctx, out := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			})
			ctx.Token = ""
			err := sectionDelete(ctx, []string{"--id", id})
			if err == nil || toExitCode(err) != exitUsage || !strings.Contains(err.Error(), "--id is required") {
				t.Fatalf("expected missing-ID usage error, got %v", err)
			}
			if out.Len() != 0 {
				t.Fatalf("unexpected output: %s", out.String())
			}
		})
	}
}

func TestSectionDeleteConfirmationAndDryRun(t *testing.T) {
	for _, tc := range []struct {
		name       string
		global     GlobalOptions
		wantError  bool
		wantDelete bool
		wantOutput string
	}{
		{name: "nonterminal", wantError: true},
		{name: "no-input", global: GlobalOptions{NoInput: true}, wantError: true},
		{name: "force", global: GlobalOptions{Force: true, NoInput: true}, wantDelete: true, wantOutput: "deleted s1"},
		{name: "dry-run", global: GlobalOptions{DryRun: true, NoInput: true}, wantOutput: "dry run: section delete"},
		{name: "force-dry-run", global: GlobalOptions{Force: true, DryRun: true, NoInput: true}, wantOutput: "dry run: section delete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deletes := 0
			ctx, out := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/sections/s1" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				deletes++
				w.WriteHeader(http.StatusNoContent)
			})
			ctx.Global = tc.global
			err := sectionDelete(ctx, []string{"--id", " Id:s1 "})
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "confirmation required") || out.Len() != 0 {
					t.Fatalf("expected confirmation error without output, got %v; output=%s", err, out.String())
				}
			} else if err != nil || !strings.Contains(out.String(), tc.wantOutput) {
				t.Fatalf("expected %q, got error=%v output=%s", tc.wantOutput, err, out.String())
			}
			wantDeletes := 0
			if tc.wantDelete {
				wantDeletes = 1
			}
			if deletes != wantDeletes {
				t.Fatalf("deletes=%d, want %d", deletes, wantDeletes)
			}
		})
	}
}
