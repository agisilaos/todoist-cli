//go:build windows

package credentials

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")
var lockFile = kernel.NewProc("LockFileEx")
var unlockFile = kernel.NewProc("UnlockFileEx")

// Windows directory handles cannot be synced with os.File.Sync. File content
// is flushed before the replace; uncertain replace errors remain recoverable.
func syncDirectory(string) error { return nil }
func (Disk) Lock(ctx context.Context, path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	ov := new(syscall.Overlapped)
	for {
		ok, _, callErr := lockFile.Call(f.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(ov)))
		if ok != 0 {
			return func() { unlockFile.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(ov))); f.Close() }, nil
		}
		if callErr != syscall.Errno(33) {
			f.Close()
			return nil, callErr
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, failure(Busy)
		case <-time.After(20 * time.Millisecond):
		}
	}
}
