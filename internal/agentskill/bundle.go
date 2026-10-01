// Package agentskill contains the maintained, offline Todoist agent skill.
package agentskill

import (
	"embed"
	"io/fs"
	"strings"
)

const Name = "todoist-cli"

//go:embed bundle
var bundle embed.FS

// Files returns independent copies of the bundled files, keyed by their
// slash-separated paths relative to the installation directory.
func Files() map[string][]byte {
	files := make(map[string][]byte)
	err := fs.WalkDir(bundle, "bundle", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := bundle.ReadFile(path)
		if err != nil {
			return err
		}
		files[strings.TrimPrefix(path, "bundle/")] = data
		return nil
	})
	if err != nil {
		panic("read embedded agent skill: " + err.Error())
	}
	return files
}
