package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
)

func TestTableWidthConfig(t *testing.T) {
	ctx := &Context{Config: config.Config{TableWidth: 99}}
	if w := tableWidth(ctx); w != 99 {
		t.Fatalf("expected 99, got %d", w)
	}
}

func TestSetRequestIDVerboseWritesStderr(t *testing.T) {
	var errBuf bytes.Buffer
	ctx := &Context{
		Stderr: &errBuf,
		Global: GlobalOptions{Verbose: true},
	}
	setRequestID(ctx, "req-123")
	if ctx.RequestID != "req-123" {
		t.Fatalf("expected request id set")
	}
	if !strings.Contains(errBuf.String(), "request_id=req-123") {
		t.Fatalf("expected verbose request id log, got %q", errBuf.String())
	}
}

func TestParseFlagSetInterspersed(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	var name string
	var yes bool
	fs.StringVar(&name, "name", "", "Name")
	fs.BoolVar(&yes, "yes", false, "Yes")
	if err := parseFlagSetInterspersed(fs, []string{"target-ref", "--name", "alice", "--yes"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if name != "alice" || !yes {
		t.Fatalf("unexpected parsed values: name=%q yes=%v", name, yes)
	}
	args := fs.Args()
	if len(args) != 1 || args[0] != "target-ref" {
		t.Fatalf("unexpected positional args: %#v", args)
	}
}

func TestParseFlagSetInterspersedUnknownFlag(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	var name string
	fs.StringVar(&name, "name", "", "Name")
	if err := parseFlagSetInterspersed(fs, []string{"--unknown"}); err == nil {
		t.Fatalf("expected unknown flag error")
	}
}
