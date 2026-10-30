package termsession

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// The ioctls of grantpt, unlockpt and ptsname on macOS (sys/ttycom.h).
const (
	tiocptygrant = 0x20007454 // _IO('t', 84)
	tiocptyunlk  = 0x20007452 // _IO('t', 82)
	tiocptygname = 0x40807453 // _IOC(IOC_OUT, 't', 83, 128)
)

// openMaster opens a new pseudo-terminal's master and returns the slave's
// name: /dev/ptmx, granted, unlocked and named, as grantpt/unlockpt/ptsname
// do it.
func openMaster() (*os.File, string, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", fmt.Errorf("termsession: %w", err)
	}
	var name [128]byte
	for _, step := range []struct {
		req uintptr
		arg unsafe.Pointer
	}{
		{tiocptygrant, nil},
		{tiocptyunlk, nil},
		{tiocptygname, unsafe.Pointer(&name[0])},
	} {
		if err := ioctl(master, step.req, step.arg); err != nil {
			_ = master.Close()
			return nil, "", fmt.Errorf("termsession: %w", err)
		}
	}
	n := bytes.IndexByte(name[:], 0)
	if n < 0 {
		n = len(name)
	}
	return master, string(name[:n]), nil
}
