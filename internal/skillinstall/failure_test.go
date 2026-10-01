package skillinstall

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var errInjected = errors.New("injected filesystem failure")

func TestInstallFailureRollsBackNewFilesAndPreservesUnrelatedContent(t *testing.T) {
	location := testLocation(t, "codex", "local")
	writeTestFile(t, filepath.Join(location.Path, "user.txt"), []byte("unrelated"))
	before := tree(t, location.Path)
	m := newManager()
	rename := m.fs.rename
	m.fs.rename = func(from, to string) error {
		if filepath.Base(to) == "commands.md" {
			return errInjected
		}
		return rename(from, to)
	}
	_, err := m.install(location, testBundle("v1"), false, UpdateOptions{})
	errorCode(t, err, "SKILL_IO")
	if !reflect.DeepEqual(before, tree(t, location.Path)) {
		t.Fatal("failed installation did not restore the unowned destination")
	}
	state, err := Inspect(location)
	if err != nil || state.Installed {
		t.Fatalf("failed installation left managed state: %#v, %v", state, err)
	}
}

func TestUpdateFailuresRestoreOwnedAndUnrelatedContent(t *testing.T) {
	for _, boundary := range []string{"stage", "replace", "remove-obsolete", "publish-manifest"} {
		t.Run(boundary, func(t *testing.T) {
			location := testLocation(t, "codex", "local")
			mustInstall(t, location, testBundle("v1"))
			writeTestFile(t, filepath.Join(location.Path, "user.txt"), []byte("unrelated"))
			before := tree(t, location.Path)
			m := newManager()
			write, rename, remove := m.fs.writeFile, m.fs.rename, m.fs.remove
			failed := false
			m.fs.writeFile = func(name string, data []byte, mode os.FileMode) error {
				if boundary == "stage" && strings.Contains(name, ".todoist-cli-stage-") && !failed {
					failed = true
					return errInjected
				}
				return write(name, data, mode)
			}
			m.fs.rename = func(from, to string) error {
				if !failed && strings.Contains(filepath.ToSlash(from), "/new/") && (boundary == "replace" && filepath.Base(to) == "commands.md" || boundary == "publish-manifest" && filepath.Base(to) == ManifestName) {
					failed = true
					return errInjected
				}
				return rename(from, to)
			}
			m.fs.remove = func(name string) error {
				if boundary == "remove-obsolete" && filepath.Base(name) == "workflow.md" && !failed {
					failed = true
					return errInjected
				}
				return remove(name)
			}
			_, err := m.install(location, nextBundle(), true, UpdateOptions{})
			typed := errorCode(t, err, "SKILL_IO")
			if !failed || typed.Committed || typed.RecoveryRequired || !errors.Is(err, errInjected) {
				t.Fatalf("failure classification: %#v", typed)
			}
			if !reflect.DeepEqual(before, tree(t, location.Path)) {
				t.Fatal("rollback did not restore owned files or preserve unrelated content")
			}
			state, err := Inspect(location)
			if err != nil || len(state.Modified)+len(state.Missing) != 0 || state.Version != "v1" {
				t.Fatalf("rolled-back state: %#v, %v", state, err)
			}
		})
	}
}

func TestUninstallFailuresRestoreFilesAndManagement(t *testing.T) {
	for _, failName := range []string{"commands.md", ManifestName} {
		t.Run(failName, func(t *testing.T) {
			location := testLocation(t, "claude-code", "global")
			mustInstall(t, location, testBundle("v1"))
			before := tree(t, location.Path)
			m := newManager()
			remove := m.fs.remove
			failed := false
			m.fs.remove = func(name string) error {
				if filepath.Base(name) == failName && !failed {
					failed = true
					return errInjected
				}
				return remove(name)
			}
			_, err := m.uninstall(location, UninstallOptions{})
			errorCode(t, err, "SKILL_IO")
			if !failed || !reflect.DeepEqual(before, tree(t, location.Path)) {
				t.Fatal("failed uninstall did not restore prior state")
			}
		})
	}
}

