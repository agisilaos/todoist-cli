package cli

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestCommandMetadataStructure(t *testing.T) {
	paths, names, orders := map[string]bool{}, map[string]bool{}, map[int]bool{}
	safeName := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	for _, command := range commandCatalog {
		if paths[command.path] {
			t.Errorf("duplicate path %s", command.path)
		}
		paths[command.path] = true
		parent, name := commandParentAndName(command.path)
		for _, word := range append([]string{name}, strings.Fields(command.aliases)...) {
			if !safeName.MatchString(word) {
				t.Errorf("unsafe shell inventory name %q", word)
			}
			key := parent + "/" + word
			if names[key] {
				t.Errorf("duplicate sibling name/alias %s", key)
			}
			names[key] = true
		}
		if parent == "" {
			if command.summary == "" || command.section == "" || command.rootHelpOrder <= 0 || orders[command.rootHelpOrder] {
				t.Errorf("invalid root help metadata: %+v", command)
			}
			orders[command.rootHelpOrder] = true
		} else if page, ok := commandHelpCatalog[parent]; !ok || !page.group {
			t.Errorf("missing group for %s", command.path)
		}
		if parent != "" && !command.group && leafHelpPages[command.path].examples == "" {
			t.Errorf("missing detailed help for %s", command.path)
		}
		if command.group && len(commandNames(command.path)) == 0 {
			t.Errorf("empty group %s", command.path)
		}
	}
	for path, page := range leafHelpPages {
		if !paths[path] {
			t.Errorf("orphaned detailed help %s", path)
		}
		if page.aliases != "" || page.group {
			t.Errorf("discovery metadata belongs in commandCatalog: %s", path)
		}
	}
}

// This deliberately reads execution source, never the discovery catalog. It is
// a bounded contract for these routers, not a general Go control-flow analyzer.
func dispatchedCommandInventory(t *testing.T) (map[string]bool, map[string]string) {
	t.Helper()
	routers := map[string]string{"dispatch": "", "authCommand": "auth", "taskCommand": "task", "projectCommand": "project", "filterCommand": "filter", "workspaceCommand": "workspace", "sectionCommand": "section", "labelCommand": "label", "commentCommand": "comment", "reminderCommand": "reminder", "notificationCommand": "notification", "statsCommand": "stats", "settingsCommand": "settings", "agentCommand": "agent", "agentSchedule": "agent schedule", "inboxCommand": "inbox", "completionScript": "completion"}
	commands, aliases, seen := map[string]bool{}, map[string]string{}, map[string]bool{}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
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
			parent, router := routers[fn.Name.Name]
			if !router && fn.Name.Name != "completionCommand" {
				continue
			}
			seen[fn.Name.Name] = true
			switches, branches := 0, 0
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if sw, ok := n.(*ast.SwitchStmt); ok {
					switches++
					// Only direct cases of the router's switch count; nested business logic
					// must not accidentally expand the command inventory.
					for _, stmt := range sw.Body.List {
						clause := stmt.(*ast.CaseClause)
						for _, expr := range clause.List {
							child := sourceString(expr)
							if child == "" || (child == "help" && parent != "") || strings.HasPrefix(child, "-") {
								continue
							}
							commands[strings.TrimSpace(parent+" "+child)] = true
						}
					}
					// Still inspect the tag for inline canonicalSubcommand alias maps.
					if call, ok := sw.Tag.(*ast.CallExpr); ok {
						readRouterAliases(call, parent, aliases)
					}
					return false
				}
				if call, ok := n.(*ast.CallExpr); ok {
					readRouterAliases(call, parent, aliases)
				}
				if fn.Name.Name == "completionCommand" {
					if branch, ok := n.(*ast.IfStmt); ok {
						if eq, ok := branch.Cond.(*ast.BinaryExpr); ok && eq.Op == token.EQL {
							if index, ok := eq.X.(*ast.IndexExpr); ok {
								ident, isIdent := index.X.(*ast.Ident)
								zero, isZero := index.Index.(*ast.BasicLit)
								if isIdent && ident.Name == "args" && isZero && zero.Value == "0" {
									if child := sourceString(eq.Y); child != "" {
										commands["completion "+child] = true
										branches++
									}
								}
							}
						}
					}
				}
				return true
			})
			if router && switches != 1 {
				t.Errorf("router %s: expected one command switch, got %d; update source contract", fn.Name.Name, switches)
			}
			if !router && branches != 2 {
				t.Errorf("completionCommand: expected two operation branches, got %d; update source contract", branches)
			}
		}
	}
	for name := range routers {
		if !seen[name] {
			t.Errorf("missing execution router %s", name)
		}
	}
	if !seen["completionCommand"] {
		t.Error("missing completionCommand router")
	}
	// Shell normalization is separately implemented, including case/whitespace.
	for _, input := range []string{"pwsh", " PWSH "} {
		if canonicalCompletionShell(input) != "powershell" {
			t.Errorf("shell alias changed: %q", input)
		}
	}
	aliases["completion pwsh"] = "completion powershell"
	return commands, aliases
}

