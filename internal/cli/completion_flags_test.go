package cli

import (
	"regexp"
	"strings"
	"testing"
)

// Read only the literal completion tables, independently of their runtime union.
// Parser registrations remain the authority for supported flag names.
func powerShellFlagTable(t *testing.T, name string) map[string][]string {
	t.Helper()
	start := "$" + name + " = @{\n"
	_, tail, ok := strings.Cut(powerShellCompletion, start)
	if !ok {
		t.Fatalf("missing PowerShell table %s", name)
	}
	body, _, ok := strings.Cut(tail, "\n}")
	if !ok {
		t.Fatalf("unterminated PowerShell table %s", name)
	}
	rows := regexp.MustCompile(`(?m)^    '([^']*)' = @\(([^\n]*)\)$`)
	words := regexp.MustCompile(`'([^']+)'`)
	result := map[string][]string{}
	for _, row := range rows.FindAllStringSubmatch(body, -1) {
		if _, exists := result[row[1]]; exists {
			t.Fatalf("duplicate %s context %q", name, row[1])
		}
		result[row[1]] = nil
		for _, word := range words.FindAllStringSubmatch(row[2], -1) {
			result[row[1]] = append(result[row[1]], word[1])
		}
	}
	if len(result) == 0 {
		t.Fatalf("empty PowerShell table %s", name)
	}
	return result
}

func TestPowerShellFlagPartitionsMatchRegistrations(t *testing.T) {
	switches := powerShellFlagTable(t, "todoistSwitchFlags")
	values := powerShellFlagTable(t, "todoistValueFlags")
	registered, globals := documentedCommandFlags(t)
	seen := map[string]map[string]bool{}
	for _, table := range []map[string][]string{switches, values} {
		for path, flags := range table {
			if path != "" {
				if _, ok := commandHelpCatalog[path]; !ok {
					t.Errorf("unknown flag completion path %q", path)
				}
				if _, ok := registered[path]; !ok {
					t.Errorf("no parser registration inventory for %q", path)
				}
			}
			if seen[path] == nil {
				seen[path] = map[string]bool{}
			}
			for _, flag := range flags {
				if seen[path][flag] {
					t.Errorf("duplicate flag %s for %q; keep switch/value tables disjoint", flag, path)
				}
				seen[path][flag] = true
				name := strings.TrimPrefix(flag, "--")
				if name == flag || (!registered[path][name] && !globals[name]) {
					t.Errorf("unsupported PowerShell flag %s for %q", flag, path)
				}
			}
		}
	}
}
