package skillinstall

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testLocation(t *testing.T, target, scope string) Location {
	t.Helper()
	directory := targetDirectory(target)
	return Location{Target: target, Scope: scope, Path: filepath.Join(t.TempDir(), scope+" anchor with spaces", directory, "skills", "todoist-cli")}
}

func testBundle(version string) Bundle {
	return Bundle{Version: version, Files: map[string][]byte{
		"SKILL.md":               []byte("---\nname: todoist-cli\ndescription: Use Todoist safely.\n---\nSkill " + version + "\n"),
		"references/commands.md": []byte("Commands " + version + "\n"),
		"references/workflow.md": []byte("Curated workflow\n"),
	}}
}

func nextBundle() Bundle {
	bundle := testBundle("v2")
	delete(bundle.Files, "references/workflow.md")
	bundle.Files["references/new.md"] = []byte("New documented command\n")
	return bundle
}

func writeTestFile(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func errorCode(t *testing.T, err error, code string) *Error {
	t.Helper()
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != code {
		t.Fatalf("expected %s, received %#v", code, err)
	}
	return typed
}

func mustInstall(t *testing.T, location Location, bundle Bundle) Result {
	t.Helper()
	result, err := Install(location, bundle)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) && name == root {
			return nil
		}
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			relative, err := filepath.Rel(root, name)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			result[filepath.ToSlash(relative)] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestLifecycleTargetsScopesAndSpaces(t *testing.T) {
	for _, target := range Targets() {
		for _, scope := range []string{"local", "global"} {
			t.Run(target.Name+"/"+scope, func(t *testing.T) {
				location := testLocation(t, target.Name, scope)
				state, err := Inspect(location)
				if err != nil || state.Installed || state.Modified == nil || state.Missing == nil {
					t.Fatalf("absent state: %#v, %v", state, err)
				}
				unrelated := filepath.Join(location.Path, "references", "user-notes.md")
				writeTestFile(t, unrelated, []byte("User notes\n"))
				v1 := testBundle("v1")
				result := mustInstall(t, location, v1)
				if result.Status != "installed" || result.Path == "" || result.Files == nil || result.Retained == nil {
					t.Fatalf("install result: %#v", result)
				}
				first := tree(t, location.Path)
				stamp := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
				if err := os.Chtimes(filepath.Join(location.Path, "SKILL.md"), stamp, stamp); err != nil {
					t.Fatal(err)
				}
				result = mustInstall(t, location, v1)
				if result.Status != "unchanged" || !reflect.DeepEqual(first, tree(t, location.Path)) {
					t.Fatal("repeated installation changed content")
				}
				info, err := os.Stat(filepath.Join(location.Path, "SKILL.md"))
				if err != nil || !info.ModTime().Equal(stamp) {
					t.Fatalf("repeated installation rewrote a file: %v", err)
				}
				state, err = Inspect(location)
				if err != nil || !state.Installed || state.Version != "v1" || len(state.Modified)+len(state.Missing) != 0 {
					t.Fatalf("installed state: %#v, %v", state, err)
				}
				result, err = Update(location, nextBundle(), UpdateOptions{})
				if err != nil || result.Status != "updated" {
					t.Fatalf("update: %#v, %v", result, err)
				}
				if _, err := os.Stat(filepath.Join(location.Path, "references", "workflow.md")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("update retained obsolete owned file")
				}
				second := tree(t, location.Path)
				result, err = Update(location, nextBundle(), UpdateOptions{})
				if err != nil || result.Status != "unchanged" || !reflect.DeepEqual(second, tree(t, location.Path)) {
					t.Fatal("repeated update changed content")
				}
				_, err = Install(location, v1)
				errorCode(t, err, "SKILL_CONFLICT")
				if !reflect.DeepEqual(second, tree(t, location.Path)) {
					t.Fatal("installation downgraded an existing package")
				}
				result, err = Uninstall(location, UninstallOptions{})
				if err != nil || result.Status != "uninstalled" || !reflect.DeepEqual(result.Retained, []string{"references/user-notes.md"}) {
					t.Fatalf("uninstall: %#v, %v", result, err)
				}
				if got := string(readTestFile(t, unrelated)); got != "User notes\n" {
					t.Fatal("uninstall changed unrelated content")
				}
				if _, err := os.Stat(filepath.Join(location.Path, ManifestName)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("uninstall retained management manifest")
				}
				result, err = Uninstall(location, UninstallOptions{})
				if err != nil || result.Status != "unchanged" {
					t.Fatalf("repeated uninstall: %#v, %v", result, err)
				}
			})
		}
	}
}

func TestModifiedAndMissingProtectionAndBackup(t *testing.T) {
	location := testLocation(t, "codex", "local")
	mustInstall(t, location, testBundle("v1"))
	writeTestFile(t, filepath.Join(location.Path, "SKILL.md"), []byte("User's customized skill\n"))
	if err := os.Remove(filepath.Join(location.Path, "references", "workflow.md")); err != nil {
		t.Fatal(err)
	}
	state, err := Inspect(location)
	if err != nil || !reflect.DeepEqual(state.Modified, []string{"SKILL.md"}) || !reflect.DeepEqual(state.Missing, []string{"references/workflow.md"}) {
		t.Fatalf("modified state: %#v, %v", state, err)
	}
	before := tree(t, location.Path)
	_, err = Update(location, nextBundle(), UpdateOptions{})
	typed := errorCode(t, err, "SKILL_MODIFIED")
	if !reflect.DeepEqual(typed.Files, []string{"SKILL.md", "references/workflow.md"}) || !reflect.DeepEqual(before, tree(t, location.Path)) {
		t.Fatal("default update did not preserve every change")
	}
	_, err = Install(location, testBundle("v1"))
	errorCode(t, err, "SKILL_MODIFIED")
	_, err = Uninstall(location, UninstallOptions{})
	errorCode(t, err, "SKILL_MODIFIED")
	if !reflect.DeepEqual(before, tree(t, location.Path)) {
		t.Fatal("default lifecycle operation changed customized files")
	}
	result, err := Update(location, nextBundle(), UpdateOptions{Backup: true})
	if err != nil || result.Status != "updated" || result.BackupPath == "" {
		t.Fatalf("backed up update: %#v, %v", result, err)
	}
	if filepath.Dir(result.BackupPath) != filepath.Dir(filepath.Dir(result.Path)) {
		t.Fatal("backup is inside the loadable skills root")
	}
	if got := string(readTestFile(t, filepath.Join(result.BackupPath, "SKILL.md"))); got != before["SKILL.md"] {
		t.Fatal("backup lost customized bytes")
	}
	if got := string(readTestFile(t, filepath.Join(result.BackupPath, ManifestName))); got != before[ManifestName] {
		t.Fatal("backup lost original ownership evidence")
	}
	writeTestFile(t, filepath.Join(location.Path, "SKILL.md"), []byte("Second customization\n"))
	again, err := Update(location, nextBundle(), UpdateOptions{Backup: true})
	if err != nil || again.BackupPath == result.BackupPath {
		t.Fatalf("backup reused an existing directory: %#v, %v", again, err)
	}
	if got := string(readTestFile(t, filepath.Join(result.BackupPath, "SKILL.md"))); got != before["SKILL.md"] {
		t.Fatal("second backup overwrote the first")
	}
	state, err = Inspect(location)
	if err != nil || len(state.Modified)+len(state.Missing) != 0 {
		t.Fatalf("replacement is not clean: %#v, %v", state, err)
	}
}

func TestKeepModifiedUninstallAndUnrelatedFiles(t *testing.T) {
	location := testLocation(t, "claude-code", "global")
	mustInstall(t, location, testBundle("v1"))
	writeTestFile(t, filepath.Join(location.Path, "SKILL.md"), []byte("Customized loading instructions\n"))
	writeTestFile(t, filepath.Join(location.Path, "notes.txt"), []byte("unrelated"))
	if err := os.Remove(filepath.Join(location.Path, "references", "workflow.md")); err != nil {
		t.Fatal(err)
	}
	result, err := Uninstall(location, UninstallOptions{KeepModified: true})
	if err != nil || result.Status != "uninstalled" || !reflect.DeepEqual(result.Retained, []string{"SKILL.md", "notes.txt"}) {
		t.Fatalf("keep modified: %#v, %v", result, err)
	}
	if got := tree(t, location.Path); !reflect.DeepEqual(got, map[string]string{"SKILL.md": "Customized loading instructions\n", "notes.txt": "unrelated"}) {
		t.Fatalf("retained content: %#v", got)
	}
	result, err = Uninstall(location, UninstallOptions{KeepModified: true})
	if err != nil || result.Status != "unchanged" {
		t.Fatalf("repeat detached uninstall: %#v, %v", result, err)
	}
	_, err = Install(location, testBundle("v1"))
	errorCode(t, err, "SKILL_CONFLICT")
}

func TestUpdateBackupRestoresMissingFilesWithoutInventingOriginals(t *testing.T) {
	location := testLocation(t, "claude-code", "local")
	bundle := testBundle("v1")
	mustInstall(t, location, bundle)
	if err := os.Remove(filepath.Join(location.Path, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	_, err := Update(location, bundle, UpdateOptions{})
	errorCode(t, err, "SKILL_MODIFIED")
	result, err := Update(location, bundle, UpdateOptions{Backup: true})
	if err != nil || result.Status != "updated" || result.BackupPath != "" {
		t.Fatalf("restore missing file: %#v, %v", result, err)
	}
	if got := readTestFile(t, filepath.Join(location.Path, "SKILL.md")); !reflect.DeepEqual(got, bundle.Files["SKILL.md"]) {
		t.Fatal("explicit update did not restore the missing owned file")
	}
}

func TestUnownedFilesCannotBeAdopted(t *testing.T) {
	for _, installFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "install", true: "update"}[installFirst], func(t *testing.T) {
			location := testLocation(t, "codex", "local")
			if installFirst {
				mustInstall(t, location, testBundle("v1"))
			}
			bundle := nextBundle()
			conflicting := "SKILL.md"
			if installFirst {
				conflicting = "references/new.md"
			}
			writeTestFile(t, filepath.Join(location.Path, conflicting), bundle.Files[conflicting])
			before := tree(t, location.Path)
			var err error
			if installFirst {
				_, err = Update(location, bundle, UpdateOptions{Backup: true})
			} else {
				_, err = Install(location, bundle)
			}
			errorCode(t, err, "SKILL_CONFLICT")
			if !reflect.DeepEqual(before, tree(t, location.Path)) {
				t.Fatal("unowned content was changed or adopted")
			}
		})
	}
}

