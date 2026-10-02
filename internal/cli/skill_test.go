package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/output"
	"github.com/agisilaos/todoist-cli/internal/skillinstall"
)

func runSkill(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, diagnostic bytes.Buffer
	code := executeTest(args, &out, &diagnostic)
	return code, out.String(), diagnostic.String()
}

func skillResultFrom(t *testing.T, data string) skillinstall.Result {
	t.Helper()
	var result skillinstall.Result
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("result JSON: %v: %s", err, data)
	}
	return result
}

func TestSkillLifecycleExplicitPlacements(t *testing.T) {
	for _, target := range skillinstall.Targets() {
		for _, scope := range []string{"local", "global"} {
			t.Run(target.Name+"/"+scope, func(t *testing.T) {
				base := filepath.Join(t.TempDir(), "Project or Home With Spaces")
				path := filepath.Join(base, target.Directory, "skills", "todoist-cli")
				args := []string{target.Name, "--scope", scope, "--path", path, "--no-input", "--json"}
				code, out, diagnostic := runSkill(t, append([]string{"skill", "install"}, args...)...)
				if code != 0 || diagnostic != "" || skillResultFrom(t, out).Status != "installed" {
					t.Fatalf("install: %d %s %s", code, out, diagnostic)
				}
				installedPath := skillResultFrom(t, out).Path
				entry, err := os.ReadFile(filepath.Join(installedPath, "SKILL.md"))
				if err != nil || !bytes.Contains(entry, []byte("name: todoist-cli")) {
					t.Fatalf("discoverable entry: %v", err)
				}
				manifest := filepath.Join(installedPath, skillinstall.ManifestName)
				before, err := os.Stat(manifest)
				if err != nil {
					t.Fatal(err)
				}
				unrelated := filepath.Join(installedPath, "my-notes.txt")
				if err := os.WriteFile(unrelated, []byte("Keep my private customization"), 0600); err != nil {
					t.Fatal(err)
				}
				for _, operation := range []string{"install", "update"} {
					code, out, diagnostic = runSkill(t, append([]string{"skill", operation}, args...)...)
					if code != 0 || diagnostic != "" || skillResultFrom(t, out).Status != "unchanged" {
						t.Fatalf("%s again: %d %s %s", operation, code, out, diagnostic)
					}
				}
				after, err := os.Stat(manifest)
				if err != nil || !after.ModTime().Equal(before.ModTime()) {
					t.Fatalf("no-op rewrote manifest: %v", err)
				}
				code, out, diagnostic = runSkill(t, append([]string{"skill", "list"}, args...)...)
				var inventory []skillListItem
				if err := json.Unmarshal([]byte(out), &inventory); err != nil || code != 0 || diagnostic != "" || len(inventory) != 1 || inventory[0].Status != "installed" {
					t.Fatalf("list: %d %s %s (%v)", code, out, diagnostic, err)
				}
				code, out, diagnostic = runSkill(t, append([]string{"skill", "uninstall"}, args...)...)
				result := skillResultFrom(t, out)
				if code != 0 || diagnostic != "" || result.Status != "uninstalled" || !reflect.DeepEqual(result.Retained, []string{"my-notes.txt"}) {
					t.Fatalf("uninstall: %d %s %s", code, out, diagnostic)
				}
				if _, err := os.Stat(filepath.Join(installedPath, "SKILL.md")); !os.IsNotExist(err) {
					t.Fatalf("owned entry still present: %v", err)
				}
				if data, err := os.ReadFile(unrelated); err != nil || string(data) != "Keep my private customization" {
					t.Fatalf("unrelated file altered: %q %v", data, err)
				}
				code, out, diagnostic = runSkill(t, append([]string{"skill", "uninstall"}, args...)...)
				if code != 0 || diagnostic != "" || skillResultFrom(t, out).Status != "unchanged" {
					t.Fatalf("uninstall again: %d %s %s", code, out, diagnostic)
				}
			})
		}
	}
}

func TestSkillCustomizationRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".agents", "skills", "todoist-cli")
	args := []string{"codex", "--scope", "local", "--path", path, "--json", "--quiet-json", "--no-input"}
	code, out, diagnostic := runSkill(t, append([]string{"skill", "install"}, args...)...)
	if code != 0 {
		t.Fatalf("install: %s", diagnostic)
	}
	path = skillResultFrom(t, out).Path
	entry := filepath.Join(path, "SKILL.md")
	edited := []byte("My edited skill instructions\n")
	if err := os.WriteFile(entry, edited, 0600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"update", "uninstall"} {
		code, out, diagnostic = runSkill(t, append([]string{"skill", operation}, args...)...)
		if code != exitConflict || out != "" || !strings.Contains(diagnostic, `"code":"SKILL_MODIFIED"`) {
			t.Fatalf("edit protection %s: %d %s %s", operation, code, out, diagnostic)
		}
		if data, err := os.ReadFile(entry); err != nil || !bytes.Equal(data, edited) {
			t.Fatalf("edit changed on conflict: %s %v", data, err)
		}
	}
	code, out, diagnostic = runSkill(t, append(append([]string{"skill", "update"}, args...), "--backup")...)
	result := skillResultFrom(t, out)
	if code != 0 || diagnostic != "" || result.BackupPath == "" || result.Status != "updated" {
		t.Fatalf("backup update: %d %s %s", code, out, diagnostic)
	}
	if data, err := os.ReadFile(filepath.Join(result.BackupPath, "SKILL.md")); err != nil || !bytes.Equal(data, edited) {
		t.Fatalf("backup lost original: %s %v", data, err)
	}
	if err := os.WriteFile(entry, edited, 0600); err != nil {
		t.Fatal(err)
	}
	code, out, diagnostic = runSkill(t, append(append([]string{"skill", "uninstall"}, args...), "--keep-modified")...)
	if code != 0 || diagnostic != "" || !reflect.DeepEqual(skillResultFrom(t, out).Retained, []string{"SKILL.md"}) {
		t.Fatalf("keep modified: %d %s %s", code, out, diagnostic)
	}
	if data, err := os.ReadFile(entry); err != nil || !bytes.Equal(data, edited) {
		t.Fatalf("retained edit altered: %s %v", data, err)
	}
	code, out, diagnostic = runSkill(t, append([]string{"skill", "list"}, args...)...)
	var inventory []skillListItem
	if err := json.Unmarshal([]byte(out), &inventory); err != nil || code != 0 || diagnostic != "" || len(inventory) != 1 || inventory[0].Status != "conflict" {
		t.Fatalf("retained entry inspection: %d %s %s", code, out, diagnostic)
	}
}

func TestSkillLifecycleDoesNotLoadTodoistState(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(broken, []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TODOIST_CONFIG", broken)
	t.Setenv("TODOIST_TOKEN", "secret-must-not-appear")
	progress := filepath.Join(t.TempDir(), "progress.jsonl")
	path := filepath.Join(t.TempDir(), ".claude", "skills", "todoist-cli")
	code, out, diagnostic := runSkill(t, "skill", "install", "claude-code", "--scope", "global", "--path", path, "--progress-jsonl", progress, "--json")
	if code != 0 || diagnostic != "" || strings.Contains(out, "secret-must-not-appear") {
		t.Fatalf("local operation accessed unrelated state: %d %s %s", code, out, diagnostic)
	}
	if _, err := os.Stat(progress); !os.IsNotExist(err) {
		t.Fatalf("local lifecycle created Todoist progress file: %v", err)
	}
}

func TestSkillLifecycleMachineErrors(t *testing.T) {
	for _, mode := range []string{"--json", "--ndjson"} {
		for _, test := range []struct {
			name string
			args []string
			code int
		}{
			{"required", []string{"skill", "install", "codex"}, exitUsage},
			{"relative", []string{"skill", "install", "codex", "--scope", "local", "--path", ".agents/skills/todoist-cli"}, exitUsage},
			{"unknown", []string{"skill", "invent"}, exitUsage},
			{"global parsing", []string{"skill", "list", "--timeout", "invalid"}, exitUsage},
			{"force", []string{"skill", "list", "--force"}, exitUsage},
			{"dry run", []string{"skill", "list", "--dry-run"}, exitUsage},
			{"path argument literal", []string{"skill", "install", "codex", "--scope", "local", "--path", "--force"}, exitUsage},
		} {
			t.Run(mode+"/"+test.name, func(t *testing.T) {
				code, out, diagnostic := runSkill(t, append(test.args, mode, "--quiet-json")...)
				var envelope struct {
					Error   string         `json:"error"`
					Code    string         `json:"code"`
					Meta    map[string]any `json:"meta"`
					Details struct {
						Files            []string `json:"files"`
						Committed        bool     `json:"committed"`
						RecoveryRequired bool     `json:"recovery_required"`
					} `json:"details"`
				}
				if err := json.Unmarshal([]byte(diagnostic), &envelope); err != nil || code != test.code || out != "" || envelope.Error == "" || !strings.HasPrefix(envelope.Code, "SKILL_") || envelope.Meta == nil || envelope.Details.Files == nil || envelope.Details.Committed {
					t.Fatalf("machine error: %d %s %s (%v)", code, out, diagnostic, err)
				}
				if strings.Count(diagnostic, "\n") != 1 {
					t.Fatalf("compact error spans lines: %s", diagnostic)
				}
			})
		}
	}
}

func TestSkillBashCompletion(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash unavailable")
	}
	path := filepath.Join(t.TempDir(), "completion.sh")
	if err := os.WriteFile(path, []byte(bashCompletion), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ words, want string }{
		{"skill ''", "install"}, {"skill install co", "codex"},
		{"skill update codex --ba", "--backup"}, {"skill install codex --scope gl", "global"},
	} {
		script := "source \"$1\"\nCOMP_WORDS=(todoist " + test.words + ")\nCOMP_CWORD=$((${#COMP_WORDS[@]} - 1))\n_todoist\nprintf '%s\\n' \"${COMPREPLY[@]}\""
		data, err := exec.Command(bash, "--noprofile", "--norc", "-c", script, "test", path).CombinedOutput()
		if err != nil || !strings.Contains(string(data), test.want+"\n") {
			t.Fatalf("completion %s: %v %s", test.words, err, data)
		}
	}
}

