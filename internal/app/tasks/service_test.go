package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
)

type fakeResolver struct {
	task api.Task
	err  error
}

func (f fakeResolver) ResolveTaskRef(_ context.Context, _ string) (api.Task, error) {
	if f.err != nil {
		return api.Task{}, f.err
	}
	return f.task, nil
}

func TestResolveTaskTargetFromRef(t *testing.T) {
	svc := Service{Resolver: fakeResolver{task: api.Task{ID: "t99"}}}
	id, err := svc.ResolveTaskTarget(context.Background(), ResolveTaskTargetInput{Ref: "Pay rent"})
	if err != nil {
		t.Fatalf("ResolveTaskTarget: %v", err)
	}
	if id != "t99" {
		t.Fatalf("unexpected id: %q", id)
	}
}

func TestResolveTaskTargetRequiresValue(t *testing.T) {
	svc := Service{}
	if _, err := svc.ResolveTaskTarget(context.Background(), ResolveTaskTargetInput{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveTaskTargetRejectsMismatchedURLType(t *testing.T) {
	svc := Service{}
	_, err := svc.ResolveTaskTarget(context.Background(), ResolveTaskTargetInput{ID: "https://app.todoist.com/app/project/home-2203306141"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestActionSelectionGuards(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   ActionSelection
		invalid bool
	}{
		{"ID", ActionSelection{ID: "t"}, false},
		{"text", ActionSelection{Ref: "Report"}, false},
		{"confirmed batch", ActionSelection{Filter: "today", Confirmed: true}, false},
		{"missing target", ActionSelection{}, true},
		{"mixed ID and text", ActionSelection{ID: "t", Ref: "Report"}, true},
		{"mixed batch and ID", ActionSelection{ID: "t", Filter: "today", Confirmed: true}, true},
		{"mixed batch and text", ActionSelection{Ref: "Report", Filter: "today", Confirmed: true}, true},
		{"unconfirmed batch", ActionSelection{Filter: "today"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateActionSelection(tc.input); (err != nil) != tc.invalid {
				t.Fatalf("invalid=%t, error=%v", tc.invalid, err)
			}
		})
	}
}

func TestMoveDestinationGuards(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   MoveDestination
		invalid bool
	}{
		{"project", MoveDestination{Project: "p"}, false},
		{"scoped section", MoveDestination{Project: "p", Section: "s"}, false},
		{"parent", MoveDestination{Parent: "t"}, false},
		{"clear parent", MoveDestination{ClearParent: true}, false},
		{"clear section", MoveDestination{ClearSection: true}, false},
		{"clear both", MoveDestination{ClearParent: true, ClearSection: true}, false},
		{"missing destination", MoveDestination{}, true},
		{"parent and project", MoveDestination{Parent: "t", Project: "p"}, true},
		{"parent and section", MoveDestination{Parent: "t", Section: "s"}, true},
		{"clear and destination", MoveDestination{ClearParent: true, Project: "p"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateMoveDestination(tc.input); (err != nil) != tc.invalid {
				t.Fatalf("invalid=%t, error=%v", tc.invalid, err)
			}
		})
	}
}

func TestResolveTaskTargetFromExactID(t *testing.T) {
	for _, id := range []string{"id:abc123", "https://app.todoist.com/app/task/report-abc123"} {
		out, err := (Service{}).ResolveTaskTarget(context.Background(), ResolveTaskTargetInput{ID: id})
		if err != nil || out != "abc123" {
			t.Fatalf("id=%q, got %q: %v", id, out, err)
		}
	}
}

func TestResolveTaskTargetPropagatesResolverError(t *testing.T) {
	want := errors.New("lookup failed")
	svc := Service{Resolver: fakeResolver{err: want}}
	_, err := svc.ResolveTaskTarget(context.Background(), ResolveTaskTargetInput{Ref: "Report"})
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want resolver error", err)
	}
}