func TestFailedRollbackPreservesRecoveryCopies(t *testing.T) {
	location := testLocation(t, "codex", "local")
	mustInstall(t, location, testBundle("v1"))
	old := readTestFile(t, filepath.Join(location.Path, "SKILL.md"))
	m := newManager()
	rename := m.fs.rename
	m.fs.rename = func(from, to string) error {
		if strings.Contains(filepath.ToSlash(from), "/old/") || filepath.Base(to) == "commands.md" {
			return errInjected
		}
		return rename(from, to)
	}
	_, err := m.install(location, nextBundle(), true, UpdateOptions{})
	typed := errorCode(t, err, "SKILL_RECOVERY_REQUIRED")
	if !typed.RecoveryRequired || typed.Committed || typed.RecoveryPath == "" {
		t.Fatalf("recovery error: %#v", typed)
	}
	if got := readTestFile(t, filepath.Join(typed.RecoveryPath, "old", "SKILL.md")); !reflect.DeepEqual(got, old) {
		t.Fatal("failed rollback lost original content")
	}
	state, err := Inspect(location)
	if err != nil || !reflect.DeepEqual(state.Modified, []string{"SKILL.md"}) {
		t.Fatalf("partial state is not visible to inspection: %#v, %v", state, err)
	}
}

func TestConcurrentEditIsPreservedInsteadOfRolledBack(t *testing.T) {
	location := testLocation(t, "codex", "local")
	mustInstall(t, location, testBundle("v1"))
	m := newManager()
	rename := m.fs.rename
	m.fs.rename = func(from, to string) error {
		if strings.Contains(filepath.ToSlash(from), "/new/") && filepath.Base(to) == "commands.md" {
			writeTestFile(t, filepath.Join(location.Path, "SKILL.md"), []byte("concurrent user edit"))
			return errInjected
		}
		return rename(from, to)
	}
	_, err := m.install(location, nextBundle(), true, UpdateOptions{})
	typed := errorCode(t, err, "SKILL_RECOVERY_REQUIRED")
	if !typed.RecoveryRequired || typed.RecoveryPath == "" || string(readTestFile(t, filepath.Join(location.Path, "SKILL.md"))) != "concurrent user edit" {
		t.Fatal("rollback overwrote a concurrent edit")
	}
}

func TestConcurrentSymlinkIsPreservedDuringRollback(t *testing.T) {
	location := testLocation(t, "codex", "local")
	mustInstall(t, location, testBundle("v1"))
	outside := filepath.Join(t.TempDir(), "outside.txt")
	writeTestFile(t, outside, []byte("unrelated"))
	m := newManager()
	rename := m.fs.rename
	m.fs.rename = func(from, to string) error {
		if strings.Contains(filepath.ToSlash(from), "/new/") && filepath.Base(to) == "commands.md" {
			if err := os.Remove(filepath.Join(location.Path, "SKILL.md")); err != nil {
				t.Fatal(err)
			}
			symlink(t, outside, filepath.Join(location.Path, "SKILL.md"))
			return errInjected
		}
		return rename(from, to)
	}
	_, err := m.install(location, nextBundle(), true, UpdateOptions{})
	errorCode(t, err, "SKILL_RECOVERY_REQUIRED")
	if got := string(readTestFile(t, outside)); got != "unrelated" {
		t.Fatal("rollback modified a symlink target")
	}
	info, err := os.Lstat(filepath.Join(location.Path, "SKILL.md"))
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("rollback replaced a concurrent symlink")
	}
}

func TestPostCommitCleanupReportsCommittedRecovery(t *testing.T) {
	location := testLocation(t, "claude-code", "local")
	m := newManager()
	m.fs.removeAll = func(string) error { return errInjected }
	result, err := m.install(location, testBundle("v1"), false, UpdateOptions{})
	typed := errorCode(t, err, "SKILL_RECOVERY_REQUIRED")
	if !typed.Committed || !typed.RecoveryRequired || result.Status != "installed" {
		t.Fatalf("post-commit cleanup status: %#v, %#v", result, typed)
	}
	state, err := Inspect(location)
	if err != nil || !state.Installed || len(state.Modified)+len(state.Missing) != 0 {
		t.Fatalf("committed state: %#v, %v", state, err)
	}
}

