package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
)

func TestResolveFilterRefAmbiguous(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filters" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[{"id":"f1","name":"Today","query":"today"},{"id":"f2","name":"Today Focus","query":"today & @focus"}]`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	_, err := resolveFilterRef(ctx, "tod")
	if err == nil {
		t.Fatalf("expected ambiguity error")
	}
	var codeErr *CodeError
	if !errors.As(err, &codeErr) || codeErr.Code != exitUsage {
		t.Fatalf("expected usage error, got %v", err)
	}
	var ambiguous *AmbiguousMatchError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected AmbiguousMatchError, got %T", err)
	}
	if ambiguous.Entity != "filter" || ambiguous.Input != "tod" {
		t.Fatalf("unexpected ambiguous metadata: %#v", ambiguous)
	}
	if len(ambiguous.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %v", ambiguous.Matches)
	}
}

func TestResolveTaskRefAmbiguous(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"t1","content":"Plan docs"},{"id":"t2","content":"Plan roadmap"}],"next_cursor":""}`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
		Global: GlobalOptions{NoInput: true},
	}
	_, err := resolveTaskRef(ctx, "plan")
	if err == nil {
		t.Fatalf("expected ambiguity error")
	}
	var ambiguous *AmbiguousMatchError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected AmbiguousMatchError, got %T", err)
	}
	if ambiguous.Entity != "task" || ambiguous.Input != "plan" {
		t.Fatalf("unexpected ambiguous metadata: %#v", ambiguous)
	}
	if len(ambiguous.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %v", ambiguous.Matches)
	}
}

func TestResolveTaskRefPrefersExactTextMatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"t1","content":"Call mom"},{"id":"t2","content":"Sample code docs"}],"next_cursor":""}`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
		Global: GlobalOptions{NoInput: true},
	}
	task, err := resolveTaskRef(ctx, "call mom")
	if err != nil {
		t.Fatalf("resolveTaskRef: %v", err)
	}
	if task.ID != "t1" {
		t.Fatalf("expected t1, got %q", task.ID)
	}
}

func TestListAllActiveTasksUsesCache(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" {
			http.NotFound(w, r)
			return
		}
		hits++
		_, _ = w.Write([]byte(`{"results":[{"id":"t1","content":"Write tests"}],"next_cursor":""}`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	a, err := listAllActiveTasks(ctx)
	if err != nil {
		t.Fatalf("first listAllActiveTasks: %v", err)
	}
	b, err := listAllActiveTasks(ctx)
	if err != nil {
		t.Fatalf("second listAllActiveTasks: %v", err)
	}
	if len(a) != 1 || len(b) != 1 || a[0].ID != "t1" || b[0].ID != "t1" {
		t.Fatalf("unexpected task results: %#v %#v", a, b)
	}
	if hits != 1 {
		t.Fatalf("expected one API hit, got %d", hits)
	}
}

func TestResolveProjectIDReturnsNumericWithoutLookup(t *testing.T) {
	ctx := &Context{}
	got, err := resolveProjectID(ctx, "id:12345")
	if err != nil {
		t.Fatalf("resolveProjectID: %v", err)
	}
	if got != "12345" {
		t.Fatalf("expected numeric id passthrough, got %q", got)
	}
}

func TestResolveProjectIDFromURLWithoutLookup(t *testing.T) {
	ctx := &Context{}
	got, err := resolveProjectID(ctx, "https://app.todoist.com/app/project/personal-2203306141")
	if err != nil {
		t.Fatalf("resolveProjectID: %v", err)
	}
	if got != "2203306141" {
		t.Fatalf("expected project id from URL, got %q", got)
	}
}

func TestResolveProjectIDRejectsMismatchedURLType(t *testing.T) {
	ctx := &Context{}
	_, err := resolveProjectID(ctx, "https://app.todoist.com/app/task/call-mom-abc123")
	if err == nil {
		t.Fatalf("expected mismatch error")
	}
	var codeErr *CodeError
	if !errors.As(err, &codeErr) || codeErr.Code != exitUsage {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestResolveProjectIDPropagatesLookupErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	if _, err := resolveProjectID(ctx, "Home"); err == nil {
		t.Fatalf("expected resolver error")
	}
}

func TestResolveFilterRefFromURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filters" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[{"id":"f1","name":"Today","query":"today"}]`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	filter, err := resolveFilterRef(ctx, "https://app.todoist.com/app/filter/today-f1")
	if err != nil {
		t.Fatalf("resolveFilterRef: %v", err)
	}
	if filter.ID != "f1" {
		t.Fatalf("expected filter f1, got %q", filter.ID)
	}
}

func TestResolveWorkspaceIDReturnsDirectID(t *testing.T) {
	ctx := &Context{}
	got, err := resolveWorkspaceID(ctx, "id:w1")
	if err != nil {
		t.Fatalf("resolveWorkspaceID: %v", err)
	}
	if got != "w1" {
		t.Fatalf("unexpected workspace id: %q", got)
	}
}

func TestResolveWorkspaceIDByName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"workspaces":[{"id":"w1","name":"Acme Corp"}]}`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	got, err := resolveWorkspaceID(ctx, "acme corp")
	if err != nil {
		t.Fatalf("resolveWorkspaceID: %v", err)
	}
	if got != "w1" {
		t.Fatalf("unexpected workspace id: %q", got)
	}
}
