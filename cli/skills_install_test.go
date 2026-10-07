package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// archiveEntry is one entry of a test repository tarball.
type archiveEntry struct {
	name, body, link string
	mode             int64
}

const testSkillManifest = "---\nname: %s\ndescription: Use the %s.\n---\n\n# %s\n"

func skillManifest(name string) string {
	return strings.ReplaceAll(testSkillManifest, "%s", name)
}

// testSkillsEntries mirrors the agent-skills repository layout, with the edge
// cases the installer must handle.
func testSkillsEntries() []archiveEntry {
	return []archiveEntry{
		{name: "README.md", body: "# Agent skills\n"},
		{name: "skills/blaxel-cli/SKILL.md", body: skillManifest("blaxel-cli")},
		{name: "skills/blaxel-cli/metadata.json", body: `{"version":"1"}`},
		{name: "skills/blaxel-cli/references/login.md", body: "# bl login\n"},
		{name: "skills/blaxel-cli/references/alias.md", link: "login.md"},
		{name: "skills/blaxel-cli/outside.md", link: "../../README.md"},
		{name: "skills/blaxel-cli/scripts/generate.sh", body: "#!/bin/sh\necho docs\n", mode: 0o775},
		{name: "skills/blaxel-cli/__pycache__/cache.pyc", body: "cache"},
		{name: "skills/blaxel-sdk/SKILL.md", body: skillManifest("blaxel-sdk")},
		{name: "skills/blaxel-sdk/references/sdk-python.md", body: "# Python\n"},
		{name: "skills/internal-tool/SKILL.md", body: "---\nname: internal-tool\ndescription: Internal.\nmetadata:\n  internal: true\n---\n"},
		{name: "skills/broken/SKILL.md", body: "---\nname: broken\n---\n"},
		{name: "skills/notes/README.md", body: "no skill here"},
		{name: "skills/../../escape/SKILL.md", body: skillManifest("escape")},
		{name: "prompts/SKILL.md", body: skillManifest("prompt")},
	}
}

