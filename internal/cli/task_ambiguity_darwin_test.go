package cli

import (
	"bytes"
	"syscall"
	"unsafe"
)

func taskAmbiguityPTYName(fd uintptr) (string, error) {
	for _, request := range []uintptr{syscall.TIOCPTYGRANT, syscall.TIOCPTYUNLK} {
		if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, request, 0); err != 0 {
			return "", err
		}
	}
	var name [128]byte
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0]))); err != 0 {
		return "", err
	}
	return string(name[:bytes.IndexByte(name[:], 0)]), nil
}
