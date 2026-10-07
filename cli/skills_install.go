package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	core.RegisterCommand("skills", SkillsCmd)
}

func SkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "skills", Short: "Manage Blaxel skills for coding agents",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error { return nil },
	}
	cmd.AddCommand(&cobra.Command{
		Use: "install", Short: "Install or refresh Blaxel skills for coding agents",
		Long:         "Install the latest Blaxel agent skills globally from github.com/blaxel-ai/agent-skills.\nSkills go to ~/.agents/skills and to the coding agents detected on this machine\n(Claude Code, Codex, Cursor, ...). Nothing else needs to be installed first.\nThis explicit command runs even when automatic installation is disabled with\nBL_INSTALL_SKILLS=false or in CI.\nExisting externally managed skill links are kept, not refreshed. Same-name\nskills in nested folders are reused automatically. Conflicting copies are\nbacked up outside the agents' skills folders and replaced with links.",
		Args:         cobra.NoArgs,
		RunE:         func(_ *cobra.Command, _ []string) error { return installSkillsOnce() },
		SilenceUsage: true, SilenceErrors: true,
	})
	cmd.AddCommand(skillsUpdateCommands()...)
	return cmd
}

// skillsInstallResult summarizes a successful installation for the user.
type skillsInstallResult struct {
	skills    []string
	preserved []string // externally managed skills used without refreshing or claiming ownership
	repaired  []string
	backups   []string
	agents    []string
	skipped   []string
}

const (
	// skillsArchiveURL serves the default branch of skillsRepo as a tarball.
	skillsArchiveURL = "https://codeload.github.com/" + skillsRepo + "/tar.gz/HEAD"
	// skillsArchiveURLEnv downloads the same tarball from a mirror instead.
	skillsArchiveURLEnv = "BL_SKILLS_ARCHIVE_URL"
	// Limits keep a broken or hostile download from filling the disk or memory.
	skillsArchiveMaxBytes   = 32 << 20
	skillsExtractedMaxBytes = 64 << 20
	skillsArchiveMaxEntries = 10000
)

// skillsArchiveSource downloads the skills archive. Tests replace it.
var skillsArchiveSource = downloadSkillsArchive

func installDetectedSkills(ctx context.Context) (skillsInstallResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return skillsInstallResult{}, err
	}
	return installSkillsFor(ctx, detectedSkillsAgents(home, os.Getenv))
}

// installSkillsFor installs the skills to ~/.agents/skills and the given agents.
func installSkillsFor(ctx context.Context, selected []skillsAgent) (skillsInstallResult, error) {
	return installSkillsForMode(ctx, selected, false)
}

func installSkillsForSafely(ctx context.Context, selected []skillsAgent) (skillsInstallResult, error) {
	return installSkillsForMode(ctx, selected, true)
}

func installSkillsForMode(ctx context.Context, selected []skillsAgent, conservative bool) (skillsInstallResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return skillsInstallResult{}, err
	}
	archive, err := skillsArchiveSource(ctx)
	if err != nil {
		return skillsInstallResult{}, err
	}
	var result skillsInstallResult
	err = withSkillsUpdateLock(ctx, home, true, func() error {
		var err error
		result, err = installSkillsArchiveUnlocked(archive, home, os.Getenv, selected, time.Now(), conservative)
		if err != nil {
			return err
		}
		return invalidateSkillsBundleRevision(home, result.skills)
	})
	return result, err
}

// skillsDownloadAttempts retries brief network problems, such as a DNS
// lookup failing while a VPN reconnects.
var skillsDownloadAttempts = []time.Duration{0, 300 * time.Millisecond, time.Second}