func buildSkillsArchive(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	// GitHub archives start with the commit as a global header.
	require.NoError(t, writer.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header",
		PAXRecords: map[string]string{"comment": "0123456789abcdef0123456789abcdef01234567"}}))
	require.NoError(t, writer.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "agent-skills-HEAD/", Mode: 0o775}))
	for _, entry := range entries {
		header := &tar.Header{Name: "agent-skills-HEAD/" + entry.name, Mode: 0o664, Size: int64(len(entry.body)), Typeflag: tar.TypeReg}
		if entry.mode != 0 {
			header.Mode = entry.mode
		}
		if entry.link != "" {
			header.Typeflag, header.Linkname, header.Size, header.Mode = tar.TypeSymlink, entry.link, 0, 0o777
		}
		require.NoError(t, writer.WriteHeader(header))
		if entry.link == "" {
			_, err := writer.Write([]byte(entry.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, writer.Close())
	require.NoError(t, compressed.Close())
	return buffer.Bytes()
}

func noEnv(string) string { return "" }

func readLock(t *testing.T, file string) map[string]any {
	t.Helper()
	var lock map[string]any
	require.NoError(t, json.Unmarshal([]byte(readTestFile(t, file)), &lock))
	return lock
}

func lockedSkill(t *testing.T, lock map[string]any, name string) map[string]any {
	t.Helper()
	skills, ok := lock["skills"].(map[string]any)
	require.True(t, ok, lock)
	entry, ok := skills[name].(map[string]any)
	require.True(t, ok, "%s is not in the lock file: %v", name, skills)
	return entry
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// assertSkillLink checks an agent's skill folder shows the shared copy: a
// relative link where links work, and a full copy on Windows.
func assertSkillLink(t *testing.T, home, agentDir, skill string) {
	t.Helper()
	path := filepath.Join(agentDir, skill)
	assert.Equal(t, readTestFile(t, filepath.Join(home, ".agents", "skills", skill, "SKILL.md")), readTestFile(t, filepath.Join(path, "SKILL.md")))
	if runtime.GOOS == "windows" {
		return
	}
	target, err := os.Readlink(path)
	require.NoError(t, err, "%s should be a link", path)
	assert.False(t, filepath.IsAbs(target), "links stay valid when the home moves: %s", target)
	resolved, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	canonical, err := filepath.EvalSymlinks(filepath.Join(home, ".agents", "skills", skill))
	require.NoError(t, err)
	assert.Equal(t, canonical, resolved)
}

func TestInstallSkillsArchive(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{".claude", ".codex", ".pi/agent"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, dir), 0755))
	}
	now := time.Date(2026, 10, 1, 12, 30, 45, 123456789, time.UTC)
	result, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, detectedSkillsAgents(home, noEnv), now)
	require.NoError(t, err)
	assert.Equal(t, []string{"blaxel-cli", "blaxel-sdk"}, result.skills)
	assert.Equal(t, []string{"Claude Code", "Codex", "Pi"}, result.agents)

	skills := filepath.Join(home, ".agents", "skills")
	assert.ElementsMatch(t, []string{"blaxel-cli", "blaxel-sdk"}, dirNames(t, skills), "only valid, public skills are installed")
	cli := filepath.Join(skills, "blaxel-cli")
	assert.Equal(t, skillManifest("blaxel-cli"), readTestFile(t, filepath.Join(cli, "SKILL.md")))
	assert.Equal(t, "# bl login\n", readTestFile(t, filepath.Join(cli, "references", "alias.md")), "links inside a skill are copied as files")
	assert.NoFileExists(t, filepath.Join(cli, "outside.md"), "links leaving the skill are skipped")
	assert.NoFileExists(t, filepath.Join(cli, "metadata.json"))
	assert.NoDirExists(t, filepath.Join(cli, "__pycache__"))
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(cli, "scripts", "generate.sh"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
		info, err = os.Stat(filepath.Join(cli, "SKILL.md"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	}

	// Agents that do not read ~/.agents/skills get a link to the shared copy.
	for _, skill := range []string{"blaxel-cli", "blaxel-sdk"} {
		assertSkillLink(t, home, filepath.Join(home, ".claude", "skills"), skill)
		assertSkillLink(t, home, filepath.Join(home, ".pi", "agent", "skills"), skill)
	}
	assert.NoDirExists(t, filepath.Join(home, ".codex", "skills"), "Codex reads ~/.agents/skills")
	// Nothing is left over from staging.
	assert.ElementsMatch(t, []string{".skill-lock.json", "skills"}, dirNames(t, filepath.Join(home, ".agents")))
	assert.ElementsMatch(t, []string{"skills"}, dirNames(t, filepath.Join(home, ".claude")))
	assert.ElementsMatch(t, []string{"blaxel-cli", "blaxel-sdk"}, dirNames(t, filepath.Join(home, ".claude", "skills")))

	lock := readLock(t, filepath.Join(home, ".agents", ".skill-lock.json"))
	assert.Equal(t, float64(3), lock["version"])
	assert.Equal(t, map[string]any{}, lock["dismissed"])
	entry := lockedSkill(t, lock, "blaxel-cli")
	assert.Equal(t, "blaxel-ai/agent-skills", entry["source"])
	assert.Equal(t, "github", entry["sourceType"])
	assert.Equal(t, "https://github.com/blaxel-ai/agent-skills.git", entry["sourceUrl"])
	assert.Equal(t, "skills/blaxel-cli/SKILL.md", entry["skillPath"])
	assert.Regexp(t, "^[0-9a-f]{40}$", entry["skillFolderHash"])
	installedHash, err := localSkillHash(cli)
	require.NoError(t, err)
	assert.Equal(t, installedHash, entry["blaxelInstalledHash"], "ownership hash includes the normalized installed representation")
	assert.Equal(t, "2026-10-01T12:30:45.123Z", entry["installedAt"])
	assert.Equal(t, "2026-10-01T12:30:45.123Z", entry["updatedAt"])
	assert.NotEqual(t, entry["skillFolderHash"], lockedSkill(t, lock, "blaxel-sdk")["skillFolderHash"])
}

func TestInstallSkillsArchiveRefreshesAndKeepsOtherSkills(t *testing.T) {
	home := t.TempDir()
	skills := filepath.Join(home, ".agents", "skills")
	writeTestFile(t, filepath.Join(skills, "blaxel-cli", "removed-upstream.md"), "stale")
	writeTestFile(t, filepath.Join(skills, "my-skill", "SKILL.md"), skillManifest("my-skill"))
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "blaxel-sdk", "SKILL.md"), "an old copy")
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "mine", "SKILL.md"), skillManifest("mine"))
	lockFile := filepath.Join(home, ".agents", ".skill-lock.json")
	writeTestFile(t, lockFile, `{"version": 3, "skills": {"my-skill": {"source": "me/skills", "skillFolderHash": "abc"},`+
		` "blaxel-cli": {"source": "blaxel-ai/agent-skills", "installedAt": "2026-01-01T00:00:00.000Z"}},`+
		` "dismissed": {"findSkillsPrompt": true}, "lastSelectedAgents": ["claude-code"]}`)

	agents := detectedSkillsAgents(home, noEnv)
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, agents, now)
	require.NoError(t, err)

	assert.NoFileExists(t, filepath.Join(skills, "blaxel-cli", "removed-upstream.md"), "a refresh replaces the whole skill")
	assert.FileExists(t, filepath.Join(skills, "my-skill", "SKILL.md"), "other skills are left alone")
	assert.FileExists(t, filepath.Join(home, ".claude", "skills", "mine", "SKILL.md"))
	assertSkillLink(t, home, filepath.Join(home, ".claude", "skills"), "blaxel-sdk")

	data := readTestFile(t, lockFile)
	lock := readLock(t, lockFile)
	assert.Equal(t, map[string]any{"source": "me/skills", "skillFolderHash": "abc"}, lockedSkill(t, lock, "my-skill"))
	cli := lockedSkill(t, lock, "blaxel-cli")
	assert.Equal(t, "2026-01-01T00:00:00.000Z", cli["installedAt"], "the first install time is kept")
	assert.Equal(t, "2026-10-02T00:00:00.000Z", cli["updatedAt"])
	assert.Equal(t, map[string]any{"findSkillsPrompt": true}, lock["dismissed"])
	assert.Equal(t, []any{"claude-code"}, lock["lastSelectedAgents"])
	assert.Less(t, strings.Index(data, `"version"`), strings.Index(data, `"lastSelectedAgents"`), "key order is kept")
	assert.Less(t, strings.Index(data, `"my-skill"`), strings.Index(data, `"blaxel-sdk"`))

	// Running again with the same skills changes only the update time.
	before := readTestFile(t, filepath.Join(skills, "blaxel-cli", "SKILL.md"))
	_, err = installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, agents, now.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, before, readTestFile(t, filepath.Join(skills, "blaxel-cli", "SKILL.md")))
	assert.Equal(t, cli["skillFolderHash"], lockedSkill(t, readLock(t, lockFile), "blaxel-cli")["skillFolderHash"])
	assertSkillLink(t, home, filepath.Join(home, ".claude", "skills"), "blaxel-cli")
}

