package agentsetup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Hash the installed representation, including additional files and modes.
// The upstream Git hash differs when excluded files or symlinks are present.
func localSkillHash(root string) (string, error) {
	var files []skillFile
	var total int64
	entries := 0
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		entries++
		if entries > skillsArchiveMaxEntries {
			return errors.New("installed skill has too many files")
		}
		if name != root && generatedSkillFile(entry.Name(), entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		file := skillFile{path: filepath.ToSlash(relative), mode: int64(info.Mode().Perm())}
		switch {
		case isSkillLink(name, info.Mode()):
			file.link, err = os.Readlink(name)
			file.isLink = true
		case info.Mode().IsRegular():
			if total += info.Size(); total > skillsExtractedMaxBytes {
				return errors.New("installed skill is too large")
			}
			var input *os.File
			input, err = os.Open(name)
			if err == nil {
				file.data, err = io.ReadAll(io.LimitReader(input, skillsExtractedMaxBytes-total+info.Size()+1))
				_ = input.Close()
				if int64(len(file.data)) != info.Size() && err == nil {
					err = errors.New("installed skill changed while hashing")
				}
			}
		default:
			return fmt.Errorf("installed skill contains a special file: %s", name)
		}
		if err != nil {
			return err
		}
		files = append(files, file)
		return nil
	})
	if err != nil {
		return "", err
	}
	return gitTreeHash(files), nil
}

// generatedSkillFile reports files that tools create inside a skill folder by
// themselves (Python caches, Finder and Explorer metadata). They are not local
// edits, so they must not stop updates; the installer never ships them.
func generatedSkillFile(name string, dir bool) bool {
	if dir {
		return name == "__pycache__"
	}
	return name == ".DS_Store" || name == "Thumbs.db" || name == "desktop.ini"
}

// Automatic and manifest updates never repair or adopt manager-owned paths.
// An older skills lock can prove ownership only when its raw Git hash matches
// the actual installed contents. Otherwise an explicit install is needed.
func preserveEditedSkills(plans []skillInstallPlan, home string, env func(string) string) error {
	data, err := os.ReadFile(SkillsLockPath(home, env))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, entries := readSkillsLock(data)
	base := filepath.Join(home, ".agents", "skills")
	resolvedBase, err := resolveSkillPath(base)
	if err != nil {
		return err
	}
	resolvedHome, err := resolveSkillPath(home)
	if err != nil {
		return err
	}
	for i := range plans {
		plan := &plans[i]
		var locked skillsLockEntry
		if index := findJSONMember(entries, plan.skill.name); index >= 0 {
			_ = json.Unmarshal(entries[index].value, &locked)
		}
		baseline := locked.InstalledHash
		if baseline == "" {
			baseline = locked.SkillFolderHash
		}
		info, err := os.Lstat(plan.canonical)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if plan.preserved || resolvedBase != filepath.Join(resolvedHome, ".agents", "skills") {
			plan.preserved, plan.links = true, nil
			continue
		}
		if info != nil {
			plan.expectedHash, err = localSkillHash(plan.canonical)
			if err != nil || locked.Source != skillsRepo || baseline == "" || plan.expectedHash != baseline {
				plan.preserved, plan.links = true, nil
				continue
			}
		}
		var safe []skillLinkPlan
		for _, link := range plan.links {
			info, err := os.Lstat(link.destination)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			// The shared copy updates through existing CLI links, and managers
			// retain their own projections. An ordinary agent copy is refreshed
			// only while it still matches what the CLI installed (older CLIs
			// copied where Windows had no directory links); others are kept.
			if !link.repair && info == nil {
				safe = append(safe, link)
			} else if info != nil && !isSkillLink(link.destination, info.Mode()) {
				if hash, err := localSkillHash(link.destination); !link.repair && err == nil && locked.Source == skillsRepo && baseline != "" && hash == baseline {
					link.expectedHash = baseline
					safe = append(safe, link)
				} else {
					plan.skippedLinks = append(plan.skippedLinks, plan.skill.name+" at "+link.destination+" (ordinary agent copy preserved)")
				}
			}
		}
		plan.links = safe
	}
	return nil
}
