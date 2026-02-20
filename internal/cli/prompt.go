package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/output"

	"io"
)

func readLine(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return scanner.Text(), nil
}

func readAllTrim(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func confirm(ctx *Context, prompt string) (bool, error) {
	if ctx.Global.NoInput {
		return false, fmt.Errorf("confirmation required; re-run with --force")
	}
	if !isTTYReader(ctx.Stdin) {
		return false, fmt.Errorf("confirmation required; re-run with --force")
	}
	fmt.Fprintf(ctx.Stderr, "%s [y/N]: ", prompt)
	line, err := readLine(ctx.Stdin)
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes", nil
}

func isTTYReader(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return output.IsTTY(f)
}