func TestInstallSkillsArchiveAgentFolderLinkedToSharedFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory links need developer mode on Windows")
	}
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".agents", "skills"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0755))
	require.NoError(t, os.Symlink(filepath.Join(home, ".agents", "skills"), filepath.Join(home, ".claude", "skills")))

	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, detectedSkillsAgents(home, noEnv), time.Now())
	require.NoError(t, err)
	info, err := os.Lstat(filepath.Join(home, ".agents", "skills", "blaxel-cli"))
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "the shared copy must not be replaced by a link to itself")
	assert.FileExists(t, filepath.Join(home, ".claude", "skills", "blaxel-cli", "SKILL.md"))
}

func TestInstallSkillsArchiveSymlinkedSharedFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory links need developer mode on Windows")
	}
	// Dotfile setups often link ~/.agents/skills to a repository.
	home, dotfiles := t.TempDir(), t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".agents"), 0755))
	require.NoError(t, os.Symlink(dotfiles, filepath.Join(home, ".agents", "skills")))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0755))

	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, detectedSkillsAgents(home, noEnv), time.Now())
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dotfiles, "blaxel-cli", "SKILL.md"))
	assert.ElementsMatch(t, []string{"blaxel-cli", "blaxel-sdk"}, dirNames(t, dotfiles))
	assertSkillLink(t, home, filepath.Join(home, ".claude", "skills"), "blaxel-cli")
	_, err = os.Readlink(filepath.Join(home, ".agents", "skills"))
	assert.NoError(t, err, "the user's link is kept")
}

func TestInstallSkillsArchiveRejectsBadArchives(t *testing.T) {
	home := t.TempDir()
	_, err := installSkillsArchive([]byte("<html>rate limited</html>"), home, noEnv, nil, time.Now())
	assert.ErrorContains(t, err, "reading the skills archive")

	_, err = installSkillsArchive(buildSkillsArchive(t, []archiveEntry{{name: "skills/notes/README.md", body: "x"}}), home, noEnv, nil, time.Now())
	assert.ErrorContains(t, err, "no skills found")
	assert.NoDirExists(t, filepath.Join(home, ".agents"), "no skills are written when the archive has no skills")
}