func downloadSkillsArchive(ctx context.Context) ([]byte, error) {
	address := skillsArchiveURL
	if mirror := strings.TrimSpace(os.Getenv(skillsArchiveURLEnv)); mirror != "" {
		address = mirror
	}
	var lastErr error
	for _, wait := range skillsDownloadAttempts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		data, retry, err := fetchSkillsArchive(ctx, address)
		if err == nil || !retry {
			return data, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// fetchSkillsArchive downloads the archive once, and reports whether a
// failure is worth retrying.
func fetchSkillsArchive(ctx context.Context, address string) (data []byte, retry bool, err error) {
	return fetchSkillsArchiveWithClient(ctx, address, http.DefaultClient)
}

func fetchSkillsArchiveWithClient(ctx context.Context, address string, client *http.Client) (data []byte, retry bool, err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, false, err
	}
	request.Header.Set("User-Agent", "blaxel-cli/"+core.GetVersion())
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		return nil, true, &skillsNetworkError{host: request.URL.Host, err: err}
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		retry := response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
		return nil, retry, fmt.Errorf("downloading the skills from %s: %s", request.URL.Host, response.Status)
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, skillsArchiveMaxBytes+1))
	if err != nil {
		return nil, true, &skillsNetworkError{host: request.URL.Host, err: err}
	}
	if len(data) > skillsArchiveMaxBytes {
		return nil, false, fmt.Errorf("the skills archive is larger than %d MB", skillsArchiveMaxBytes>>20)
	}
	return data, false, nil
}

// skillsNetworkError reads as a short sentence; the cause stays available.
type skillsNetworkError struct {
	host string
	err  error
}

func (e *skillsNetworkError) Error() string {
	return "couldn't reach " + e.host + " · check your connection"
}

func (e *skillsNetworkError) Unwrap() error { return e.err }

// skillFile is one entry of a skill folder, with its path relative to the folder.
type skillFile struct {
	path   string
	mode   int64
	data   []byte
	link   string
	isLink bool
}

// archivedSkill is a skill folder found in the archive.
type archivedSkill struct {
	name, folder string
	files        []skillFile
}

// readSkillsArchive returns the skills under skills/<name>/ in a repository tarball.
func readSkillsArchive(archive []byte) ([]archivedSkill, error) {
	compressed, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("reading the skills archive: %w", err)
	}
	defer func() { _ = compressed.Close() }()
	// Include tar headers/padding in the decompression cap, even for entries
	// outside skills/ that will be ignored below.
	reader := tar.NewReader(io.LimitReader(compressed, skillsExtractedMaxBytes+skillsArchiveMaxEntries*1024+1))
	folders := map[string][]skillFile{}
	var total int64
	for entries := 0; ; entries++ {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading the skills archive: %w", err)
		}
		if entries >= skillsArchiveMaxEntries {
			return nil, fmt.Errorf("the skills archive has more than %d entries", skillsArchiveMaxEntries)
		}
		// Paths are <repository>-<ref>/skills/<skill>/<file>.
		_, name, _ := strings.Cut(strings.TrimPrefix(header.Name, "./"), "/")
		name = strings.TrimSuffix(name, "/")
		if !strings.HasPrefix(name, "skills/") || !validArchivePath(name) {
			continue
		}
		parts := strings.SplitN(name, "/", 3)
		if len(parts) < 3 {
			continue
		}
		folder := parts[0] + "/" + parts[1]
		file := skillFile{path: parts[2], mode: header.Mode}
		switch header.Typeflag {
		case tar.TypeReg:
			if total += header.Size; total > skillsExtractedMaxBytes {
				return nil, fmt.Errorf("the skills archive expands to more than %d MB", skillsExtractedMaxBytes>>20)
			}
			if file.data, err = io.ReadAll(reader); err != nil {
				return nil, fmt.Errorf("reading the skills archive: %w", err)
			}
		case tar.TypeSymlink:
			file.link, file.isLink = header.Linkname, true
		default:
			continue
		}
		folders[folder] = append(folders[folder], file)
	}

	var skills []archivedSkill
	for folder, files := range folders {
		var manifest []byte
		for _, file := range files {
			if file.path == "SKILL.md" && !file.isLink {
				manifest = file.data
			}
		}
		name, ok := skillManifestName(manifest)
		if !ok {
			continue
		}
		sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
		skills = append(skills, archivedSkill{name: name, folder: folder, files: files})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].name < skills[j].name })
	if len(skills) == 0 {
		return nil, errors.New("no skills found in the skills archive")
	}
	return skills, nil
}