func readRouterAliases(call *ast.CallExpr, parent string, aliases map[string]string) {
	name, ok := call.Fun.(*ast.Ident)
	if !ok || name.Name != "canonicalSubcommand" || len(call.Args) != 2 {
		return
	}
	literal, ok := call.Args[1].(*ast.CompositeLit)
	if !ok {
		return
	}
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		alias, target := sourceString(pair.Key), sourceString(pair.Value)
		if alias != "" && target != "" {
			aliases[parent+" "+alias] = parent + " " + target
		}
	}
}

func discoveryContractErrors(commands map[string]bool, aliases map[string]string, catalog []commandMetadata) []string {
	canonical, advertisedAliases := map[string]bool{}, map[string]string{}
	for _, command := range catalog {
		canonical[command.path] = true
		parent, _ := commandParentAndName(command.path)
		for _, alias := range strings.Fields(command.aliases) {
			advertisedAliases[strings.TrimSpace(parent+" "+alias)] = command.path
		}
	}
	var errors []string
	for path := range commands {
		if !canonical[path] {
			errors = append(errors, "missing executable command: "+path)
		}
	}
	for path := range canonical {
		if !commands[path] {
			errors = append(errors, "undispatched discovery command: "+path)
		}
	}
	for alias, target := range aliases {
		if advertisedAliases[alias] != target {
			errors = append(errors, "missing/misdirected executable alias: "+alias)
		}
	}
	for alias, target := range advertisedAliases {
		if aliases[alias] != target {
			errors = append(errors, "unsupported discovery alias: "+alias)
		}
	}
	return errors
}

func TestHelpCatalogCoversDispatch(t *testing.T) {
	commands, aliases := dispatchedCommandInventory(t)
	for _, problem := range discoveryContractErrors(commands, aliases, commandCatalog) {
		t.Error(problem)
	}
}

func TestDiscoveryContractDetectsSharedOmission(t *testing.T) {
	commands, aliases := dispatchedCommandInventory(t)
	catalog := append([]commandMetadata(nil), commandCatalog...)
	for i, command := range catalog {
		if command.path == "review" {
			catalog = append(catalog[:i], catalog[i+1:]...)
			break
		}
	}
	problems := strings.Join(discoveryContractErrors(commands, aliases, catalog), "\n")
	if !strings.Contains(problems, "missing executable command: review") {
		t.Fatalf("shared help/completion omission was not detected: %s", problems)
	}
}

func TestCompletionInventoryRendering(t *testing.T) {
	for shell, script := range map[string]string{"bash": bashCompletion, "zsh": zshCompletion, "fish": fishCompletion, "powershell": powerShellCompletion} {
		if strings.Contains(script, "{{") {
			t.Errorf("%s has unresolved inventory slots", shell)
		}
		if !strings.Contains(script, "review") {
			t.Errorf("%s omits review", shell)
		}
	}
	// Check each shell context, not just whether a name appears somewhere among
	// flags or examples. Shells keep their existing depth and traversal behavior.
	for _, parent := range []string{"auth", "task", "filter", "project", "workspace", "section", "label", "comment", "reminder", "notification", "stats", "settings", "inbox", "agent"} {
		names := strings.Join(commandNames(parent), " ")
		for shell, template := range map[string]string{"bash": bashCompletionTemplate, "zsh": zshCompletionTemplate, "fish": fishCompletionTemplate} {
			if !strings.Contains(template, "{{commands:"+parent+"}}") {
				t.Errorf("%s missing %s inventory slot", shell, parent)
			}
			if !strings.Contains(renderCommandInventory(template), names) {
				t.Errorf("%s missing %s inventory", shell, parent)
			}
		}
	}
	for _, command := range commandCatalog {
		if command.group {
			want := fmt.Sprintf("'%s' = %s", command.path, powerShellWords(commandNames(command.path)))
			if !strings.Contains(powerShellCompletion, want) {
				t.Errorf("PowerShell missing %s", want)
			}
		}
	}
	if !strings.Contains(zshCompletion, "2:command:("+strings.Join(commandNames(""), " ")+")") {
		t.Error("zsh help inventory differs from root")
	}
}

func TestBashCommandInventories(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash unavailable")
	}
	script := filepath.Join(t.TempDir(), "completion.bash")
	if err := os.WriteFile(script, []byte(bashCompletion), 0600); err != nil {
		t.Fatal(err)
	}
	for _, parent := range []string{"", "auth", "task", "project", "filter", "workspace", "section", "label", "comment", "reminder", "notification", "stats", "settings", "inbox", "agent"} {
		t.Run(parent, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := `source "$1"
COMP_WORDS=(todoist)
if [[ -n "$2" ]]; then COMP_WORDS+=("$2"); fi
COMP_WORDS+=("")
COMP_CWORD=$((${#COMP_WORDS[@]} - 1))
_todoist
printf '%s\n' "${COMPREPLY[@]}"`
			output, err := exec.CommandContext(ctx, bash, "--noprofile", "--norc", "-c", command, "test", script, parent).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v: %s", err, output)
			}
			var got []string
			for _, word := range strings.Fields(string(output)) {
				if !strings.HasPrefix(word, "-") {
					got = append(got, word)
				}
			}
			if want := commandNames(parent); !reflect.DeepEqual(got, want) {
				t.Errorf("got %v; want %v", got, want)
			}
		})
	}
}
