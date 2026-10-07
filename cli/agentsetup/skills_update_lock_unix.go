//go:build !windows

package agentsetup

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func trySkillsFileLock(file *os.File) (bool, error) {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return false, nil
	}
	return err == nil, err
}

func unlockSkillsFile(file *os.File) { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN) }
