package termsession

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// openMaster opens a new pseudo-terminal's master and returns the slave's
// name: /dev/ptmx, unlocked and named, as grantpt/unlockpt/ptsname do it on
// Linux and macOS.
func openMaster() (*os.File, string, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", fmt.Errorf("termsession: %w", err)
	}
	var unlock int32
	if err := ioctl(master, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		_ = master.Close()
		return nil, "", fmt.Errorf("termsession: unlocking the terminal: %w", err)
	}
	var n uint32
	if err := ioctl(master, syscall.TIOCGPTN, unsafe.Pointer(&n)); err != nil {
		_ = master.Close()
		return nil, "", fmt.Errorf("termsession: naming the terminal: %w", err)
	}
	return master, fmt.Sprintf("/dev/pts/%d", n), nil
}
