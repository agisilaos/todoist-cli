package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// commandHelp describes public help paths, not execution handlers. Dispatch remains
// authoritative; the contract tests check this catalog against its command cases.
type commandHelp struct {
	aliases  string
	group    bool
	usage    string
	flags    string
	examples string
	notes    string
	globals  string
}

func findHelpChild(parent, name string) (string, bool) {
	if parent == "completion" && name != "install" && name != "uninstall" {
		canonical := canonicalCompletionShell(name)
		switch canonical {
		case "bash", "zsh", "fish", "powershell":
			name = canonical
		}
	}
	path := name
	if parent != "" {
		path = parent + " " + name
	}
	if _, ok := commandHelpCatalog[path]; ok && !strings.Contains(name, " ") {
		return path, true
	}
	for candidate, page := range commandHelpCatalog {
		p := ""
		if i := strings.LastIndex(candidate, " "); i >= 0 {
			p = candidate[:i]
		}
		if p != parent {
			continue
		}
		for _, alias := range strings.Fields(page.aliases) {
			if alias == name {
				return candidate, true
			}
		}
	}
	return "", false
}

func resolveHelpPath(args []string) (string, error) {
	path := ""
	for _, arg := range args {
		// Options do not introduce deeper command paths. Once a leaf is reached,
		// remaining tokens are its operands, not commands to correct.
		if strings.HasPrefix(arg, "-") {
			break
		}
		next, ok := findHelpChild(path, arg)
		if !ok {
			return "", unknownCommand(path, arg)
		}
		path = next
		if !commandHelpCatalog[path].group {
			break
		}
	}
	return path, nil
}

func printLeafHelp(out io.Writer, path string, page commandHelp) {
	fmt.Fprintln(out, "Usage:")
	for _, usage := range strings.Split(page.usage, "\n") {
		fmt.Fprintln(out, "  "+strings.TrimSpace("todoist "+path+" "+usage))
	}
	if page.aliases != "" {
		fmt.Fprintln(out, "\nAliases: "+page.aliases)
	}
	fmt.Fprintln(out, "\nFlags:")
	if page.flags == "" {
		fmt.Fprintln(out, "  No command-specific flags.")
	} else {
		fmt.Fprintln(out, page.flags)
	}
	fmt.Fprintln(out, "\nGlobal flags:")
	fmt.Fprintln(out, "  -h, --help             Show help without running the command")
	if page.globals != "" {
		fmt.Fprintln(out, page.globals)
	}
	fmt.Fprintln(out, "  See 'todoist --help' for all global flags and output modes.")
	if page.notes != "" {
		fmt.Fprintln(out, "\nNotes:\n"+page.notes)
	}
	fmt.Fprintln(out, "\nExamples:\n"+page.examples)
}

type unknownCommandError struct{ parent, input, message string }

func (e *unknownCommandError) Error() string { return e.message }

func unknownCommand(parent, input string) error {
	label := parent
	if i := strings.LastIndex(label, " "); i >= 0 {
		label = label[i+1:]
	}
	message := fmt.Sprintf("unknown command: %s", input)
	if label != "" {
		message = fmt.Sprintf("unknown %s subcommand: %s", label, input)
	}
	return &CodeError{Code: exitUsage, Err: &unknownCommandError{parent: parent, input: input, message: message}}
}

func humanCommandHints(opts GlobalOptions) bool {
	return !opts.JSON && !opts.NDJSON && !opts.Plain && !opts.IDsOnly && !opts.QuietJSON
}

func printCommandHints(out io.Writer, parent, input string) {
	matches := commandSuggestions(parent, input)
	if len(matches) > 0 {
		quoted := make([]string, len(matches))
		for i, match := range matches {
			quoted[i] = "'todoist " + match + "'"
		}
		fmt.Fprintln(out, "Did you mean "+strings.Join(quoted, " or ")+"?")
	}
	fmt.Fprintf(out, "Run '%s --help' for available commands.\n", strings.TrimSpace("todoist "+parent))
}

func commandSuggestions(parent, input string) []string {
	// Bound both the usefulness and the work for arbitrary user input.
	length := utf8.RuneCountInString(input)
	if length < 3 || length > 64 || strings.HasPrefix(input, "-") {
		return nil
	}
	threshold := 1
	if length >= 6 {
		threshold = 2
	}
	best := threshold + 1
	var matches []string
	for path, page := range commandHelpCatalog {
		p, name := "", path
		if i := strings.LastIndex(path, " "); i >= 0 {
			p, name = path[:i], path[i+1:]
		}
		if p != parent {
			continue
		}
		distance := commandEditDistance(input, name)
		for _, alias := range strings.Fields(page.aliases) {
			if d := commandEditDistance(input, alias); d < distance {
				distance = d
			}
		}
		if distance > threshold || distance > best {
			continue
		}
		if distance < best {
			best, matches = distance, nil
		}
		matches = append(matches, path)
	}
	if len(matches) > 3 {
		return nil
	}
	sort.Strings(matches)
	return matches
}

// Optimal string alignment distance counts an adjacent transposition as one edit.
func commandEditDistance(left, right string) int {
	a, b := []rune(left), []rune(right)
	rows := make([][]int, len(a)+1)
	for i := range rows {
		rows[i] = make([]int, len(b)+1)
		rows[i][0] = i
	}
	for j := range rows[0] {
		rows[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			rows[i][j] = min(rows[i-1][j]+1, rows[i][j-1]+1, rows[i-1][j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				rows[i][j] = min(rows[i][j], rows[i-2][j-2]+1)
			}
		}
	}
	return rows[len(a)][len(b)]
}
