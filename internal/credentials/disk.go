package credentials

import (
	"context"
	"os"
	"path/filepath"
)

// Persistence permits failure injection at the durable storage boundary.
type Persistence interface {
	Read(string) ([]byte, error)
	Write(string, []byte) error
	Remove(string) error
	Lock(context.Context, string) (func(), error)
}
type Disk struct{}

func (Disk) Read(path string) ([]byte, error) { return os.ReadFile(path) }
func (Disk) Write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".credentials-*")
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
