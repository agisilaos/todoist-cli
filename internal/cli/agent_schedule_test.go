package cli

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestSchedulePreservesArgumentsWithMetacharacters(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config&reader.json")
	policyPath := filepath.Join(dir, "policy<reader>.json")
	bin := filepath.Join(dir, "capture&args")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s\\000' \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	want := []string{"--profile", "read&only", "--config", configPath, "agent", "run", "--policy", policyPath, "--instruction", `literal$HOME\%`, "--confirm", "it's-reviewed", "--dry-run", "--plan-version", "1"}
	for _, format := range []string{"launchd", "cron"} {
		t.Run(format, func(t *testing.T) {
			cron := format == "cron"
			if cron && runtime.GOOS == "windows" {
				t.Skip("cron executes through a POSIX shell")
			}
			args := []string{"--profile", "read&only", "--config", configPath, "agent", "schedule", "print", "--weekly", "sat 09:00", "--policy", policyPath, "--instruction", `literal$HOME\%`, "--confirm", "it's-reviewed", "--dry-run", "--bin", bin}
			if cron {
				args = append(args, "--cron")
			}
			var out, diagnostic bytes.Buffer
			if code := executeTest(args, &out, &diagnostic); code != 0 {
				t.Fatalf("schedule failed (%d): %s", code, diagnostic.String())
			}
			var got []string
			if cron {
				command := strings.TrimPrefix(strings.TrimSpace(out.String()), "0 9 * * 6 ")
				// Cron removes the backslash before escaped %, treats an
				// unescaped % as stdin, and preserves other backslash pairs.
				var shellCommand strings.Builder
				for i := 0; i < len(command); i++ {
					if command[i] == '%' {
						t.Fatal("generated cron entry contains an unescaped percent separator")
					}
					if command[i] == '\\' && i+1 < len(command) {
						if command[i+1] != '%' {
							shellCommand.WriteByte('\\')
						}
						i++
					}
					shellCommand.WriteByte(command[i])
				}
				result, err := exec.Command("/bin/sh", "-c", shellCommand.String()).CombinedOutput()
				if err != nil {
					t.Fatalf("generated command failed: %v: %s", err, result)
				}
				got = strings.Split(strings.TrimSuffix(string(result), "\x00"), "\x00")
			} else {
				var plist struct {
					Arguments []string `xml:"dict>array>string"`
				}
				if err := xml.Unmarshal(out.Bytes(), &plist); err != nil {
					t.Fatalf("invalid launchd plist: %v", err)
				}
				if len(plist.Arguments) == 0 || plist.Arguments[0] != bin {
					t.Fatalf("binary path changed: %q", plist.Arguments)
				}
				got = plist.Arguments[1:]
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("arguments changed:\n got %q\nwant %q", got, want)
			}
		})
	}
}

func TestCronScheduleEscapesPercentAndRejectsLineBreaks(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	for _, tc := range []struct {
		profile string
		code    int
	}{
		{"read%only", 0},
		{"read\nonly", 2},
		{"read\ronly", 2},
	} {
		var out, diagnostic bytes.Buffer
		code := executeTest([]string{"--config", filepath.Join(t.TempDir(), "config.json"), "--profile", tc.profile, "agent", "schedule", "print", "--weekly", "sat 09:00", "--instruction", "review", "--cron", "--bin", "todoist"}, &out, &diagnostic)
		if code != tc.code {
			t.Fatalf("profile %q: exit %d want %d: %s", tc.profile, code, tc.code, diagnostic.String())
		}
		if code == 0 {
			if !strings.Contains(out.String(), `--profile read'\%'only`) {
				t.Fatalf("cron percent separator was not escaped: %s", out.String())
			}
		} else if out.Len() != 0 || !strings.Contains(diagnostic.String(), "line breaks") {
			t.Fatalf("unsafe cron entry emitted: %s %s", out.String(), diagnostic.String())
		}
	}
}

func TestParseWeeklySpec(t *testing.T) {
	spec, err := parseWeeklySpec("sat 09:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Weekday != 7 || spec.Hour != 9 || spec.Minute != 30 {
		t.Fatalf("unexpected spec: %#v", spec)
	}
}

func TestCronLine(t *testing.T) {
	spec := scheduleSpec{Weekday: 7, Hour: 9, Minute: 0}
	line := cronLine(spec, "/usr/local/bin/todoist", []string{"agent", "run", "--instruction", "hello"})
	want := "0 9 * * 6 /usr/local/bin/todoist agent run --instruction hello"
	if line != want {
		t.Fatalf("unexpected cron line: %q", line)
	}
}

func TestAgentSchedulePreservesRequestedWeekday(t *testing.T) {
	// Both launchd.plist(5) and crontab(5) number Sunday as 0, Saturday as 6.
	for _, tc := range []struct {
		day     string
		weekday int
	}{{"sun", 0}, {"mon", 1}, {"tue", 2}, {"wed", 3}, {"thu", 4}, {"fri", 5}, {"sat", 6}} {
		for _, cron := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cron=%t", tc.day, cron), func(t *testing.T) {
				args := []string{"--config", filepath.Join(t.TempDir(), "config.json"), "agent", "schedule", "print", "--weekly", tc.day + " 09:30", "--instruction", "Review", "--bin", "todoist"}
				if cron {
					args = append(args, "--cron")
				}
				var out, diagnostic bytes.Buffer
				code := executeTestWithEnvironment(args, &out, &diagnostic, Environment{Getenv: func(string) string { return "" }})
				if code != 0 {
					t.Fatalf("schedule exited %d: %s", code, diagnostic.String())
				}
				if cron {
					if !strings.HasPrefix(out.String(), fmt.Sprintf("30 9 * * %d ", tc.weekday)) {
						t.Fatalf("cron weekday changed: %s", out.String())
					}
					return
				}
				var plist struct {
					Interval []int `xml:"dict>dict>integer"`
				}
				if err := xml.Unmarshal(out.Bytes(), &plist); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(plist.Interval, []int{tc.weekday, 9, 30}) {
					t.Fatalf("launchd interval = %v; requested %s 09:30 requires weekday %d", plist.Interval, tc.day, tc.weekday)
				}
			})
		}
	}
}
