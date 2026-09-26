package cli

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFlagShapedContentReachesAPI(t *testing.T) {
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/tasks" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var task struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			t.Error(err)
		}
		received = task.Content
		fmt.Fprint(w, `{"id":"101","content":"saved"}`)
	}))
	defer server.Close()
	t.Setenv("TODOIST_TOKEN", "synthetic-flag-value-token")
	t.Setenv("TODOIST_BASE_URL", server.URL)
	for _, value := range []string{"--json", "--help", "--version", "--profile", "-n", "--ids-only"} {
		code, out, errOut := executeAuthorization(t, filepath.Join(t.TempDir(), "config.json"), "task", "add", "--content", value, "--json")
		if code != 0 || !json.Valid([]byte(out)) || received != value {
			t.Errorf("value %q: received=%q exit=%d stdout=%q stderr=%q", value, received, code, out, errOut)
		}
	}
}

// This guards the duplicated arity metadata at the global/command parser boundary.
func TestCommandFlagValuesMatchRegistrations(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			receiver, ok := selector.X.(*ast.Ident)
			if !ok || receiver.Name != "fs" {
				return true
			}
			takesValue := false
			switch selector.Sel.Name {
			case "StringVar", "IntVar", "Var":
				takesValue = true
			case "BoolVar":
			default:
				return true
			}
			name := sourceString(call.Args[1])
			if name == "" {
				t.Fatalf("nonliteral flag registration in %s", path)
			}
			if commandFlagTakesValue(name) != takesValue {
				t.Errorf("update command flag arity for %s in %s", name, path)
			}
			if takesValue {
				args := []string{"task", "add", "--" + name, "--help"}
				opts, rest, err := parseGlobalFlags(append(args, "--json"), nil)
				if err != nil || opts.Help || !opts.JSON || !reflect.DeepEqual(rest, args) {
					t.Errorf("value not preserved for %s: %+v %v %v", name, opts, rest, err)
				}
			} else if name != "help" && name != "h" {
				opts, _, err := parseGlobalFlags([]string{"task", "list", "--" + name, "--json"}, nil)
				if err != nil || !opts.JSON {
					t.Errorf("boolean %s swallowed output flag", name)
				}
			}
			return true
		})
	}
}
