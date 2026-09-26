package credentials

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Persistence permits failure injection at the durable storage boundary.
type Persistence interface {
	Read(string) ([]byte, error)
	Write(string, []byte) error
	Remove(string) error
	RemoveStaging(string) error
	Lock(context.Context, string) (func(), error)
}
type Disk struct{}

func (Disk) Read(path string) ([]byte, error) { return os.ReadFile(path) }
func (Disk) Write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".todoist-credentials-stage-*")
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
	return syncDirectory(filepath.Dir(path))
}
func (Disk) Remove(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

// RemoveStaging runs only while the profile-store writer lock is held. These
// names are reserved for this application's atomic writes, including the older
// numeric .credentials-* names. Other dotfiles and symlink targets are untouched.
func (Disk) RemoveStaging(path string) error {
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	removed := false
	for _, entry := range entries {
		name := entry.Name()
		legacy := strings.TrimPrefix(name, ".credentials-")
		numeric := legacy != name && legacy != ""
		for _, r := range legacy {
			if r < '0' || r > '9' {
				numeric = false
				break
			}
		}
		if !strings.HasPrefix(name, ".todoist-credentials-stage-") && !numeric {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return failure(Recovery)
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
		removed = true
	}
	if removed {
		return syncDirectory(dir)
	}
	return nil
}
