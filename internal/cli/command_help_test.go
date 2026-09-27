package cli

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

func helpTreeState(t *testing.T, dir string) map[string]string {
	t.Helper()
	state := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		state[path] = info.Mode().String()
		if !entry.IsDir() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			state[path] += string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestFocusedHelpAllLeavesOfflineAndWithoutSideEffects(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "help must not request data", 500)
	}))
	defer server.Close()
	t.Setenv("TODOIST_TOKEN", "")
	t.Setenv("TODOIST_BASE_URL", server.URL)
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	configPath := filepath.Join(dir, "config.json")
	// If help reaches credential parsing, these deliberately invalid bytes fail.
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), []byte("untouched credentials"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, config := range []string{"{}", "{broken"} {
		if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		before := helpTreeState(t, dir)
		for path, page := range commandHelpCatalog {
			if page.examples == "" {
				continue
			}
			words := strings.Fields(path)
			variants := [][]string{
				append(append([]string{}, words...), "--help"),
				append([]string{"help"}, words...),
				append([]string{"--help"}, words...),
				append(append([]string{words[0], "help"}, words[1:]...), "--no-input"),
			}
			for _, alias := range strings.Fields(page.aliases) {
				aliasWords := append([]string{}, words...)
				aliasWords[len(aliasWords)-1] = alias
				variants = append(variants, append(aliasWords, "-h"))
			}
			for _, args := range variants {
				// A progress path that could not be opened proves help precedes log creation.
				args = append(args, "--progress-jsonl", filepath.Join(configPath, "cannot-create"))
				code, out, errOut := executeAuthorization(t, configPath, args...)
				if code != 0 || errOut != "" {
					t.Fatalf("%v: exit=%d stderr=%q", args, code, errOut)
				}
				usage := strings.SplitN(out, "\n\n", 2)[0]
				for _, line := range strings.Split(usage, "\n")[1:] {
					if !strings.HasPrefix(line, "  todoist "+path) {
						t.Errorf("%v: unrelated usage %q", args, line)
					}
				}
				for _, want := range []string{"Usage:", "Flags:", "Examples:", "todoist --help"} {
					if !strings.Contains(out, want) {
						t.Errorf("%v: missing %q", args, want)
					}
				}
			}
		}
		if after := helpTreeState(t, dir); !reflect.DeepEqual(before, after) {
			t.Fatal("help changed local state")
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("help sent %d requests", requests.Load())
	}
}

func TestLeafHelpFlagsMatchRegistrations(t *testing.T) {
	commands, globals := documentedCommandFlags(t)
	flagPattern := regexp.MustCompile(`(?m)^  --([a-z][a-z-]*)`)
	manifest, err := os.ReadFile("../../scripts/help-snapshots.txt")
	if err != nil {
		t.Fatal(err)
	}
	for path, page := range commandHelpCatalog {
		if page.examples == "" {
			continue
		}
		registered, ok := commands[path]
		if !ok {
			t.Errorf("no parser inventory for %s", path)
			continue
		}
		got := map[string]bool{}
		for _, match := range flagPattern.FindAllStringSubmatch(page.flags, -1) {
			got[match[1]] = true
			if !registered[match[1]] {
				t.Errorf("%s advertises unsupported --%s", path, match[1])
			}
		}
		for name := range registered {
			if name != "help" && name != "h" && !globals[name] && !got[name] {
				t.Errorf("%s omits --%s", path, name)
			}
		}
		if !strings.Contains(string(manifest), "\t"+path+" --help\n") {
			t.Errorf("missing snapshot for %s", path)
		}
	}
}

func TestHelpCatalogCoversDispatch(t *testing.T) {
	// Compare the public command cases and alias maps to the help catalog, rather
	// than only iterating the catalog (which would miss an omitted command).
	groups := map[string]string{"": "dispatch", "auth": "authCommand", "task": "taskCommand", "project": "projectCommand", "filter": "filterCommand", "workspace": "workspaceCommand", "section": "sectionCommand", "label": "labelCommand", "comment": "commentCommand", "reminder": "reminderCommand", "notification": "notificationCommand", "stats": "statsCommand", "settings": "settingsCommand", "agent": "agentCommand", "agent schedule": "agentSchedule", "inbox": "inboxCommand", "completion": "completionScript"}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{"completion install": true, "completion uninstall": true}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		tree, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range tree.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			for parent, name := range groups {
				if fn.Name.Name != name {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if clause, ok := n.(*ast.CaseClause); ok {
						for _, expr := range clause.List {
							child := sourceString(expr)
							if child == "" || child == "help" || strings.HasPrefix(child, "-") {
								continue
							}
							path, exists := findHelpChild(parent, child)
							if !exists {
								t.Errorf("help missing dispatched %s %s", parent, child)
							} else {
								found[path] = true
							}
						}
					}
					if pair, ok := n.(*ast.KeyValueExpr); ok {
						alias, target := sourceString(pair.Key), sourceString(pair.Value)
						if alias != "" && target != "" {
							path, exists := findHelpChild(parent, alias)
							if !exists || path != strings.TrimSpace(parent+" "+target) {
								t.Errorf("help alias mismatch: %s %s -> %s", parent, alias, target)
							}
						}
					}
					return true
				})
			}
		}
	}
	for path := range commandHelpCatalog {
		if path != "help" && !found[path] {
			t.Errorf("help describes undispatched path %s", path)
		}
	}
}

