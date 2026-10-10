package agentsetup

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// A projection owned by a dotfile/skill manager is used, never refreshed from
// upstream. Preflight the whole install before replacing any folder or lock.
type skillInstallPlan struct {
	skill        archivedSkill
	canonical    string
	preserved    bool
	links        []skillLinkPlan
	expectedHash string
	skippedLinks []string
}

// A repair uses an existing skill rather than the upstream archive. Displaced
// directories are backed up outside the root, never deleted or left scannable.
type skillLinkPlan struct {
	target, destination string
	root                string
	repair              bool
	// expectedHash, when set, is the contents an existing agent copy must
	// still have for an update to replace it.
	expectedHash string
}

func planSkillsInstall(base string, paths SkillsAgentPaths, skills []archivedSkill, selected []SkillsAgent) ([]skillInstallPlan, error) {
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
			case isSkillLink(canonical, info.Mode()):
				manifest, err := readExistingSkillManifest(filepath.Join(canonical, "SKILL.md"))
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
		flatTarget := target
		source, repairs, err := planExistingSkill(base, canonical, target, shared[skill.name], plan.preserved)
		if err != nil {
			return nil, err
		}
		if len(repairs) > 0 {
			plan.preserved = true
			plan.canonical = source
			plan.links = append(plan.links, repairs...)
			target = source
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
			managed := info != nil && isSkillLink(link, info.Mode())
			if managed && destination != target && destination != flatTarget {
				manifest, err := readExistingSkillManifest(filepath.Join(link, "SKILL.md"))
				name, valid := skillManifestName(manifest)
				if err != nil || !valid || name != skill.name {
					return nil, fmt.Errorf("externally managed link %s does not contain a valid %s skill; left unchanged", link, skill.name)
				}
			}
			if err == nil && !info.IsDir() && !managed {
				return nil, fmt.Errorf("skill destination %s is not a directory; left unchanged", link)
			}
			// A root shared with ~/.agents/skills has already been reconciled.
			resolvedRoot, err := resolveSkillPath(root)
			if err != nil {
				return nil, err
			}
			resolvedBase, err := resolveSkillPath(base)
			if err != nil {
				return nil, err
			}
			if resolvedRoot == resolvedBase {
				continue
			}
			_, repairs, err := planExistingSkill(root, link, destination, found[skill.name], managed)
			if err != nil {
				return nil, err
			}
			if len(repairs) > 0 {
				plan.links = append(plan.links, repairs...)
				continue
			}
			if !managed || destination == target || destination == flatTarget {
				repair := plan.preserved && !managed && info != nil && info.IsDir() && destination != target
				plan.links = append(plan.links, skillLinkPlan{target: plan.canonical, destination: link, root: root, repair: repair})
			}
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

// Prefer a namespaced copy over the installer's flat directory. A flat
// symlink is already managed, so its target takes precedence instead. Only
// duplicate directories inside this root may be moved: never rename an
// externally projected target or replace someone else's symlink.
func planExistingSkill(root, flat, target string, existing []string, managed bool) (string, []skillLinkPlan, error) {
	source := target
	if !managed {
		for _, folder := range existing {
			if folder != target {
				source = folder
				break
			}
		}
	}
	resolvedRoot, err := resolveSkillPath(root)
	if err != nil {
		return "", nil, err
	}
	var repairs []skillLinkPlan
	for _, folder := range existing {
		if folder == source || folder == target {
			continue
		}
		relative, err := filepath.Rel(resolvedRoot, folder)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", nil, fmt.Errorf("multiple externally managed copies of a skill in %s; left unchanged", root)
		}
		repairs = append(repairs, skillLinkPlan{target: source, destination: folder, root: root, repair: true})
	}
	if source != target {
		repairs = append(repairs, skillLinkPlan{target: source, destination: flat, root: root, repair: true})
	}
	return source, repairs, nil
}

// Windows junctions are ModeIrregular on modern Go. Recognize only irregular
// entries that Readlink accepts, not arbitrary devices or other reparse points.
func isSkillLink(name string, mode os.FileMode) bool {
	if mode&os.ModeSymlink != 0 {
		return true
	}
	if mode&os.ModeIrregular == 0 {
		return false
	}
	_, err := os.Readlink(name)
	return err == nil
}

// Check required link support before installing any earlier upstream skill.
// In particular, junctions are unavailable on some Windows filesystems. Probe
// outside scanned roots, remove the probe, and leave every skill/lock unchanged
// when the platform cannot project a retained copy.
func preflightSkillRepairs(plans []skillInstallPlan) error {
	checked := map[struct{ root, target string }]bool{}
	for _, plan := range plans {
		for _, link := range plan.links {
			if !link.repair {
				continue
			}
			root, err := resolveSkillPath(link.root)
			if err != nil {
				return err
			}
			target, err := resolveSkillPath(link.target)
			if err != nil {
				return err
			}
			key := struct{ root, target string }{root, target}
			if checked[key] {
				continue
			}
			if err := probeSkillDirectoryLink(root, target); err != nil {
				return fmt.Errorf("cannot link retained skill at %s; no skills were changed: %w", link.destination, err)
			}
			checked[key] = true
		}
	}
	return nil
}

func probeSkillDirectoryLink(root, target string) error {
	staging, err := os.MkdirTemp(filepath.Dir(root), ".blaxel-skills-link-check-")
	if err != nil {
		return err
	}
	err = skillDirectoryLink(target, target, filepath.Join(staging, "link"))
	return errors.Join(err, os.RemoveAll(staging))
}

// repairSkillLink retains the displaced directory on the same filesystem,
// outside the scanned root. If linking fails, restore it before returning.
func repairSkillLink(plan skillLinkPlan) (string, error) {
	target, err := resolveSkillPath(plan.target)
	if err != nil {
		return "", err
	}
	destination, err := resolveSkillPath(plan.destination)
	if err != nil {
		return "", err
	}
	if destination == target {
		return "", nil
	}
	info, err := os.Lstat(plan.destination)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	backup := ""
	if err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("skill destination %s is not a directory; left unchanged", plan.destination)
		}
		root, err := resolveSkillPath(plan.root)
		if err != nil {
			return "", err
		}
		directory, err := os.MkdirTemp(filepath.Dir(root), ".blaxel-skills-backup-")
		if err != nil {
			return "", err
		}
		backup = filepath.Join(directory, filepath.Base(plan.destination))
		if err := os.Rename(plan.destination, backup); err != nil {
			_ = os.Remove(directory)
			return "", err
		}
	}
	if err := linkSkillFolder(plan.target, plan.destination, nil); err != nil {
		if backup != "" {
			if restoreErr := os.Rename(backup, plan.destination); restoreErr != nil {
				return backup, fmt.Errorf("%w; previous skill retained at %s (restore failed: %v)", err, backup, restoreErr)
			}
			_ = os.Remove(filepath.Dir(backup))
		}
		return "", err
	}
	return backup, nil
}

// Resolve existing links even when the final destination does not exist yet.
// A dangling skill link can then be compared to its planned canonical target.
func resolveSkillPath(name string) (string, error) {
	resolved, err := EvalSkillLinks(name)
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
		parent, err := EvalSkillLinks(filepath.Dir(target))
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
	entries := 0
	var walk func(string) error
	walk = func(directory string) error {
		return filepath.WalkDir(directory, func(name string, entry fs.DirEntry, walkErr error) error {
			if errors.Is(walkErr, os.ErrNotExist) && name == directory {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			entries++
			if entries > skillsArchiveMaxEntries {
				return fmt.Errorf("skill root %s exceeds the scan limit", root)
			}
			if name != directory && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules") {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if isSkillLink(name, entry.Type()) {
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
			manifest, err := readExistingSkillManifest(filepath.Join(name, "SKILL.md"))
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

func readExistingSkillManifest(name string) ([]byte, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("skill manifest %s must be a regular file", name)
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, 1<<20+1))
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("skill manifest %s is too large", name)
	}
	return data, err
}
