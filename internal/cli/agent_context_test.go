package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func TestParseDays(t *testing.T) {
	val, err := parseDays("7d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 7 {
		t.Fatalf("expected 7, got %d", val)
	}
}

func TestAgentPlannerCompletedContextRespectsScope(t *testing.T) {
	for _, tc := range []struct {
		name, project, label string
		want                 []string
	}{
		{"project", "Work", "", []string{"work-urgent", "work-other"}},
		{"label", "", "urgent", []string{"work-urgent", "home-urgent"}},
		{"both", "Work", "urgent", []string{"work-urgent"}},
		{"unscoped", "", "", []string{"work-urgent", "work-other", "home-urgent"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := []string{"--context-completed", "7d"}
			if tc.project != "" {
				flags = append(flags, "--context-project", tc.project)
			}
			if tc.label != "" {
				flags = append(flags, "--context-label", tc.label)
			}
			request := captureAgentPlannerRequest(t, flags...)
			got := make([]string, 0, len(request.Context.CompletedTasks))
			for _, task := range request.Context.CompletedTasks {
				got = append(got, task.(map[string]any)["id"].(string))
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("completed planner tasks = %v, want scoped tasks %v", got, tc.want)
			}
		})
	}
}

func captureAgentPlannerRequest(t *testing.T, flags ...string) PlannerRequest {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("external planners currently require /bin/sh")
	}
	dir := t.TempDir()
	capturePath := filepath.Join(dir, "planner-request.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "no mutations allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/projects":
			fmt.Fprint(w, `{"results":[{"id":"work","name":"Work"},{"id":"home","name":"Home"}]}`)
		case "/sections":
			fmt.Fprint(w, `{"results":[]}`)
		case "/labels":
			fmt.Fprint(w, `{"results":[{"id":"label-urgent","name":"urgent"}]}`)
		case "/tasks":
			fmt.Fprint(w, `{"results":[{"id":"active-urgent","content":"Active","project_id":"work","labels":["urgent"]}]}`)
		case "/tasks/completed/by_completion_date":
			fmt.Fprint(w, `{"items":[{"id":"work-urgent","content":"One","project_id":"work","labels":["urgent"]},{"id":"work-other","content":"Two","project_id":"work","labels":["other"]},{"id":"home-urgent","content":"Three","project_id":"home","labels":["urgent"]}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	planner := "cat > " + shellEscape(capturePath) + `; printf '%s' '{"version":1,"confirm_token":"capture","actions":[]}'`
	args := []string{"--config", filepath.Join(dir, "config.json"), "--base-url", server.URL, "--json", "agent", "plan", "Capture context", "--planner", planner}
	args = append(args, flags...)
	var out, diagnostic bytes.Buffer
	code := executeTestWithEnvironment(args, &out, &diagnostic, Environment{Getenv: func(key string) string {
		if key == "TODOIST_TOKEN" {
			return "synthetic"
		}
		return ""
	}})
	if code != 0 {
		t.Fatalf("planner exited %d: %s", code, diagnostic.String())
	}
	data, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}
	var request PlannerRequest
	if err := json.Unmarshal(data, &request); err != nil {
		t.Fatal(err)
	}
	return request
}

func TestFilterProjectIDsUnknown(t *testing.T) {
	ctx := &Context{}
	projects := []api.Project{{ID: "1", Name: "Work"}}
	_, err := filterProjectIDs(ctx, projects, []string{"Nope"})
	if err == nil {
		t.Fatalf("expected error for unknown project")
	}
}

func TestFilterActiveTasksForContext(t *testing.T) {
	tasks := []api.Task{
		{ID: "t1", Content: "A", ProjectID: "p1", Labels: []string{"urgent"}},
		{ID: "t2", Content: "B", ProjectID: "p2", Labels: []string{"chore"}},
		{ID: "t3", Content: "C", ProjectID: "p1", Labels: []string{"chore"}},
	}
	projectIDs := map[string]struct{}{"p1": {}}
	got := filterActiveTasksForContext(tasks, projectIDs, []string{"urgent"})
	if len(got) != 1 || got[0].ID != "t1" {
		t.Fatalf("unexpected filtered tasks: %#v", got)
	}
}

func TestFilterActiveTasksForContextCapsAt50(t *testing.T) {
	tasks := make([]api.Task, 0, 60)
	for i := 0; i < 60; i++ {
		tasks = append(tasks, api.Task{ID: time.Now().Add(time.Duration(i) * time.Second).Format("150405.000")})
	}
	got := filterActiveTasksForContext(tasks, nil, nil)
	if len(got) != 50 {
		t.Fatalf("expected cap at 50, got %d", len(got))
	}
}
