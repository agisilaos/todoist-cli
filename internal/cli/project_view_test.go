package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestProjectViewJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Home"}],"next_cursor":""}`))
		case "/projects/p1":
			_, _ = w.Write([]byte(`{"id":"p1","name":"Home","description":"Personal","workspace_id":"w1","view_style":"list","is_archived":false,"is_shared":false,"is_favorite":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := &Context{
		Stdout: &out,
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeJSON,
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	if err := projectView(ctx, []string{"Home"}); err != nil {
		t.Fatalf("projectView: %v", err)
	}
	if !strings.Contains(out.String(), `"id": "p1"`) || !strings.Contains(out.String(), `"name": "Home"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}