func TestBackupFailurePreservesInstalledCustomization(t *testing.T) {
	for _, cleanupFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "cleanup succeeds", true: "cleanup fails"}[cleanupFails], func(t *testing.T) {
			location := testLocation(t, "codex", "local")
			mustInstall(t, location, testBundle("v1"))
			writeTestFile(t, filepath.Join(location.Path, "SKILL.md"), []byte("user customization"))
			before := tree(t, location.Path)
			m := newManager()
			write := m.fs.writeFile
			m.fs.writeFile = func(name string, data []byte, mode os.FileMode) error {
				if strings.Contains(name, "todoist-cli-backup-") && filepath.Base(name) == "SKILL.md" {
					return errInjected
				}
				return write(name, data, mode)
			}
			if cleanupFails {
				m.fs.removeAll = func(string) error { return errInjected }
			}
			result, err := m.install(location, nextBundle(), true, UpdateOptions{Backup: true})
			code := "SKILL_IO"
			if cleanupFails {
				code = "SKILL_RECOVERY_REQUIRED"
			}
			typed := errorCode(t, err, code)
			if typed.Committed || !reflect.DeepEqual(before, tree(t, location.Path)) {
				t.Fatal("backup failure changed installed customization")
			}
			if cleanupFails && (result.BackupPath == "" || typed.RecoveryPath == "") {
				t.Fatal("incomplete backup location not reported")
			}
		})
	}
}

func TestLockCleanupFailureIsNotHiddenByEarlierError(t *testing.T) {
	location := testLocation(t, "codex", "local")
	mustInstall(t, location, testBundle("v1"))
	writeTestFile(t, filepath.Join(location.Path, "SKILL.md"), []byte("modified"))
	m := newManager()
	remove := m.fs.remove
	m.fs.remove = func(name string) error {
		if filepath.Base(name) == ".todoist-cli.lock" {
			return errInjected
		}
		return remove(name)
	}
	_, err := m.install(location, nextBundle(), true, UpdateOptions{})
	typed := errorCode(t, err, "SKILL_RECOVERY_REQUIRED")
	if typed.Committed || typed.LockPath == "" || !typed.RecoveryRequired || typed.RecoveryPath != typed.LockPath {
		t.Fatalf("lost lock cleanup failure: %#v", typed)
	}
	if got := string(readTestFile(t, filepath.Join(location.Path, "SKILL.md"))); got != "modified" {
		t.Fatal("lock cleanup failure changed customization")
	}
}

func TestBackupPathSurvivesLockCleanupFailure(t *testing.T) {
	for _, updateFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "committed update", true: "rolled back update"}[updateFails], func(t *testing.T) {
			location := testLocation(t, "codex", "local")
			mustInstall(t, location, testBundle("v1"))
			customization := []byte("user customization")
			writeTestFile(t, filepath.Join(location.Path, "SKILL.md"), customization)
			m := newManager()
			remove, rename := m.fs.remove, m.fs.rename
			m.fs.remove = func(name string) error {
				if filepath.Base(name) == ".todoist-cli.lock" {
					return errInjected
				}
				return remove(name)
			}
			m.fs.rename = func(from, to string) error {
				if updateFails && strings.Contains(filepath.ToSlash(from), "/new/") && filepath.Base(to) == "commands.md" {
					return errInjected
				}
				return rename(from, to)
			}
			result, err := m.install(location, nextBundle(), true, UpdateOptions{Backup: true})
			typed := errorCode(t, err, "SKILL_RECOVERY_REQUIRED")
			if result.BackupPath == "" || typed.BackupPath != result.BackupPath || typed.LockPath == "" || !typed.RecoveryRequired || typed.Committed == updateFails {
				t.Fatalf("lost backup or update outcome in cleanup error: result=%#v error=%#v", result, typed)
			}
			if got := readTestFile(t, filepath.Join(typed.BackupPath, "SKILL.md")); !bytes.Equal(got, customization) {
				t.Fatal("reported backup did not preserve the exact customization")
			}
		})
	}
}