func TestReadSkillsArchiveFailsWhenCutAtTheCap(t *testing.T) {
	archive := buildSkillsArchive(t, testSkillsEntries())
	compressed, err := gzip.NewReader(bytes.NewReader(archive))
	require.NoError(t, err)
	expanded, err := io.ReadAll(compressed)
	require.NoError(t, err)
	complete, err := readSkillsArchive(archive)
	require.NoError(t, err)

	previous := skillsArchiveReadMax
	t.Cleanup(func() { skillsArchiveReadMax = previous })
	// Wherever the cap lands, including right after a file's data, a cut
	// archive must fail rather than pass for a complete one with fewer skills.
	for limit := int64(0); limit < int64(len(expanded)); limit++ {
		skillsArchiveReadMax = limit
		_, err := readSkillsArchive(archive)
		require.ErrorIs(t, err, errSkillsArchiveTooLarge, "cap at byte %d", limit)
	}
	skillsArchiveReadMax = int64(len(expanded))
	skills, err := readSkillsArchive(archive)
	require.NoError(t, err, "an archive that ends exactly at the cap is complete")
	assert.Len(t, skills, len(complete))
}

func TestConservativeUpdateRecordsSkillsReplacedBeforeAFailure(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder that can't be moved, which Windows and root don't enforce")
	}
	home := resolvedTempDir(t)
	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, nil, time.Now())
	require.NoError(t, err)
	updated := testSkillsEntries()
	for i := range updated {
		if strings.HasSuffix(updated[i].name, "/SKILL.md") {
			updated[i].body += "\nUpdated.\n"
		}
	}
	archive := buildSkillsArchive(t, updated)

	// Moving a folder to another parent rewrites its "..", so a read-only
	// blaxel-sdk fails after blaxel-cli has already been replaced.
	root := filepath.Join(home, ".agents", "skills")
	sdk := filepath.Join(root, "blaxel-sdk")
	require.NoError(t, os.Chmod(sdk, 0o555))
	t.Cleanup(func() { _ = os.Chmod(sdk, 0o755) })
	_, err = installSkillsArchiveUnlocked(archive, home, noEnv, nil, time.Now(), true)
	require.ErrorContains(t, err, "installing blaxel-sdk")

	cli := filepath.Join(root, "blaxel-cli")
	assert.Contains(t, readTestFile(t, filepath.Join(cli, "SKILL.md")), "Updated.")
	hash, err := localSkillHash(cli)
	require.NoError(t, err)
	locked := lockedSkill(t, readLock(t, skillsLockPath(home, noEnv)), "blaxel-cli")
	assert.Equal(t, hash, locked["blaxelInstalledHash"], "the replaced skill must stay CLI-owned")

	require.NoError(t, os.Chmod(sdk, 0o755))
	result, err := installSkillsArchiveUnlocked(archive, home, noEnv, nil, time.Now(), true)
	require.NoError(t, err)
	assert.Empty(t, result.preserved, "the next update must not mistake the new copy for a local edit")
	assert.Contains(t, readTestFile(t, filepath.Join(sdk, "SKILL.md")), "Updated.")
}

func TestSkillsLockLocationAndReset(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	env := func(key string) string {
		if key == "XDG_STATE_HOME" {
			return state
		}
		return ""
	}
	lockFile := filepath.Join(state, "skills", ".skill-lock.json")
	writeTestFile(t, lockFile, `{"version": 2, "skills": {"old": {}}}`)
	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, env, nil, time.Now())
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(home, ".agents", ".skill-lock.json"))
	lock := readLock(t, lockFile)
	skills, _ := lock["skills"].(map[string]any)
	assert.NotContains(t, skills, "old", "like the skills CLI, an outdated lock file starts over")
	assert.Contains(t, skills, "blaxel-cli")

	for _, broken := range []string{"not json", `{"version": 3, "skills": null}`, `[]`} {
		writeTestFile(t, lockFile, broken)
		_, err = installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, env, nil, time.Now())
		require.NoError(t, err, broken)
		lockedSkill(t, readLock(t, lockFile), "blaxel-sdk")
	}
}

