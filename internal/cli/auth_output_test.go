package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestAuthLoginAndLogoutNDJSON(t *testing.T) {
	ctx := newAuthTestContext(t)
	ctx.Mode = output.ModeNDJSON
	ctx.Stdin = strings.NewReader("synthetic-output-token\n")
	authValidationServer(t, ctx, 200, "synthetic-output-token")
	for _, operation := range []string{"login", "logout"} {
		ctx.Stdout.(*bytes.Buffer).Reset()
		var err error
		if operation == "login" {
			err = authLogin(ctx, []string{"--token-stdin", "--credential-store=file"})
		} else {
			err = authLogout(ctx)
		}
		if err != nil {
			t.Fatalf("%s failed: %v", operation, err)
		}
		data := ctx.Stdout.(*bytes.Buffer).Bytes()
		if bytes.Contains(data, []byte("synthetic-output-token")) {
			t.Fatal("success output exposed token")
		}
		if bytes.Count(data, []byte("\n")) != 1 {
			t.Fatal("expected one NDJSON record")
		}
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("%s returned invalid JSON", operation)
		}
		key := "stored"
		if operation == "logout" {
			key = "removed"
		}
		if payload[key] != true || payload["profile"] != ctx.Profile {
			t.Fatalf("%s missing success metadata", operation)
		}
	}
}
