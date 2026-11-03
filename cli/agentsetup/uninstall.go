package agentsetup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

// UninstallItem is one thing bl uninstall removes, or keeps and says why.
type UninstallItem struct {
	// Group names the kind of item: MCP servers, Skills or Skills updates.
	Group string
	// Path is the file or folder; Detail what in it, or why it is kept.
	Path, Detail string
	// Err is why the item could not be removed, after Run.
	Err error

	apply func(ctx context.Context) error
}

const (
	UninstallGroupMCP        = "MCP servers"
	UninstallGroupMCPUpdates = "MCP updates"
	UninstallGroupSkills     = "Skills"
	UninstallGroupUpdates    = "Skills updates"
)

// UninstallPlan is what bl uninstall removes: the Blaxel MCP servers and
// skills that bl setup, bl skills and bl mcp wrote and nobody changed since,
// and the automatic updates that would add them back. Everything else is kept.
type UninstallPlan struct {
	Remove, Keep []UninstallItem
	// AutoUpdateOff is set when uninstall turns automatic skills updates off, and
	// MCPAutoUpdateOff when upgrades and install scripts stop adding the MCP
	// servers.
	AutoUpdateOff, MCPAutoUpdateOff bool

	home   string
	skills bool
}

// UninstallOptions selects what to plan, and where.
type UninstallOptions struct {
	Home                string
	Env                 func(string) string
	MCP                 MCPEnv
	SkipSkills, SkipMCP bool
}

// PlanUninstall finds what bl uninstall removes. It changes nothing.
func PlanUninstall(ctx context.Context, options UninstallOptions) (UninstallPlan, error) {
	plan := UninstallPlan{home: options.Home}
	if !options.SkipSkills {
		if err := plan.addSkills(options.Home, options.Env); err != nil {
			return UninstallPlan{}, err
		}
	}
	if !options.SkipMCP {
		plan.addMCP(ctx, options.MCP)
	}
	return plan, nil
}

// Run removes the planned items, and returns them with the problem each met.
// Every item is checked again first: one that changed since the plan is kept.
func (p UninstallPlan) Run(ctx context.Context) ([]UninstallItem, error) {
	items := make([]UninstallItem, len(p.Remove))
	copy(items, p.Remove)
	run := func() error {
		for i := range items {
			items[i].Err = items[i].apply(ctx)
		}
		return nil
	}
	if !p.skills {
		return items, run()
	}
	// Hold the skills lock, so no update installs skills while they go.
	return items, withSkillsUpdateLock(ctx, p.home, true, run)
}

var errChangedSincePlan = errors.New("changed since the plan; left as it is")

// addMCP plans the Blaxel MCP servers to remove from every agent setup knows,
// detected or not.
func (p *UninstallPlan) addMCP(ctx context.Context, e MCPEnv) {
	seen := map[string]bool{}
	for _, agent := range SetupAgents() {
		target, ok := MCPTargets[agent.ID]
		if !ok || target.remove == nil || seen[target.File(e)] {
			continue
		}
		file := target.File(e)
		seen[file] = true
		var names []string
		for _, name := range []string{ResourceMCPServer("").Name, DocsMCPServer().Name} {
			entry := target.Entry(e, name)
			switch {
			case entry == nil:
			case blaxelMCPEntry(ctx, target, name, entry):
				names = append(names, name)
			default:
				p.Keep = append(p.Keep, UninstallItem{Group: UninstallGroupMCP, Path: file,
					Detail: name + ": not what setup writes, so it may be yours"})
			}
		}
		if len(names) == 0 {
			continue
		}
		p.Remove = append(p.Remove, UninstallItem{Group: UninstallGroupMCP, Path: file, Detail: strings.Join(names, ", "),
			apply: func(ctx context.Context) error {
				var problems []error
				for _, name := range names {
					entry := target.Entry(e, name)
					if entry == nil {
						continue
					}
					if !blaxelMCPEntry(ctx, target, name, entry) {
						problems = append(problems, fmt.Errorf("%s %w", name, errChangedSincePlan))
					} else if err := target.remove(ctx, e, name); err != nil {
						problems = append(problems, fmt.Errorf("%s: %w", name, err))
					}
				}
				return errors.Join(problems...)
			}})
	}
	// Without automatic updates, neither a CLI upgrade nor an install script
	// adds the servers back; bl setup does.
	if home := p.home; !mcpAutoUpdateSavedOff(home) {
		p.MCPAutoUpdateOff = true
		p.Remove = append(p.Remove, UninstallItem{Group: UninstallGroupMCPUpdates, Path: mcpSetupStatePath(home), Detail: "automatic updates off",
			apply: func(context.Context) error { return SetMCPAutoUpdate(home, false) }})
	}
}

