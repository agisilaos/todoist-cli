package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// SelectionPersistence is the atomic file boundary used by profile selection.
// Callers serialize writers before invoking SetDefaultProfile.
type SelectionPersistence interface {
	Read(string) ([]byte, error)
	Write(string, []byte) error
}

type SelectionDisk struct{}

func (SelectionDisk) Read(path string) ([]byte, error) { return os.ReadFile(path) }

func (SelectionDisk) Write(path string, data []byte) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".todoist-config-stage-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	return syncSelectionDirectory(filepath.Dir(path))
}

// SelectionError never includes configuration contents or raw filesystem errors.
type SelectionError struct{ Uncertain bool }

func (e *SelectionError) Error() string {
	if e.Uncertain {
		return "Could not confirm the saved default profile; the saved choice may have changed. Inspect 'todoist profile current' before retrying."
	}
	return "Could not read configuration; the default profile was not changed. Preserve the file and fix it before retrying."
}

// SetDefaultProfile changes only the selection field in the user configuration.
// In particular it never saves the merged project or environment configuration.
func SetDefaultProfile(path, profile string, files SelectionPersistence) error {
	if files == nil {
		files = SelectionDisk{}
	}
	data, err := files.Read(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return &SelectionError{}
	}
	fields := map[string]json.RawMessage{}
	if err == nil {
		var known Config
		if json.Unmarshal(data, &known) != nil || json.Unmarshal(data, &fields) != nil || fields == nil {
			return &SelectionError{}
		}
	}
	removeKnownJSONFields(fields, "default_profile")
	fields["default_profile"], _ = json.Marshal(profile)
	updated, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return &SelectionError{}
	}
	if files.Write(path, updated) != nil {
		// A writer may fail after replacement, for example during directory sync.
		// Do not promise that the old selection remained active.
		return &SelectionError{Uncertain: true}
	}
	return nil
}
