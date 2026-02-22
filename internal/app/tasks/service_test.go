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

type fakeLister struct {
	tasks []api.Task
	err   error
}

func (f fakeLister) ListByFilter(_ context.Context, _ string) ([]api.Task, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tasks, nil
}

func TestResolveCompletionTargetsSingleFromID(t *testing.T) {
	svc := Service{}
	out, err := svc.ResolveCompletionTargets(context.Background(), ResolveCompletionInput{ID: "id:abc"})
	if err != nil {
		t.Fatalf("ResolveCompletionTargets: %v", err)
	}
	if out.Mode != "single" || out.ID != "abc" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestResolveCompletionTargetsSingleFromRef(t *testing.T) {
	svc := Service{Resolver: fakeResolver{task: api.Task{ID: "t1"}}}
	out, err := svc.ResolveCompletionTargets(context.Background(), ResolveCompletionInput{Ref: "call mom"})
	if err != nil {
		t.Fatalf("ResolveCompletionTargets: %v", err)
	}
	if out.Mode != "single" || out.ID != "t1" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestResolveCompletionTargetsBulk(t *testing.T) {
	svc := Service{Lister: fakeLister{tasks: []api.Task{{ID: "a"}, {ID: "b"}}}}
	out, err := svc.ResolveCompletionTargets(context.Background(), ResolveCompletionInput{Filter: "today", Yes: true})
	if err != nil {
		t.Fatalf("ResolveCompletionTargets: %v", err)
	}
	if out.Mode != "bulk" || len(out.IDs) != 2 {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestResolveCompletionTargetsBulkRequiresYesOrForce(t *testing.T) {
	svc := Service{Lister: fakeLister{tasks: []api.Task{{ID: "a"}}}}
	_, err := svc.ResolveCompletionTargets(context.Background(), ResolveCompletionInput{Filter: "today"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "bulk complete with --filter requires --yes (or --force)" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveCompletionTargetsBulkRejectsMixedInputs(t *testing.T) {
	svc := Service{}
	_, err := svc.ResolveCompletionTargets(context.Background(), ResolveCompletionInput{Filter: "today", ID: "abc"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "--filter cannot be combined with --id or positional task reference" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveCompletionTargetsPropagatesResolverError(t *testing.T) {
	svc := Service{Resolver: fakeResolver{err: errors.New("boom")}}
	_, err := svc.ResolveCompletionTargets(context.Background(), ResolveCompletionInput{Ref: "call mom"})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveMoveTargetsSingleFromRef(t *testing.T) {
	svc := Service{Resolver: fakeResolver{task: api.Task{ID: "t1"}}}
	out, err := svc.ResolveMoveTargets(context.Background(), ResolveMoveInput{Ref: "Call mom", Project: "Inbox"})
	if err != nil {
		t.Fatalf("ResolveMoveTargets: %v", err)
	}
	if out.Mode != "single" || out.ID != "t1" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestResolveMoveTargetsBulkRequiresYes(t *testing.T) {
	svc := Service{Lister: fakeLister{tasks: []api.Task{{ID: "a"}}}}
	_, err := svc.ResolveMoveTargets(context.Background(), ResolveMoveInput{Filter: "today", Project: "Inbox"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "bulk move with --filter requires --yes (or --force)" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveMoveTargetsRequiresDestination(t *testing.T) {
	svc := Service{}
	_, err := svc.ResolveMoveTargets(context.Background(), ResolveMoveInput{ID: "t1"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "at least one of --project, --section, or --parent is required" {
		t.Fatalf("unexpected error: %v", err)
	}
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