// validArchivePath rejects absolute paths and parent directory references.
func validArchivePath(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// skillManifestName reads the name of a SKILL.md, following the skills
// installer: a skill needs a name and a description, and internal skills are
// only installed on request.
func skillManifestName(manifest []byte) (string, bool) {
	text := strings.ReplaceAll(string(manifest), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return "", false
	}
	body, _, found := strings.Cut(text[4:], "\n---")
	if !found {
		return "", false
	}
	var frontmatter struct {
		Name        any `yaml:"name"`
		Description any `yaml:"description"`
		Metadata    struct {
			Internal any `yaml:"internal"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal([]byte(body), &frontmatter); err != nil {
		return "", false
	}
	name, nameOK := frontmatter.Name.(string)
	description, descriptionOK := frontmatter.Description.(string)
	if !nameOK || !descriptionOK || strings.TrimSpace(name) == "" || strings.TrimSpace(description) == "" {
		return "", false
	}
	if internal, _ := frontmatter.Metadata.Internal.(bool); internal {
		return "", false
	}
	return strings.TrimSpace(name), true
}

var skillNameInvalid = regexp.MustCompile(`[^a-z0-9._]+`)

// sanitizeSkillName turns a skill name into its folder name, as the skills installer does.
func sanitizeSkillName(name string) string {
	name = skillNameInvalid.ReplaceAllString(strings.ToLower(name), "-")
	name = strings.TrimRight(strings.TrimLeft(name, ".-"), ".-")
	if len(name) > 255 {
		name = name[:255]
	}
	if name == "" {
		return "unnamed-skill"
	}
	return name
}

// gitTreeHash is the Git tree ID of a skill folder. The skills CLI records it
// in its lock file to find updates, so `npx skills update` keeps working.
func gitTreeHash(files []skillFile) string {
	type entry struct {
		name, mode string
		id         []byte
	}
	var build func(prefix string) []byte
	build = func(prefix string) []byte {
		children := map[string]entry{}
		for _, file := range files {
			if !strings.HasPrefix(file.path, prefix) {
				continue
			}
			rest := strings.TrimPrefix(file.path, prefix)
			if first, _, nested := strings.Cut(rest, "/"); nested {
				if _, seen := children[first]; !seen {
					children[first] = entry{name: first, mode: "40000", id: build(prefix + first + "/")}
				}
				continue
			}
			switch {
			case file.isLink:
				children[rest] = entry{name: rest, mode: "120000", id: gitObjectID("blob", []byte(file.link))}
			case file.mode&0o111 != 0:
				children[rest] = entry{name: rest, mode: "100755", id: gitObjectID("blob", file.data)}
			default:
				children[rest] = entry{name: rest, mode: "100644", id: gitObjectID("blob", file.data)}
			}
		}
		sorted := make([]entry, 0, len(children))
		for _, child := range children {
			sorted = append(sorted, child)
		}
		// Git orders trees as if their names ended with a slash.
		key := func(e entry) string {
			if e.mode == "40000" {
				return e.name + "/"
			}
			return e.name
		}
		sort.Slice(sorted, func(i, j int) bool { return key(sorted[i]) < key(sorted[j]) })
		var body bytes.Buffer
		for _, child := range sorted {
			body.WriteString(child.mode + " " + child.name + "\x00")
			body.Write(child.id)
		}
		return gitObjectID("tree", body.Bytes())
	}
	return hex.EncodeToString(build(""))
}

func gitObjectID(kind string, data []byte) []byte {
	// Git object IDs are SHA-1; they identify content and protect nothing.
	hash := sha1.New()
	_, _ = fmt.Fprintf(hash, "%s %d\x00", kind, len(data))
	hash.Write(data)
	return hash.Sum(nil)
}

// Files the skills installer leaves out of installed skills.
func skillFileExcluded(name string) bool {
	for _, part := range strings.Split(name, "/") {
		if part == ".git" || part == "__pycache__" || part == "__pypackages__" {
			return true
		}
	}
	return path.Base(name) == "metadata.json"
}

// installSkillsArchive installs every skill in the archive the way
// `skills add -g` does: one copy in ~/.agents/skills, which most agents read,
// and a link from each other selected agent's skills folder to that copy.
func installSkillsArchive(archive []byte, home string, env func(string) string, selected []skillsAgent, now time.Time) (skillsInstallResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), skillsUpdateTimeout)
	defer cancel()
	var result skillsInstallResult
	err := withSkillsUpdateLock(ctx, home, true, func() error {
		var err error
		result, err = installSkillsArchiveUnlocked(archive, home, env, selected, now, false)
		if err != nil {
			return err
		}
		return invalidateSkillsBundleRevision(home, result.skills)
	})
	return result, err
}

func installSkillsArchiveUnlocked(archive []byte, home string, env func(string) string, selected []skillsAgent, now time.Time, conservative bool) (skillsInstallResult, error) {
	skills, err := readSkillsArchive(archive)
	if err != nil {
		return skillsInstallResult{}, err
	}
	paths := newSkillsAgentPaths(home, env)
	canonicalBase := filepath.Join(home, ".agents", "skills")
	plans, err := planSkillsInstall(canonicalBase, paths, skills, selected)
	if err != nil {
		return skillsInstallResult{}, err
	}
	if conservative {
		if err := preserveEditedSkills(plans, home, env); err != nil {
			return skillsInstallResult{}, err
		}
	}
	if err := preflightSkillRepairs(plans); err != nil {
		return skillsInstallResult{}, err
	}
	_, agentNames := skillsTargets(selected)
	result := skillsInstallResult{agents: agentNames}
	var installed []archivedSkill
	for _, plan := range plans {
		result.skipped = append(result.skipped, plan.skippedLinks...)
		files := plan.skill.files
		if plan.preserved {
			result.preserved = append(result.preserved, plan.skill.name)
			files = nil // a failed symlink must never fall back to a copy of the upstream fork
		} else {
			if err := replaceSkillFolderChecked(plan.canonical, files, conservative, plan.expectedHash); err != nil {
				return skillsInstallResult{}, fmt.Errorf("installing %s: %w", plan.skill.name, err)
			}
			installed = append(installed, plan.skill)
			result.skills = append(result.skills, plan.skill.name)
		}
		for _, link := range plan.links {
			if link.repair {
				backup, err := repairSkillLink(link)
				if err != nil {
					return skillsInstallResult{}, fmt.Errorf("reconciling %s at %s: %w", plan.skill.name, link.destination, err)
				}
				result.repaired = append(result.repaired, link.destination)
				if backup != "" {
					result.backups = append(result.backups, backup)
				}
				continue
			}
			if err := linkSkillFolder(link.target, link.destination, files); errors.Is(err, errManagedSkillNotLinked) {
				continue // that agent is left unchanged
			} else if err != nil {
				return skillsInstallResult{}, fmt.Errorf("linking %s at %s: %w", plan.skill.name, link.destination, err)
			}
		}
	}
	if len(installed) > 0 {
		if err := recordSkillsLock(home, env, installed, now); err != nil {
			return skillsInstallResult{}, fmt.Errorf("recording the skills in the skills lock file: %w", err)
		}
	}
	return result, nil
}

// writeSkillFolder writes a skill's files into an empty folder. Links inside
// the skill are replaced by the file they point to, as the skills installer
// copies with dereferencing; links leaving the skill are skipped.
func writeSkillFolder(dir string, files []skillFile) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, file := range installedSkillFiles(files) {
		destination := filepath.Join(dir, filepath.FromSlash(file.path))
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if file.mode&0o111 != 0 {
			mode = 0755
		}
		if err := os.WriteFile(destination, file.data, mode); err != nil {
			return err
		}
	}
	return nil
}

// Use the same normalized representation for installation and the ownership
// hash. Hashing a destination after installation could adopt concurrent edits.
func installedSkillFiles(files []skillFile) []skillFile {
	byPath := map[string]skillFile{}
	var installed []skillFile
	for _, file := range files {
		byPath[file.path] = file
	}
	for _, file := range files {
		if skillFileExcluded(file.path) {
			continue
		}
		resolved := file
		for hops := 0; resolved.isLink && hops < 8; hops++ {
			target := path.Join(path.Dir(resolved.path), resolved.link)
			next, ok := byPath[target]
			if !ok || strings.HasPrefix(resolved.link, "/") {
				break
			}
			resolved = next
		}
		if resolved.isLink {
			continue
		}
		resolved.path = file.path
		if runtime.GOOS == "windows" {
			resolved.mode = 0644
		}
		installed = append(installed, resolved)
	}
	return installed
}

// replaceSkillFolder swaps in a fresh copy of the skill, so agents never see a
// half-written skill and a failure leaves the previous version in place.
func replaceSkillFolder(destination string, files []skillFile) error {
	return replaceSkillFolderChecked(destination, files, false, "")
}

func replaceSkillFolderChecked(destination string, files []skillFile, check bool, baseline string) error {
	if info, err := os.Lstat(destination); err == nil && isSkillLink(destination, info.Mode()) {
		return fmt.Errorf("externally managed skill link %s was left unchanged", destination)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	if resolved, err := evalSkillLinks(parent); err == nil {
		parent = resolved
		destination = filepath.Join(parent, filepath.Base(destination))
	}
	// Stage beside the skills folder, not inside it, so agents scanning it
	// never load the staging copy, and the final rename stays on one disk.
	staging, err := os.MkdirTemp(filepath.Dir(parent), ".blaxel-skills-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	fresh := filepath.Join(staging, "new")
	if err := writeSkillFolder(fresh, files); err != nil {
		return err
	}
	if check {
		info, err := os.Lstat(destination)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if info != nil {
			hash, err := localSkillHash(destination)
			if isSkillLink(destination, info.Mode()) || baseline == "" || err != nil || hash != baseline {
				return errors.New("skill changed during update; left unchanged")
			}
			freshHash, err := localSkillHash(fresh)
			if err != nil {
				return err
			}
			if hash == freshHash {
				return nil
			}
		} else if baseline != "" {
			return errors.New("skill removed during update; left unchanged")
		}
	}
	previous := filepath.Join(staging, "previous")
	hadPrevious := false
	if _, err := os.Lstat(destination); err == nil {
		if err := os.Rename(destination, previous); err != nil {
			return err
		}
		hadPrevious = true
	}
	if err := os.Rename(fresh, destination); err != nil {
		if hadPrevious {
			_ = os.Rename(previous, destination)
		}
		return err
	}
	return nil
}

// linkSkillFolder points an agent's skill folder at the shared copy, and
// uses a junction on Windows without requiring symlink privileges, and falls
// back to a copy where directory links are unavailable.
// errManagedSkillNotLinked means an externally managed skill could not be
// linked into an agent's folder. No upstream copy
// is substituted; that agent's folder is left unchanged.
var errManagedSkillNotLinked = errors.New("externally managed skill not linked")

var skillDirectoryLink = createSkillDirectoryLink

func linkSkillFolder(canonical, linkPath string, files []skillFile) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return err
	}
	linkDir, err := evalSkillLinks(filepath.Dir(linkPath))
	if err != nil {
		return err
	}
	target, err := evalSkillLinks(canonical)
	if err != nil {
		return err
	}
	linkPath = filepath.Join(linkDir, filepath.Base(linkPath))
	// Already linked, or the agent's skills folder is itself a link to the shared one.
	if existing, err := evalSkillLinks(linkPath); err == nil && existing == target {
		return nil
	}
	if info, err := os.Lstat(linkPath); err == nil && isSkillLink(linkPath, info.Mode()) {
		return fmt.Errorf("externally managed skill link %s was left unchanged", linkPath)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	relative, err := filepath.Rel(linkDir, target)
	if err != nil {
		relative = target
	}
	// As in replaceSkillFolder, stage outside the folder the agent scans.
	staging, err := os.MkdirTemp(filepath.Dir(linkDir), ".blaxel-skills-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	fresh := filepath.Join(staging, "new")
	// A relative link resolves from the folder it ends up in, not the staging folder.
	if err := skillDirectoryLink(target, relative, fresh); err != nil {
		if files == nil {
			return fmt.Errorf("%w: %v", errManagedSkillNotLinked, err)
		}
		if err := writeSkillFolder(fresh, files); err != nil {
			return err
		}
	}
	previous := filepath.Join(staging, "previous")
	hadPrevious := false
	if _, err := os.Lstat(linkPath); err == nil {
		if err := os.Rename(linkPath, previous); err != nil {
			return err
		}
		hadPrevious = true
	}
	if err := os.Rename(fresh, linkPath); err != nil {
		if hadPrevious {
			_ = os.Rename(previous, linkPath)
		}
		return err
	}
	return nil
}

type skillsLockEntry struct {
	Source          string `json:"source"`
	SourceType      string `json:"sourceType"`
	SourceURL       string `json:"sourceUrl"`
	SkillPath       string `json:"skillPath"`
	SkillFolderHash string `json:"skillFolderHash"`
	InstalledAt     string `json:"installedAt"`
	UpdatedAt       string `json:"updatedAt"`
	InstalledHash   string `json:"blaxelInstalledHash,omitempty"`
}

const skillsLockVersion = 3

// skillsLockPath is where the skills CLI keeps its global lock file.
func skillsLockPath(home string, env func(string) string) string {
	if state := env("XDG_STATE_HOME"); state != "" {
		return filepath.Join(state, "skills", ".skill-lock.json")
	}
	return filepath.Join(home, ".agents", ".skill-lock.json")
}

// recordSkillsLock records the skills in the skills CLI lock file, keeping
// every other entry and their order, so `npx skills list -g`, `check` and
// `update` know them.
func recordSkillsLock(home string, env func(string) string, skills []archivedSkill, now time.Time) error {
	file := skillsLockPath(home, env)
	data, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lock, entries := readSkillsLock(data)
	timestamp := now.UTC().Format("2006-01-02T15:04:05.000Z")
	for _, skill := range skills {
		entry := skillsLockEntry{
			Source:          skillsRepo,
			SourceType:      "github",
			SourceURL:       "https://github.com/" + skillsRepo + ".git",
			SkillPath:       skill.folder + "/SKILL.md",
			SkillFolderHash: gitTreeHash(skill.files),
			InstalledAt:     timestamp,
			UpdatedAt:       timestamp,
			InstalledHash:   gitTreeHash(installedSkillFiles(skill.files)),
		}
		index := findJSONMember(entries, skill.name)
		if index >= 0 {
			var existing struct {
				InstalledAt string `json:"installedAt"`
			}
			if json.Unmarshal(entries[index].value, &existing) == nil && existing.InstalledAt != "" {
				entry.InstalledAt = existing.InstalledAt
			}
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if index >= 0 {
			entries[index].value = encoded
		} else {
			entries = append(entries, jsonMember{skill.name, encoded})
		}
	}
	encodedEntries, err := encodeJSONObject(entries)
	if err != nil {
		return err
	}
	lock[findJSONMember(lock, "skills")].value = encodedEntries
	updated, err := encodeJSONObject(lock)
	if err != nil {
		return err
	}
	return writeConfigFile(file, updated)
}

// readSkillsLock returns the lock file members and its skills. Like the
// skills CLI, it starts over from an unreadable or outdated lock file.
func readSkillsLock(data []byte) (lock, entries []jsonMember) {
	fresh := []jsonMember{
		{"version", json.RawMessage(fmt.Sprint(skillsLockVersion))},
		{"skills", json.RawMessage("{}")},
		{"dismissed", json.RawMessage("{}")},
	}
	lock, err := parseJSONObject(data)
	versionIndex, skillsIndex := findJSONMember(lock, "version"), findJSONMember(lock, "skills")
	if err != nil || versionIndex < 0 || skillsIndex < 0 {
		return fresh, nil
	}
	var version float64
	if json.Unmarshal(lock[versionIndex].value, &version) != nil || version < skillsLockVersion {
		return fresh, nil
	}
	if entries, err = parseJSONObject(lock[skillsIndex].value); err != nil {
		return fresh, nil
	}
	return lock, entries
}

// installedBlaxelSkills lists the Blaxel skills recorded in the skills lock file.
func installedBlaxelSkills(home string, env func(string) string) []string {
	data, err := os.ReadFile(skillsLockPath(home, env))
	if err != nil {
		return nil
	}
	_, entries := readSkillsLock(data)
	var names []string
	for _, entry := range entries {
		var locked struct {
			Source string `json:"source"`
		}
		if json.Unmarshal(entry.value, &locked) == nil && locked.Source == skillsRepo {
			names = append(names, entry.key)
		}
	}
	return names
}

// skillsInstalledFor reports whether the agent sees every installed Blaxel skill.
func skillsInstalledFor(agent skillsAgent, home string, env func(string) string, skills []string) bool {
	if len(skills) == 0 {
		return false
	}
	dir := filepath.Join(home, ".agents", "skills")
	if !agent.universal {
		dir = skillsAgentDir(agent, newSkillsAgentPaths(home, env))
	}
	for _, name := range skills {
		if _, err := os.Stat(filepath.Join(dir, sanitizeSkillName(name), "SKILL.md")); err != nil {
			return false
		}
	}
	return true
}