func TestAbsentOperations(t *testing.T) {
	location := testLocation(t, "codex", "global")
	result, err := Uninstall(location, UninstallOptions{})
	if err != nil || result.Status != "unchanged" {
		t.Fatalf("absent uninstall: %#v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Dir(location.Path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("absent uninstall created target directories")
	}
	_, err = Update(location, testBundle("v1"), UpdateOptions{})
	errorCode(t, err, "SKILL_NOT_INSTALLED")
}

func TestMalformedManifestNeverChangesFiles(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"version":   func(m map[string]any) { m["format_version"] = 2 },
		"target":    func(m map[string]any) { m["target"] = "claude-code" },
		"scope":     func(m map[string]any) { m["scope"] = "global" },
		"digest":    func(m map[string]any) { m["package_digest"] = strings.Repeat("0", 64) },
		"traversal": func(m map[string]any) { m["files"].(map[string]any)["../private.txt"] = strings.Repeat("0", 64) },
		"reserved":  func(m map[string]any) { m["files"].(map[string]any)[ManifestName] = strings.Repeat("0", 64) },
		"hash":      func(m map[string]any) { m["files"].(map[string]any)["SKILL.md"] = "not a hash" },
		"unknown":   func(m map[string]any) { m["unexpected"] = true },
		"empty":     func(m map[string]any) { m["files"] = map[string]any{} },
		"case":      func(m map[string]any) { m["files"].(map[string]any)["skill.md"] = strings.Repeat("0", 64) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			location := testLocation(t, "codex", "local")
			mustInstall(t, location, testBundle("v1"))
			var value map[string]any
			if err := json.Unmarshal(readTestFile(t, filepath.Join(location.Path, ManifestName)), &value); err != nil {
				t.Fatal(err)
			}
			mutate(value)
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(location.Path, ManifestName), data)
			before := tree(t, location.Path)
			_, err = Update(location, nextBundle(), UpdateOptions{Backup: true})
			errorCode(t, err, "SKILL_MANIFEST_INVALID")
			_, err = Uninstall(location, UninstallOptions{KeepModified: true})
			errorCode(t, err, "SKILL_MANIFEST_INVALID")
			if !reflect.DeepEqual(before, tree(t, location.Path)) {
				t.Fatal("malformed ownership evidence permitted changes")
			}
		})
	}
	for _, contents := range []string{"{", "null", "{}{}", `{"format_version":1,"format_version":2}`, `{"files":{"SKILL.md":"a","SKILL.md":"b"}}`} {
		location := testLocation(t, "codex", "local")
		writeTestFile(t, filepath.Join(location.Path, ManifestName), []byte(contents))
		_, err := Inspect(location)
		errorCode(t, err, "SKILL_MANIFEST_INVALID")
	}
}

func TestConcurrentWriterIsRejected(t *testing.T) {
	location := testLocation(t, "codex", "local")
	started, finish := make(chan struct{}), make(chan struct{})
	m := newManager()
	rename := m.fs.rename
	m.fs.rename = func(from, to string) error {
		if filepath.Base(to) == "SKILL.md" {
			close(started)
			<-finish
		}
		return rename(from, to)
	}
	done := make(chan error, 1)
	go func() {
		_, err := m.install(location, testBundle("v1"), false, UpdateOptions{})
		done <- err
	}()
	<-started
	_, err := Install(location, testBundle("v1"))
	errorCode(t, err, "SKILL_BUSY")
	close(finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := Install(location, testBundle("v1")); err != nil {
		t.Fatal("lock not released after successful operation:", err)
	}
}
