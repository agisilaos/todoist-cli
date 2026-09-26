package cli

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Inspect flag registrations instead of executing documentation examples: examples
// can authenticate, delete remote data, install completions, or invoke a planner.
func documentedCommandFlags(t *testing.T) (map[string]map[string]bool, map[string]bool) {
	t.Helper()
	commands := map[string]map[string]bool{}
	globals := map[string]bool{}
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
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			command := ""
			flags := map[string]bool{"help": true, "h": true}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if fn.Name.Name == "parseGlobalFlags" {
					value := sourceString(node)
					if len(value) > 2 && strings.HasPrefix(value, "--") && !strings.ContainsAny(value, " \n") {
						globals[strings.TrimSuffix(strings.TrimPrefix(value, "--"), "=")] = true
					}
				}
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if name, ok := call.Fun.(*ast.Ident); ok && len(call.Args) > 0 {
					switch name.Name {
					case "newFlagSet":
						command = sourceString(call.Args[0])
					case "requireIDArg", "requireEntityIDArg":
						command = sourceString(call.Args[0])
						flags["id"] = true
					}
				}
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) >= 2 {
					if receiver, ok := selector.X.(*ast.Ident); ok && receiver.Name == "fs" {
						switch selector.Sel.Name {
						case "StringVar", "BoolVar", "IntVar", "Var":
							flags[sourceString(call.Args[1])] = true
						}
					}
				}
				return true
			})
			if command != "" {
				commands[command] = flags
			}
		}
	}
	// These entry points delegate to another command's parser.
	for alias, target := range map[string]string{
		"planner": "agent planner", "completed": "task list",
		"project create": "project add", "settings": "settings view",
	} {
		commands[alias] = commands[target]
	}
	for _, resource := range []string{"task", "project", "section", "label", "comment", "filter"} {
		for alias, target := range map[string]string{"ls": "list", "rm": "delete", "del": "delete"} {
			commands[resource+" "+alias] = commands[resource+" "+target]
		}
	}
	commands["task show"] = commands["task view"]
	return commands, globals
}

func sourceString(node ast.Node) string {
	literal, ok := node.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return ""
	}
	value, _ := strconv.Unquote(literal.Value)
	return value
}

func TestDocumentationFlags(t *testing.T) {
	commands, globals := documentedCommandFlags(t)
	flagPattern := regexp.MustCompile(`--([a-z][a-z0-9-]*)`)
	paths := []string{"../../README.md", "../../docs/SPEC.md"}
	helpPaths, err := filepath.Glob("../../docs/help/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, helpPaths...)
	checked := 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		inFence := false
		for index, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inFence = !inFence
				continue
			}
			// Markdown prose is not CLI syntax. Inspect fenced lines and inline code.
			fragments := []string{line}
			if strings.HasSuffix(path, ".md") && !inFence {
				fragments = nil
				parts := strings.Split(line, "`")
				for i := 1; i < len(parts); i += 2 {
					fragments = append(fragments, parts[i])
				}
			}
			for _, fragment := range fragments {
				start := strings.Index(fragment, "todoist ")
				if start < 0 {
					continue
				}
				invocation := strings.TrimSpace(fragment[start+len("todoist "):])
				command := ""
				for candidate := range commands {
					if (invocation == candidate || strings.HasPrefix(invocation, candidate+" ")) && len(candidate) > len(command) {
						command = candidate
					}
				}
				if command == "" {
					continue // Group help and commands without a flag parser.
				}
				checked++
				for _, match := range flagPattern.FindAllStringSubmatch(invocation, -1) {
					if !globals[match[1]] && !commands[command][match[1]] {
						t.Errorf("%s:%d: %s does not register --%s", path, index+1, command, match[1])
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no documented command invocations checked")
	}
	t.Logf("checked flags in %d documented invocations", checked)
}

func TestDocumentationGlobalFlags(t *testing.T) {
	_, globals := documentedCommandFlags(t)
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	var help bytes.Buffer
	printRootHelp(&help)
	for name, section := range map[string]struct{ text, start, end string }{
		"README": {string(readme), "Global flags", "Flag parsing notes:"},
		"help":   {help.String(), "Global flags:", "Examples:"},
	} {
		_, content, found := strings.Cut(section.text, section.start)
		if !found {
			t.Fatalf("%s missing global flags section", name)
		}
		content, _, found = strings.Cut(content, section.end)
		if !found {
			t.Fatalf("%s missing end of global flags section", name)
		}
		documented := map[string]bool{}
		for _, match := range regexp.MustCompile(`--([a-z][a-z0-9-]*)`).FindAllStringSubmatch(content, -1) {
			documented[match[1]] = true
			if !globals[match[1]] {
				t.Errorf("%s documents unknown global flag --%s", name, match[1])
			}
		}
		for flag := range globals {
			if !documented[flag] {
				t.Errorf("%s missing global flag --%s", name, flag)
			}
		}
	}
}

func TestDocumentationHelpCoverage(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "dispatch.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("../../scripts/help-snapshots.txt")
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expression := range clause.List {
			command := sourceString(expression)
			if command == "help" {
				continue // Root help is covered by --help.
			}
			if !strings.Contains(string(manifest), "\t"+command+" --help\n") {
				t.Errorf("add %s --help to scripts/help-snapshots.txt and run scripts/update-help.sh", command)
			}
		}
		return true
	})
}