func TestFocusedCompleteHelpAndRoutingBoundaries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, args := range [][]string{
		{"task", "complete", "--help"}, {"--profile", "reader", "task", "--help", "complete"},
		{"task", "complete", "--filter", "--help", "--help"}, {"task", "complete", "todai", "--help"},
	} {
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 0 || errOut != "" {
			t.Fatalf("%v: %d %q", args, code, errOut)
		}
		for _, want := range []string{"todoist task complete <ref>", "--id <id>", "--filter <query>", "--yes", "--dry-run", "task list", "task reopen"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in %s", want, out)
			}
		}
		for _, unwanted := range []string{"--content", "--priority", "--due-date", "todoist task add"} {
			if strings.Contains(out, unwanted) {
				t.Errorf("unrelated %s", unwanted)
			}
		}
	}
	// Existing explicit group pages and the historical help-only topic remain intact.
	for _, args := range [][]string{{"task", "--help"}, {"agent", "schedule", "--help"}, {"help", "examples"}} {
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 0 || out == "" || errOut != "" {
			t.Errorf("%v: %d %q", args, code, errOut)
		}
	}
}

func TestUnknownCommandRecoveryAndMachineCompatibility(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	path := filepath.Join(t.TempDir(), "config.json")
	cases := []struct {
		args             []string
		text, hint, help string
	}{
		{[]string{"todai"}, "unknown command: todai", "todoist today", "todoist --help"},
		{[]string{"task", "complet"}, "unknown task subcommand: complet", "todoist task complete", "todoist task --help"},
		{[]string{"agent", "schedule", "pritn"}, "unknown schedule subcommand: pritn", "todoist agent schedule print", "todoist agent schedule --help"},
		{[]string{"completion", "instal"}, "unsupported shell: instal (supported: " + supportedCompletionShells + ")", "todoist completion install", "todoist completion --help"},
	}
	for _, tc := range cases {
		code, out, errOut := executeAuthorization(t, path, tc.args...)
		if code != 2 || out != "" || !strings.Contains(errOut, tc.hint) || !strings.Contains(errOut, tc.help) || strings.Contains(errOut, "Commands:") {
			t.Errorf("%v: %d %q %q", tc.args, code, out, errOut)
		}
		for _, mode := range [][]string{{"--json"}, {"--json", "--quiet-json"}, {"--ndjson"}, {"--plain"}, {"--quiet-json"}} {
			args := append(append([]string{}, tc.args...), mode...)
			code, out, errOut = executeAuthorization(t, path, args...)
			if code != 2 || out != "" {
				t.Errorf("%v: %d %q", args, code, out)
			}
			if mode[0] == "--json" {
				var payload map[string]any
				if err := json.Unmarshal([]byte(errOut), &payload); err != nil {
					t.Fatal(err)
				}
				if payload["error"] != tc.text || len(payload) != 2 {
					t.Errorf("changed machine error: %s", errOut)
				}
			} else {
				want := "error: " + tc.text + "\n"
				if len(tc.args) == 1 {
					var root bytes.Buffer
					printRootHelp(&root)
					want = tc.text + "\n" + root.String()
				}
				if errOut != want {
					t.Errorf("%v: changed machine text %q", args, errOut)
				}
			}
		}
	}
	for _, args := range [][]string{{"help", "todai"}, {"help", "task", "complet"}, {"task", "complet", "--help"}, {"help", "agent", "schedule", "pritn"}} {
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 2 || out != "" || !strings.Contains(errOut, "Did you mean") {
			t.Errorf("%v: %d %q %q", args, code, out, errOut)
		}
	}
	code, out, errOut := executeAuthorization(t, path, "task", "complet", "--ids-only")
	if code != 2 || out != "" || !strings.Contains(errOut, "--ids-only is only supported") || strings.Contains(errOut, "Did you mean") {
		t.Errorf("ID eligibility changed: %d %q %q", code, out, errOut)
	}
}

