package cli

import (
	"fmt"
	"syscall"
	"unsafe"
)

func taskAmbiguityPTYName(fd uintptr) (string, error) {
	var unlock int32
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); err != 0 {
		return "", err
	}
	var number uint32
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGPTN, uintptr(unsafe.Pointer(&number))); err != 0 {
		return "", err
	}
	return fmt.Sprintf("/dev/pts/%d", number), nil
}
