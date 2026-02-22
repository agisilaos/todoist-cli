package cli

import (
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
