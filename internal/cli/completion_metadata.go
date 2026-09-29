package cli

import (
	"fmt"
	"strings"
)

// Only inventory slots are generated. Shell control flow, flag/value completion,
// and escaping of user input stay in the shell-owned templates.
func renderCommandInventory(template string) string {
	parents := []string{""}
	for _, command := range commandCatalog {
		if command.group {
			parents = append(parents, command.path)
		}
	}
	for _, parent := range parents {
		template = strings.ReplaceAll(template, "{{commands:"+parent+"}}", strings.Join(commandNames(parent), " "))
	}
	return template
}

func powerShellWords(words []string) string {
	quoted := make([]string, len(words))
	for i, word := range words {
		quoted[i] = "'" + strings.ReplaceAll(word, "'", "''") + "'"
	}
	return "@(" + strings.Join(quoted, ", ") + ")"
}

func renderPowerShellInventory(template string) string {
	var out strings.Builder
	out.WriteString("$todoistCommands = @{\n")
	fmt.Fprintf(&out, "    '' = %s\n", powerShellWords(commandNames("")))
	for _, command := range commandCatalog {
		if command.group {
			fmt.Fprintf(&out, "    '%s' = %s\n", command.path, powerShellWords(commandNames(command.path)))
		}
	}
	// These are shell arguments and a help topic, not additional execution paths.
	var shells []string
	for _, name := range commandNames("completion") {
		if name != "install" && name != "uninstall" {
			shells = append(shells, name)
		}
	}
	for _, operation := range []string{"install", "uninstall"} {
		fmt.Fprintf(&out, "    'completion %s' = %s\n", operation, powerShellWords(shells))
	}
	fmt.Fprintf(&out, "    'help' = %s\n}\n\n", powerShellWords(commandNames("")))
	out.WriteString("$todoistAliases = @{\n")
	for _, command := range commandCatalog {
		parent, _ := commandParentAndName(command.path)
		for _, alias := range strings.Fields(command.aliases) {
			path := strings.TrimSpace(parent + " " + alias)
			fmt.Fprintf(&out, "    '%s' = '%s'\n", path, command.path)
		}
	}
	out.WriteString("}\n")
	return strings.ReplaceAll(template, "{{powershell-commands}}", out.String())
}
