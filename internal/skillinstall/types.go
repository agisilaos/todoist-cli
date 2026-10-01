// Package skillinstall manages an explicit, owned installation of a bundled
// agent skill. It does not discover agents, retrieve credentials, or use a network.
package skillinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

const ManifestName = ".todoist-skill.json"

type Target struct {
	Name      string `json:"name"`
	Directory string `json:"directory"`
}

func Targets() []Target {
	return []Target{{Name: "codex", Directory: ".agents"}, {Name: "claude-code", Directory: ".claude"}}
}

type Location struct {
	Target string `json:"target"`
	Scope  string `json:"scope"`
	Path   string `json:"path"`
}

type Bundle struct {
	Version string
	Files   map[string][]byte
}

type UpdateOptions struct{ Backup bool }
type UninstallOptions struct{ KeepModified bool }

type Result struct {
	Operation     string   `json:"operation"`
	Status        string   `json:"status"`
	Target        string   `json:"target"`
	Scope         string   `json:"scope"`
	Path          string   `json:"path"`
	PackageDigest string   `json:"package_digest,omitempty"`
	Version       string   `json:"version,omitempty"`
	Files         []string `json:"files"`
	Retained      []string `json:"retained"`
	BackupPath    string   `json:"backup_path,omitempty"`
}

type State struct {
	Target        string   `json:"target"`
	Scope         string   `json:"scope"`
	Path          string   `json:"path"`
	Installed     bool     `json:"installed"`
	PackageDigest string   `json:"package_digest,omitempty"`
	Version       string   `json:"version,omitempty"`
	Modified      []string `json:"modified"`
	Missing       []string `json:"missing"`
}

type Error struct {
	Code             string   `json:"code"`
	Message          string   `json:"message"`
	Path             string   `json:"path,omitempty"`
	Files            []string `json:"files,omitempty"`
	Committed        bool     `json:"committed"`
	RecoveryRequired bool     `json:"recovery_required"`
	BackupPath       string   `json:"backup_path,omitempty"`
	RecoveryPath     string   `json:"recovery_path,omitempty"`
	LockPath         string   `json:"lock_path,omitempty"`
	err              error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.err }

func fileDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Digest identifies the sorted relative paths and their content hashes. It is
// independent of build version and machine-specific destination paths.
func Digest(bundle Bundle) (string, error) {
	files, err := bundleHashes(bundle)
	if err != nil {
		return "", err
	}
	return hashesDigest(files), nil
}

func hashesDigest(files map[string]string) string {
	h := sha256.New()
	for _, name := range sortedKeys(files) {
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00", name, files[name])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sortedKeys[V any](values map[string]V) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func bundleHashes(bundle Bundle) (map[string]string, error) {
	if bundle.Version == "" {
		return nil, invalidBundle("The skill package must have a version.")
	}
	if len(bundle.Files["SKILL.md"]) == 0 {
		return nil, invalidBundle("The skill package must contain a nonempty SKILL.md.")
	}
	if err := validateFileNames(sortedKeys(bundle.Files)); err != nil {
		return nil, err
	}
	files := make(map[string]string, len(bundle.Files))
	for name, data := range bundle.Files {
		files[name] = fileDigest(data)
	}
	return files, nil
}

func invalidBundle(message string) *Error {
	return &Error{Code: "SKILL_PACKAGE_INVALID", Message: message}
}