func TestSkillZshCompletionFlagsMatchOperation(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh unavailable")
	}
	path := filepath.Join(t.TempDir(), "completion.zsh")
	if err := os.WriteFile(path, []byte(zshCompletion), 0600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"install", "list", "update", "uninstall"} {
		// Capture the options passed to zsh's completion helper after it has
		// selected the skill command context.
		script := "_arguments() { print -r -- \"$@\"; }\nwords=(skill \"$2\" codex '')\nsource \"$1\""
		data, err := exec.Command(zsh, "-f", "-c", script, "test", path, operation).CombinedOutput()
		if err != nil {
			t.Fatalf("completion %s: %v %s", operation, err, data)
		}
		for flag, expected := range map[string]bool{"--backup[": operation == "update", "--keep-modified[": operation == "uninstall"} {
			if strings.Contains(string(data), flag) != expected {
				t.Errorf("completion %s: incorrect %s option in %s", operation, flag, data)
			}
		}
	}
}

type skillFailingOutput struct{ writesBeforeFailure int }

func (w *skillFailingOutput) Write(data []byte) (int, error) {
	if w.writesBeforeFailure == 0 {
		return 0, errors.New("output unavailable")
	}
	w.writesBeforeFailure--
	return len(data), nil
}

func TestSkillResultOutputFailureReportsCommittedState(t *testing.T) {
	for _, mode := range []string{"--json", "--ndjson"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".agents", "skills", "todoist-cli")
			var diagnostic bytes.Buffer
			code := executeTest([]string{"skill", "install", "codex", "--scope", "local", "--path", path, mode}, &skillFailingOutput{}, &diagnostic)
			var envelope struct {
				Code    string `json:"code"`
				Details struct {
					Committed bool   `json:"committed"`
					Path      string `json:"path"`
				} `json:"details"`
			}
			if err := json.Unmarshal(diagnostic.Bytes(), &envelope); err != nil || code != exitError || envelope.Code != "SKILL_IO" || !envelope.Details.Committed || envelope.Details.Path == "" {
				t.Fatalf("lost committed evidence: %d %s (%v)", code, diagnostic.String(), err)
			}
			if _, err := os.Stat(filepath.Join(envelope.Details.Path, skillinstall.ManifestName)); err != nil {
				t.Fatalf("expected committed installation: %v", err)
			}
		})
	}
	for _, result := range []skillinstall.Result{
		{Status: "updated", Path: "/installation", BackupPath: "/backup"},
		{Status: "uninstalled", Path: "/installation", Retained: []string{"my-notes.txt"}},
	} {
		err := writeSkillResult(&Context{Mode: output.ModePlain, Stdout: &skillFailingOutput{writesBeforeFailure: 1}}, result)
		var typed *skillinstall.Error
		if !errors.As(err, &typed) || !typed.Committed {
			t.Fatalf("lost detail-output failure: %v", err)
		}
	}
}

func TestSkillHumanRecoveryReportsPaths(t *testing.T) {
	var diagnostic bytes.Buffer
	err := &skillinstall.Error{Code: "SKILL_RECOVERY_REQUIRED", Message: "Inspect retained copies before retrying.", Path: "/project with spaces/.agents/skills/todoist-cli", RecoveryPath: "/project with spaces/.agents/.todoist-cli-stage-unique", BackupPath: "/project with spaces/.agents/todoist-cli-backup-unique", LockPath: "/project with spaces/.agents/.todoist-cli.lock", Files: []string{"SKILL.md"}, Committed: true, RecoveryRequired: true}
	writeError(&Context{Stderr: &diagnostic, Mode: output.ModePlain}, err)
	for _, expected := range []string{err.Code, strconv.Quote(err.Path), strconv.Quote(err.RecoveryPath), strconv.Quote(err.BackupPath), strconv.Quote(err.LockPath), strconv.Quote("SKILL.md"), "File changes committed", "Recovery required"} {
		if !strings.Contains(diagnostic.String(), expected) {
			t.Errorf("missing actionable %q in %s", expected, diagnostic.String())
		}
	}
}
