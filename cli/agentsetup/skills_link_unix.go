//go:build !windows

package agentsetup

import (
	"os"
	"path/filepath"
)

// Keep Unix links relative to their final destination, not the staging folder.
func CreateSkillDirectoryLink(_, relative, link string) error {
	return os.Symlink(relative, link)
}

func EvalSkillLinks(name string) (string, error) {
	return filepath.EvalSymlinks(name)
}