func TestCommandSuggestionsBoundedAndScoped(t *testing.T) {
	for _, tc := range []struct {
		parent, input string
		want          []string
	}{
		{"", "todai", []string{"today"}}, {"task", "complte", []string{"task complete"}},
		{"task", "complexx", []string{"task complete"}}, {"task", "veiw", []string{"task view"}},
		{"task", "shwo", []string{"task view"}}, {"task", "mve", []string{"task move"}},
		{"task", "ad", nil}, {"task", "--complete", nil}, {"task", "vacation", nil},
		{"task", "nonsense", nil}, {"task", strings.Repeat("x", 1000), nil},
		{"project", "fav", nil}, {"task", "lis", []string{"task list"}},
	} {
		if got := commandSuggestions(tc.parent, tc.input); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s %s: got %v want %v", tc.parent, tc.input, got, tc.want)
		}
	}
	// A short input can still have one unambiguous nearest command.
	if got := commandSuggestions("notification", "rea"); !reflect.DeepEqual(got, []string{"notification read"}) {
		t.Errorf("got %v", got)
	}
	for _, tc := range []struct {
		a, b string
		want int
	}{{"ab", "ba", 1}, {"complete", "complexx", 2}, {"", "abc", 3}, {"today", "today", 0}} {
		if got := commandEditDistance(tc.a, tc.b); got != tc.want {
			t.Errorf("distance(%q,%q)=%d", tc.a, tc.b, got)
		}
	}
}

func TestHelpSkipsRequestedLocalWrites(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	before := helpTreeState(t, dir)
	for _, args := range [][]string{
		{"completion", "install", "bash", "--path", target, "--help"},
		{"completion", "uninstall", "bash", "--path", target, "--help"},
		{"agent", "plan", "instruction", "--out", target, "--planner", "false", "--help"},
		{"agent", "planner", "--set", "--cmd", "false", "--help"},
		{"auth", "logout", "--help"}, {"auth", "migrate", "--credential-store", "file", "--help"},
	} {
		code, _, errOut := executeAuthorization(t, configPath, args...)
		if code != 0 || errOut != "" {
			t.Errorf("%v: %d %s", args, code, errOut)
		}
	}
	if after := helpTreeState(t, dir); !reflect.DeepEqual(before, after) {
		t.Fatal("help changed requested output/config files")
	}
}

func TestTypoAndHelpErrorPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := executeAuthorization(t, path, "todai")
	if code == 0 || strings.Contains(errOut, "unknown command") {
		t.Errorf("config precedence changed: %d %s", code, errOut)
	}
	code, out, errOut := executeAuthorization(t, path, "help", "todai", "--json")
	if code != 2 || out != "" || !json.Valid([]byte(errOut)) || !strings.Contains(errOut, "unknown command: todai") {
		t.Errorf("unknown help: %d %q %s", code, out, errOut)
	}
	for _, args := range [][]string{{"task", "complete", "--help", "--timeout", "bad"}, {"task", "complete", "--help", "--json", "--plain"}} {
		code, _, _ := executeAuthorization(t, path, args...)
		if code != 2 {
			t.Errorf("%v exit %d", args, code)
		}
	}
}

func TestCommandSuggestionAmbiguity(t *testing.T) {
	original := commandHelpCatalog
	t.Cleanup(func() { commandHelpCatalog = original })
	commandHelpCatalog = map[string]commandHelp{
		"cat": {aliases: "cats"}, "cap": {}, "can": {}, "cab": {},
	}
	if got := commandSuggestions("", "car"); len(got) != 0 {
		t.Errorf("four tied matches should be suppressed: %v", got)
	}
	delete(commandHelpCatalog, "cab")
	if got := commandSuggestions("", "car"); !reflect.DeepEqual(got, []string{"can", "cap", "cat"}) {
		t.Errorf("ties should be sorted: %v", got)
	}
	if got := commandSuggestions("", "cats"); !reflect.DeepEqual(got, []string{"cat"}) {
		t.Errorf("aliases should collapse to canonical command: %v", got)
	}
}

func TestUnknownCommandDoesNotDispatchOrChangeFiles(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "unexpected dispatch", 500)
	}))
	defer server.Close()
	t.Setenv("TODOIST_TOKEN", "synthetic-help-test")
	t.Setenv("TODOIST_BASE_URL", server.URL)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	before := helpTreeState(t, dir)
	for _, args := range [][]string{{"todai"}, {"task", "complet"}, {"agent", "schedule", "pritn"}, {"completion", "instal"}, {"task", "complete", "--no-input"}} {
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 2 || out != "" {
			t.Errorf("%v: %d %q %q", args, code, out, errOut)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid command made %d requests", requests.Load())
	}
	if after := helpTreeState(t, dir); !reflect.DeepEqual(before, after) {
		t.Fatal("invalid command changed files")
	}
}

func TestHelpKeepsCommandTokenBoundaries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, args := range [][]string{{"help", "task complete"}, {"help", " task", "complete"}, {"help", "task", " complete"}, {"completion", "INSTALL", "--help"}} {
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 2 || out != "" || errOut == "" {
			t.Errorf("%v: exit=%d stdout=%q stderr=%q", args, code, out, errOut)
		}
	}
	// Shell selectors retain their existing case-insensitive, whitespace-trimmed grammar.
	code, out, errOut := executeAuthorization(t, path, "completion", " PWSH ", "--help")
	if code != 0 || !strings.Contains(out, "todoist completion powershell") || errOut != "" {
		t.Errorf("shell alias: %d %q %q", code, out, errOut)
	}
}