// blaxelMCPEntry reports whether the entry is exactly what Blaxel writes for
// this target: the docs server, bl mcp started from a bl binary, or the
// hosted server that earlier versions added. Disabled, extended or otherwise
// changed entries are not.
func blaxelMCPEntry(ctx context.Context, target MCPTarget, name string, entry map[string]any) bool {
	var candidates []MCPServer
	switch name {
	case DocsMCPServer().Name:
		candidates = append(candidates, DocsMCPServer())
	case ResourceMCPServer("").Name:
		if command, _ := EntryCommand(entry); filepath.IsAbs(command) && blCommand(command) {
			candidates = append(candidates, ResourceMCPServer(command))
		}
		for _, key := range []string{"url", "httpUrl", "serverUrl"} {
			if url, _ := entry[key].(string); isHostedResourceMCPURL(url) {
				candidates = append(candidates, MCPServer{Name: name, URL: url})
			}
		}
	}
	for _, server := range candidates {
		if !target.Takes(server) {
			continue
		}
		if expected, err := writtenMCPEntry(ctx, target, server); err == nil && reflect.DeepEqual(withoutEmptyMaps(entry), withoutEmptyMaps(expected)) {
			return true
		}
	}
	return false
}

// writtenMCPEntry is the entry the target's own writer adds for the server,
// read back the same way, from an empty scratch home.
func writtenMCPEntry(ctx context.Context, target MCPTarget, server MCPServer) (map[string]any, error) {
	home, err := os.MkdirTemp("", "blaxel-uninstall-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(home) }()
	scratch := MCPEnv{Home: home, Config: filepath.Join(home, ".config"),
		Env:      func(string) string { return "" },
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Run: func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("no commands run in a scratch home")
		},
		Exists: func(string) bool { return true },
	}
	if err := target.write(ctx, scratch, server, false); err != nil {
		return nil, err
	}
	return target.Entry(scratch, server.Name), nil
}

// withoutEmptyMaps drops an empty env or headers, which the claude CLI
// writes and changes nothing.
func withoutEmptyMaps(entry map[string]any) map[string]any {
	entry = maps.Clone(entry)
	for _, key := range []string{"env", "headers"} {
		if value, ok := entry[key].(map[string]any); ok && len(value) == 0 {
			delete(entry, key)
		}
	}
	return entry
}

// addSkills plans the Blaxel skills to remove, with the ownership rule of
// automatic updates: a folder in ~/.agents/skills goes only while it still
// has the contents recorded when it was installed. Links managed elsewhere
// and edited folders stay, with their lock entries.
func (p *UninstallPlan) addSkills(home string, env func(string) string) error {
	lockFile := SkillsLockPath(home, env)
	data, err := os.ReadFile(lockFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, entries := readSkillsLock(data)
	base := filepath.Join(home, ".agents", "skills")
	insideHome, err := skillsBaseInsideHome(home, base)
	if err != nil {
		return err
	}
	roots, err := agentSkillRoots(NewSkillsAgentPaths(home, env), base)
	if err != nil {
		return err
	}
	var folders, links, kept []UninstallItem
	var names []string
	folderOf := map[string]string{}
	for _, entry := range entries {
		var locked skillsLockEntry
		if json.Unmarshal(entry.value, &locked) != nil || locked.Source != skillsRepo {
			continue
		}
		baseline := locked.InstalledHash
		if baseline == "" {
			baseline = locked.SkillFolderHash
		}
		folder := sanitizeSkillName(entry.key)
		canonical := filepath.Join(base, folder)
		info, err := os.Lstat(canonical)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if info != nil {
			if isSkillLink(canonical, info.Mode()) {
				kept = append(kept, UninstallItem{Group: UninstallGroupSkills, Path: canonical, Detail: "externally managed link"})
				continue
			}
			if !insideHome || !info.IsDir() || baseline == "" || !sameSkillHash(canonical, baseline) {
				kept = append(kept, UninstallItem{Group: UninstallGroupSkills, Path: canonical, Detail: "changed since it was installed"})
				continue
			}
			folders = append(folders, UninstallItem{Group: UninstallGroupSkills, Path: canonical,
				apply: func(context.Context) error { return removeSkillCopy(canonical, baseline) }})
		}
		names = append(names, entry.key)
		folderOf[entry.key] = canonical
		target, err := resolveSkillPath(canonical)
		if err != nil {
			return err
		}
		for _, root := range roots {
			link := filepath.Join(root, folder)
			info, err := os.Lstat(link)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			switch {
			case isSkillLink(link, info.Mode()):
				if destination, err := resolveSkillPath(link); err != nil || destination != target {
					kept = append(kept, UninstallItem{Group: UninstallGroupSkills, Path: link, Detail: "links to another copy"})
					continue
				}
				links = append(links, UninstallItem{Group: UninstallGroupSkills, Path: link, Detail: "link",
					apply: func(context.Context) error { return removeSkillLink(link, target) }})
			case info.IsDir():
				// Where directory links are unavailable, agents got a copy.
				if baseline == "" || !sameSkillHash(link, baseline) {
					kept = append(kept, UninstallItem{Group: UninstallGroupSkills, Path: link, Detail: "changed since it was installed"})
					continue
				}
				links = append(links, UninstallItem{Group: UninstallGroupSkills, Path: link, Detail: "copy",
					apply: func(context.Context) error { return removeSkillCopy(link, baseline) }})
			}
		}
	}
	// Links first, so no agent is left with a link to a removed folder.
	remove := slices.Concat(links, folders)
	if len(names) > 0 {
		remove = append(remove, UninstallItem{Group: UninstallGroupSkills, Path: lockFile, Detail: "entries for " + joinSkillsNames(names),
			apply: func(context.Context) error {
				// A folder that could not be removed keeps its entry.
				gone := slices.DeleteFunc(slices.Clone(names), func(name string) bool {
					_, err := os.Lstat(folderOf[name])
					return !errors.Is(err, os.ErrNotExist)
				})
				return removeSkillsLockEntries(lockFile, gone)
			}})
	}
	// Without automatic updates, neither bl, a CLI upgrade nor an install
	// script reinstalls skills. The saved state keeps only the uninstall mark,
	// which bl setup clears, and the user's own update choice, which it keeps.
	off := SkillsUpdateState{Uninstalled: true}
	state, stateErr := ReadSkillsUpdateState(home)
	if stateErr == nil {
		off.AutoUpdate = state.AutoUpdate
	}
	if stateErr != nil || !reflect.DeepEqual(state, off) {
		p.AutoUpdateOff = off.AutoUpdate == nil || *off.AutoUpdate
		remove = append(remove, UninstallItem{Group: UninstallGroupUpdates, Path: skillsUpdateStatePath(home), Detail: "automatic updates off",
			apply: func(context.Context) error { return WriteSkillsUpdateState(home, off) }})
	}
	p.Remove, p.Keep = append(p.Remove, remove...), append(p.Keep, kept...)
	p.skills = len(remove) > 0
	return nil
}

// sameSkillHash reports whether a skill folder has the recorded contents.
func sameSkillHash(dir, baseline string) bool {
	hash, err := localSkillHash(dir)
	return err == nil && hash == baseline
}

// skillsBaseInsideHome reports whether ~/.agents/skills, followed through any
// link (for example from dotfiles), stays in the home folder.
func skillsBaseInsideHome(home, base string) (bool, error) {
	resolvedBase, err := resolveSkillPath(base)
	if err != nil {
		return false, err
	}
	resolvedHome, err := resolveSkillPath(home)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(resolvedHome, resolvedBase)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel), nil
}

