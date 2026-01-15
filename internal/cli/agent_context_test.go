package cli

import (
	"testing"

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
