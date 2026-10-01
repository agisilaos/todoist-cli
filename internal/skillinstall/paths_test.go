package skillinstall

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRejectUnsafeLocationsBeforeWrites(t *testing.T) {
	good := testLocation(t, "codex", "local")
	cases := []Location{
		{Target: "cursor", Scope: "local", Path: good.Path},
		{Target: "codex", Scope: "detected", Path: good.Path},
		{Target: "codex", Scope: "local", Path: ".agents/skills/todoist-cli"},
		{Target: "codex", Scope: "local", Path: filepath.Join(t.TempDir(), ".claude", "skills", "todoist-cli")},
		{Target: "claude-code", Scope: "global", Path: good.Path},
		{Target: "codex", Scope: "global", Path: filepath.Join(t.TempDir(), ".agents", "skills", "different-name")},
		{Target: "codex", Scope: "local", Path: filepath.Dir(filepath.Dir(filepath.Dir(good.Path))) + "/parent/../.agents/skills/todoist-cli"},
		{Target: "codex", Scope: "local", Path: good.Path + "\x00"},
	}
	for _, location := range cases {
		_, err := Install(location, testBundle("v1"))
		errorCode(t, err, "SKILL_PATH_INVALID")
	}
	if _, err := os.Stat(filepath.Dir(good.Path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid destination created agent directories")
	}
}

func symlink(t *testing.T, old, new string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(new), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(old, new); err != nil {
		t.Skip("symlink creation unavailable:", err)
	}
}

func TestCanonicalizeSymlinkAncestorsAboveAgentDirectory(t *testing.T) {
	base := t.TempDir()
	actual, alias := filepath.Join(base, "actual anchor"), filepath.Join(base, "alias anchor")
	if err := os.MkdirAll(actual, 0o755); err != nil {
		t.Fatal(err)
	}
	symlink(t, actual, alias)
	location := Location{Target: "codex", Scope: "local", Path: filepath.Join(alias, "project", ".agents", "skills", "todoist-cli")}
	result := mustInstall(t, location, testBundle("v1"))
	canonical, err := filepath.EvalSymlinks(actual)
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != filepath.Join(canonical, "project", ".agents", "skills", "todoist-cli") {
		t.Fatalf("canonical destination: %s", result.Path)
	}
	result, err = Uninstall(location, UninstallOptions{})
	if err != nil || result.Status != "uninstalled" {
		t.Fatalf("uninstall using ancestor alias: %#v, %v", result, err)
	}
}

func TestRejectSymlinksInsideManagedDirectories(t *testing.T) {
	for _, segment := range []string{"agent", "skills", "root", "nested", "file", "manifest"} {
		t.Run(segment, func(t *testing.T) {
			location := testLocation(t, "codex", "local")
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.MkdirAll(outside, 0o755); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(outside, "private.txt"), []byte("unrelated private content"))
			link := filepath.Dir(filepath.Dir(location.Path))
			switch segment {
			case "skills":
				link = filepath.Dir(location.Path)
			case "root":
				link = location.Path
			case "nested":
				link = filepath.Join(location.Path, "references")
			case "file":
				link = filepath.Join(location.Path, "SKILL.md")
				outside = filepath.Join(outside, "private.txt")
			case "manifest":
				link = filepath.Join(location.Path, ManifestName)
				outside = filepath.Join(outside, "private.txt")
			}
			symlink(t, outside, link)
			_, err := Install(location, testBundle("v1"))
			errorCode(t, err, "SKILL_PATH_INVALID")
			if segment != "file" && segment != "manifest" {
				if got := string(readTestFile(t, filepath.Join(outside, "private.txt"))); got != "unrelated private content" {
					t.Fatal("operation changed content through a symlink")
				}
			}
		})
	}
}

func TestUnrelatedSymlinkIsRetainedWithoutFollowingIt(t *testing.T) {
	location := testLocation(t, "claude-code", "local")
	mustInstall(t, location, testBundle("v1"))
	outside := filepath.Join(t.TempDir(), "private.txt")
	writeTestFile(t, outside, []byte("private content"))
	symlink(t, outside, filepath.Join(location.Path, "user-link"))
	result, err := Uninstall(location, UninstallOptions{})
	if err != nil || !reflect.DeepEqual(result.Retained, []string{"user-link"}) {
		t.Fatalf("unrelated symlink retained: %#v, %v", result, err)
	}
	if got := string(readTestFile(t, outside)); got != "private content" {
		t.Fatal("uninstall touched unrelated symlink target")
	}
}

func TestRejectUnsafePackagePaths(t *testing.T) {
	for _, name := range []string{"../private", "foo/../../private", "/private", "foo\\private", "foo\x00bar", "foo\nbar", ".", "foo//bar", ManifestName, ManifestName + "/private", ".TODOIST-SKILL.JSON", "SKILL.md", "skill.md", "references.", "references ", "NUL.txt", "references/COM1", "references/a:b"} {
		t.Run(name, func(t *testing.T) {
			bundle := testBundle("v1")
			bundle.Files[name] = []byte("bad")
			if name == "SKILL.md" {
				bundle.Files["SKILL.md/child"] = []byte("bad")
			}
			_, err := Install(testLocation(t, "codex", "local"), bundle)
			errorCode(t, err, "SKILL_PACKAGE_INVALID")
		})
	}
	for _, bundle := range []Bundle{{Version: "", Files: testBundle("v1").Files}, {Version: "v1", Files: map[string][]byte{}}, {Version: "v1", Files: map[string][]byte{"SKILL.md": {}}}} {
		_, err := Digest(bundle)
		errorCode(t, err, "SKILL_PACKAGE_INVALID")
	}
}

func TestPackageDigestIsDeterministicAndContentSensitive(t *testing.T) {
	a := Bundle{Version: "v1", Files: map[string][]byte{"SKILL.md": []byte("skill"), "a.md": []byte("a"), "b.md": []byte("b")}}
	b := Bundle{Version: "another version", Files: map[string][]byte{"b.md": []byte("b"), "SKILL.md": []byte("skill"), "a.md": []byte("a")}}
	first, err := Digest(a)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Digest(b)
	if err != nil || first != second {
		t.Fatalf("nondeterministic digest: %q %q, %v", first, second, err)
	}
	b.Files["a.md"] = []byte("changed")
	second, err = Digest(b)
	if err != nil || first == second {
		t.Fatal("digest did not detect changed content")
	}
}
