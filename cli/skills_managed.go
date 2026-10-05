package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// A projection owned by a dotfile/skill manager is used, never refreshed from
// upstream. Preflight the whole install before replacing any folder or lock.
type skillInstallPlan struct {
	skill     archivedSkill
	canonical string
	preserved bool
	links     []string
}

func planSkillsInstall(base string, paths skillsAgentPaths, skills []archivedSkill, selected []skillsAgent) ([]skillInstallPlan, error) {
	wanted, folders := map[string]bool{}, map[string]bool{}
	for _, skill := range skills {
		folder := sanitizeSkillName(skill.name)
		if folders[folder] {
			return nil, fmt.Errorf("multiple archived skills use the installation folder %q", folder)
		}
		wanted[skill.name], folders[folder] = true, true
	}
	// Shared agent roots (including symlinks to ~/.agents/skills) are scanned once.
	indexes := map[string]map[string][]string{}
	index := func(root string) (map[string][]string, error) {
		resolved, err := resolveSkillPath(root)
		if err != nil {
			return nil, err
		}
		if cached, ok := indexes[resolved]; ok {
			return cached, nil
		}
		found, err := existingSkillFolders(resolved, wanted)
		if err != nil {
			return nil, err
		}
		indexes[resolved] = found
		return found, nil
	}
	shared, err := index(base)
	if err != nil {
		return nil, err
	}
	var plans []skillInstallPlan
	for _, skill := range skills {
		canonical := filepath.Join(base, sanitizeSkillName(skill.name))
		plan := skillInstallPlan{skill: skill, canonical: canonical}
		info, err := os.Lstat(canonical)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err == nil {
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				manifest, err := os.ReadFile(filepath.Join(canonical, "SKILL.md"))
				name, valid := skillManifestName(manifest)
				if err != nil || !valid || name != skill.name {
					return nil, fmt.Errorf("externally managed link %s does not contain a valid %s skill; left unchanged", canonical, skill.name)
				}
				plan.preserved = true
			case !info.IsDir():
				return nil, fmt.Errorf("skill destination %s is not a directory; left unchanged", canonical)
			}
		}
		target, err := resolveSkillPath(canonical)
		if err != nil {
			return nil, err
		}
		for _, existing := range shared[skill.name] {
			if existing != target {
				return nil, skillNameConflict(skill.name, existing, canonical)
			}
		}
		for _, agent := range selected {
			if agent.universal {
				continue
			}
			root := skillsAgentDir(agent, paths)
			found, err := index(root)
			if err != nil {
				return nil, err
			}
			link := filepath.Join(root, sanitizeSkillName(skill.name))
			destination, err := resolveSkillPath(link)
			if err != nil {
				return nil, err
			}
			info, err := os.Lstat(link)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			if err == nil && info.Mode()&os.ModeSymlink != 0 && destination != target {
				return nil, fmt.Errorf("externally managed link %s points to %s, not %s; left unchanged", link, destination, target)
			}
			if err == nil && !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
				return nil, fmt.Errorf("skill destination %s is not a directory; left unchanged", link)
			}
			for _, existing := range found[skill.name] {
				if existing != target && existing != destination {
					return nil, skillNameConflict(skill.name, existing, link)
				}
			}
			plan.links = append(plan.links, link)
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func skillNameConflict(name, existing, destination string) error {
	return fmt.Errorf("skill %s already exists at %s; no skills were changed. Keep one copy and link %s to it before retrying", name, existing, destination)
}

// Resolve existing links even when the final destination does not exist yet.
// A dangling skill link can then be compared to its planned canonical target.
func resolveSkillPath(name string) (string, error) {
	resolved, err := filepath.EvalSymlinks(name)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if target, linkErr := os.Readlink(name); linkErr == nil {
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(name), target)
		}
		// EvalSymlinks already rejects cycles; a dangling link's target has a
		// missing component, so resolve only its parent rather than follow it again.
		parent, err := filepath.EvalSymlinks(filepath.Dir(target))
		if err == nil {
			return filepath.Join(parent, filepath.Base(target)), nil
		}
		return "", fmt.Errorf("cannot resolve externally managed link %s: %w", name, err)
	}
	parent := filepath.Dir(name)
	if parent == name {
		return "", err
	}
	resolved, err = resolveSkillPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(name)), nil
}

// Find same-name skills in nested roots, not just at the installer's flat
// destination. Follow projected namespaces once, without traversing cycles,
// skill contents, hidden folders or dependencies.
func existingSkillFolders(root string, wanted map[string]bool) (map[string][]string, error) {
	found, visited := map[string][]string{}, map[string]bool{}
	var walk func(string) error
	walk = func(directory string) error {
		return filepath.WalkDir(directory, func(name string, entry fs.DirEntry, walkErr error) error {
			if errors.Is(walkErr, os.ErrNotExist) && name == directory {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			if name != directory && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules") {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				info, err := os.Stat(name)
				if errors.Is(err, os.ErrNotExist) {
					return nil // destination preflight handles broken projections
				}
				if err != nil {
					return err
				}
				if !info.IsDir() {
					return nil
				}
				target, err := resolveSkillPath(name)
				if err != nil {
					return err
				}
				return walk(target)
			}
			if !entry.IsDir() {
				return nil
			}
			if visited[name] {
				return filepath.SkipDir
			}
			visited[name] = true
			manifest, err := os.ReadFile(filepath.Join(name, "SKILL.md"))
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			if skill, valid := skillManifestName(manifest); valid && wanted[skill] {
				found[skill] = append(found[skill], name)
			}
			return filepath.SkipDir
		})
	}
	err := walk(root)
	return found, err
}
