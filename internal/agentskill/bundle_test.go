package agentskill

import (
	"bytes"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"
)

func TestFilesAreIndependent(t *testing.T) {
	first := Files()
	original := append([]byte(nil), first["SKILL.md"]...)
	if len(original) == 0 {
		t.Fatal("missing skill entrypoint")
	}
	first["SKILL.md"][0] = '!'
	delete(first, "references/plans.md")
	second := Files()
	if !bytes.Equal(second["SKILL.md"], original) || len(second["references/plans.md"]) == 0 {
		t.Fatal("caller mutation changed the embedded bundle")
	}
}

func TestBundlePathsAndReferences(t *testing.T) {
	files := Files()
	entry := string(files["SKILL.md"])
	if !strings.HasPrefix(entry, "---\nname: "+Name+"\ndescription: ") {
		t.Fatal("entrypoint lacks discoverable skill frontmatter")
	}
	links := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	for name, data := range files {
		if !fs.ValidPath(name) || len(data) == 0 {
			t.Errorf("invalid bundle file %q", name)
		}
		for _, match := range links.FindAllStringSubmatch(string(data), -1) {
			target, _, _ := strings.Cut(match[1], "#")
			if strings.Contains(target, ":") || target == "" {
				continue
			}
			resolved := path.Clean(path.Join(path.Dir(name), target))
			if !fs.ValidPath(resolved) || files[resolved] == nil {
				t.Errorf("%s links outside the bundle or to missing file %q", name, target)
			}
		}
	}
}