// The lock file records Git tree IDs, which the skills CLI compares with
// GitHub to find updates. Check them against Git itself.
func TestSkillsFolderHashMatchesGit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture has symlinks")
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	repository := t.TempDir()
	for _, entry := range testSkillsEntries() {
		if strings.Contains(entry.name, "..") {
			continue
		}
		path := filepath.Join(repository, filepath.FromSlash(entry.name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		if entry.link != "" {
			require.NoError(t, os.Symlink(entry.link, path))
			continue
		}
		mode := os.FileMode(0644)
		if entry.mode&0o111 != 0 {
			mode = 0755
		}
		require.NoError(t, os.WriteFile(path, []byte(entry.body), mode))
	}
	git := func(args ...string) string {
		command := exec.Command(gitPath, append([]string{"-C", repository, "-c", "core.fileMode=true", "-c", "core.symlinks=true"}, args...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		output, err := command.CombinedOutput()
		require.NoError(t, err, string(output))
		return strings.TrimSpace(string(output))
	}
	git("init", "-q")
	git("add", "-A")
	tree := git("write-tree")

	home := t.TempDir()
	_, err = installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, nil, time.Now())
	require.NoError(t, err)
	lock := readLock(t, filepath.Join(home, ".agents", ".skill-lock.json"))
	for _, skill := range []string{"blaxel-cli", "blaxel-sdk"} {
		assert.Equal(t, git("rev-parse", tree+":skills/"+skill), lockedSkill(t, lock, skill)["skillFolderHash"], skill)
	}
}

func TestDownloadSkillsArchive(t *testing.T) {
	archive := buildSkillsArchive(t, testSkillsEntries())
	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	t.Setenv(skillsArchiveURLEnv, server.URL+"/archive.tar.gz")
	data, err := downloadSkillsArchive(context.Background())
	require.NoError(t, err)
	assert.Equal(t, archive, data)
	assert.True(t, strings.HasPrefix(userAgent, "blaxel-cli/"), userAgent)

	t.Setenv(skillsArchiveURLEnv, server.URL+"/missing")
	_, err = downloadSkillsArchive(context.Background())
	assert.ErrorContains(t, err, "404")
}

func TestSkillManifestName(t *testing.T) {
	for manifest, expected := range map[string]string{
		skillManifest("blaxel-cli"):                                                     "blaxel-cli",
		"---\r\nname: Windows\r\ndescription: CRLF.\r\n---\r\n":                         "Windows",
		"---\nname: quoted\ndescription: \"a: b\"\nmetadata:\n  internal: false\n---\n": "quoted",
		"---\nname: internal\ndescription: x\nmetadata:\n  internal: true\n---\n":       "",
		"---\nname: no-description\n---\n":                                              "",
		"---\nname: [a]\ndescription: not a string name\n---\n":                         "",
		"---\nname: unterminated\ndescription: x\n":                                     "",
		"# No frontmatter\n":                                                            "",
		"---\nname: : :\n---\n":                                                         "",
	} {
		name, ok := skillManifestName([]byte(manifest))
		assert.Equal(t, expected, name, manifest)
		assert.Equal(t, expected != "", ok, manifest)
	}
}

func TestSanitizeSkillName(t *testing.T) {
	for name, expected := range map[string]string{
		"blaxel-cli": "blaxel-cli", "Blaxel SDK": "blaxel-sdk", "../../etc": "etc", "a/b": "a-b",
		"--x--": "x", "...": "unnamed-skill", "v1.2_beta": "v1.2_beta", strings.Repeat("a", 300): strings.Repeat("a", 255),
	} {
		assert.Equal(t, expected, sanitizeSkillName(name), name)
	}
}

// The folders match the global skills folders of the skills package.
func TestSkillsAgentDir(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{"XDG_CONFIG_HOME": filepath.Join(home, "xdg")}
	paths := newSkillsAgentPaths(home, func(key string) string { return env[key] })
	expected := map[string]string{
		"claude-code": ".claude/skills", "windsurf": ".codeium/windsurf/skills", "goose": "xdg/goose/skills",
		"kiro-cli": ".kiro/skills", "roo": ".roo/skills", "continue": ".continue/skills", "augment": ".augment/skills",
		"junie": ".junie/skills", "trae": ".trae/skills", "qwen-code": ".qwen/skills", "openhands": ".openhands/skills",
		"pi": ".pi/agent/skills", "crush": ".config/crush/skills", "devin": "xdg/devin/skills", "openclaw": ".openclaw/skills",
	}
	for _, agent := range skillsAgents {
		if agent.universal {
			continue
		}
		relative, ok := expected[agent.id]
		require.True(t, ok, "add %s to this test", agent.id)
		assert.Equal(t, filepath.Join(home, filepath.FromSlash(relative)), skillsAgentDir(agent, paths), agent.id)
	}
	openclaw, _ := findSkillsAgent("openclaw")
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".clawdbot"), 0755))
	assert.Equal(t, filepath.Join(home, ".clawdbot", "skills"), skillsAgentDir(openclaw, paths))
	env["CLAUDE_CONFIG_DIR"] = filepath.Join(home, "claude")
	claude, _ := findSkillsAgent("claude-code")
	assert.Equal(t, filepath.Join(home, "claude", "skills"), skillsAgentDir(claude, paths))
}

