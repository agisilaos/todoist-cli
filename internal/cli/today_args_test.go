package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTodayRejectsUnsupportedArgumentsBeforeAuthentication(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	path := filepath.Join(t.TempDir(), "config.json")
	for _, args := range [][]string{{"--limit", "2"}, {"--bogus"}, {"tomorrow"}} {
		code, out, errOut := executeAuthorization(t, path, append([]string{"today", "--json"}, args...)...)
		if code != exitUsage || out != "" || strings.Contains(errOut, "missing auth token") {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", args, code, out, errOut)
		}
	}
}
