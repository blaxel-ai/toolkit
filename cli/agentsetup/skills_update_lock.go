package agentsetup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var errSkillsUpdateBusy = errors.New("another Blaxel skills operation is running")

func skillsUpdateDir(home string) string { return filepath.Join(home, ".blaxel", "skills") }

// Kernel locks coordinate all CLI and MCP processes and release on exit,
// including a crash. Never unlink the lock file: waiters hold its inode.
func withSkillsUpdateLock(ctx context.Context, home string, wait bool, run func() error) error {
	dir := skillsUpdateDir(home)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(dir, "update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		locked, err := trySkillsFileLock(file)
		if err != nil {
			return err
		}
		if locked {
			defer unlockSkillsFile(file)
			return run()
		}
		if !wait {
			return errSkillsUpdateBusy
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