// agentSkillRoots returns the skills folders of agents that do not read
// ~/.agents/skills, where setup links each skill. Folders that are the shared
// one under another name are left out.
func agentSkillRoots(paths SkillsAgentPaths, base string) ([]string, error) {
	resolvedBase, err := resolveSkillPath(base)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{resolvedBase: true}
	var roots []string
	for _, agent := range SkillsAgents {
		if agent.universal {
			continue
		}
		for _, dir := range agent.homes(paths) {
			root := filepath.Join(dir, "skills")
			resolved, err := resolveSkillPath(root)
			if err != nil {
				return nil, err
			}
			if !seen[resolved] {
				seen[resolved] = true
				roots = append(roots, root)
			}
		}
	}
	return roots, nil
}

// removeSkillLink removes an agent's link while it still points at the folder.
func removeSkillLink(link, target string) error {
	info, err := os.Lstat(link)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if destination, err := resolveSkillPath(link); err != nil || !isSkillLink(link, info.Mode()) || destination != target {
		return errChangedSincePlan
	}
	return os.Remove(link)
}

// removeSkillCopy removes a skill folder while it still has the contents
// that were installed.
func removeSkillCopy(dir, baseline string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if isSkillLink(dir, info.Mode()) || !info.IsDir() || !sameSkillHash(dir, baseline) {
		return errChangedSincePlan
	}
	return os.RemoveAll(dir)
}

// removeSkillsLockEntries deletes the named skills from the skills lock file,
// keeping every other entry, and deletes the file when nothing is left in it.
func removeSkillsLockEntries(file string, names []string) error {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	lock, entries := readSkillsLock(data)
	var remaining []jsonMember
	for _, entry := range entries {
		if !slices.Contains(names, entry.key) {
			remaining = append(remaining, entry)
		}
	}
	if len(remaining) == len(entries) {
		return nil
	}
	encoded, err := encodeJSONObject(remaining)
	if err != nil {
		return err
	}
	lock[findJSONMember(lock, "skills")].value = encoded
	updated, err := encodeJSONObject(lock)
	if err != nil {
		return err
	}
	empty := true
	for _, member := range lock {
		if member.key != "version" && !emptyJSONObject(member.value) {
			empty = false
		}
	}
	if empty {
		return removeConfigFile(file, updated)
	}
	return writeConfigFile(file, updated)
}