func TestDetectSkillsAgents(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{}
	lookup := func(key string) string { return env[key] }

	targets, names := detectSkillsAgents(home, lookup)
	assert.Equal(t, []string{"universal"}, targets, "no agents: only the shared directory")
	assert.Empty(t, names)
	entries, err := os.ReadDir(home)
	require.NoError(t, err)
	assert.Empty(t, entries, "detection must not create directories")

	for _, dir := range []string{".claude", ".codex", filepath.Join(".config", "goose"), ".cursor"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, dir), 0700))
	}
	targets, names = detectSkillsAgents(home, lookup)
	assert.Equal(t, []string{"universal", "claude-code", "goose"}, targets)
	assert.Equal(t, []string{"Claude Code", "Codex", "Cursor", "Goose"}, names)

	// Custom config locations are honored, like the skills installer does.
	other := t.TempDir()
	env["CLAUDE_CONFIG_DIR"] = filepath.Join(other, "claude")
	env["XDG_CONFIG_HOME"] = filepath.Join(other, "xdg")
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".claude")))
	require.NoError(t, os.MkdirAll(env["CLAUDE_CONFIG_DIR"], 0700))
	require.NoError(t, os.MkdirAll(filepath.Join(env["XDG_CONFIG_HOME"], "opencode"), 0700))
	targets, names = detectSkillsAgents(home, lookup)
	assert.Equal(t, []string{"universal", "claude-code"}, targets)
	assert.Equal(t, []string{"Claude Code", "Codex", "Cursor", "OpenCode"}, names)
}

func TestSkillsInstalledMessage(t *testing.T) {
	skills := []string{"blaxel-cli", "blaxel-sdk"}
	assert.Equal(t, "Blaxel skills installed to ~/.agents/skills (blaxel-cli, blaxel-sdk). Restart your coding agent to load them.",
		skillsInstalledMessage(skillsInstallResult{skills: skills}))
	assert.Equal(t, "Blaxel skills installed for Claude Code (blaxel-cli, blaxel-sdk). Restart your coding agent to load them.",
		skillsInstalledMessage(skillsInstallResult{skills: skills, agents: []string{"Claude Code"}}))
	assert.Equal(t, "Blaxel skills installed for Claude Code, Codex and Cursor (blaxel-cli, blaxel-sdk). Restart your coding agent to load them.",
		skillsInstalledMessage(skillsInstallResult{skills: skills, agents: []string{"Claude Code", "Codex", "Cursor"}}))
}

func TestIsSkillsCommand(t *testing.T) {
	assert.True(t, isSkillsCommand([]string{"skills", "install"}))
	assert.False(t, isSkillsCommand([]string{"get", "skills"}))
	assert.False(t, isSkillsCommand(nil))
}

func TestDownloadSkillsArchiveRetriesBriefProblems(t *testing.T) {
	previous := skillsDownloadAttempts
	skillsDownloadAttempts = []time.Duration{0, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { skillsDownloadAttempts = previous })
	archive := buildSkillsArchive(t, testSkillsEntries())
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/flaky" && calls < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if r.URL.Path == "/gone" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	t.Setenv(skillsArchiveURLEnv, server.URL+"/flaky")
	data, err := downloadSkillsArchive(context.Background())
	require.NoError(t, err)
	assert.Equal(t, archive, data)
	assert.Equal(t, 3, calls)

	calls = 0
	t.Setenv(skillsArchiveURLEnv, server.URL+"/gone")
	_, err = downloadSkillsArchive(context.Background())
	assert.ErrorContains(t, err, "404")
	assert.Equal(t, 1, calls, "a missing archive is not retried")

	t.Setenv(skillsArchiveURLEnv, "http://127.0.0.1:1/unreachable")
	_, err = downloadSkillsArchive(context.Background())
	assert.EqualError(t, err, "couldn't reach 127.0.0.1:1 · check your connection")
}
