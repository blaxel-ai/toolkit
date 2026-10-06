//go:build !windows

package cli

import "os"

// Keep Unix links relative to their final destination, not the staging folder.
func createSkillDirectoryLink(_, relative, link string) error {
	return os.Symlink(relative, link)
}
