package agentsetup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/blaxel-ai/toolkit/cli/core"
)

const (
	skillsManifestURL      = "https://raw.githubusercontent.com/" + skillsRepo + "/main/releases/skills-manifest.json"
	SkillsUpdateInterval   = 6 * time.Hour
	skillsUpdateTimeout    = 30 * time.Second
	skillsManifestMaxBytes = 16 << 10
)

var skillsRevisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type skillsUpdateManifest struct {
	Revision   string `json:"revision"`
	BundleURL  string `json:"bundleUrl"`
	SHA256     string `json:"sha256"`
	MinimumCLI string `json:"minimumCli"`
}

type SkillsUpdateState struct {
	AutoUpdate          *bool             `json:"autoupdate,omitempty"`
	Revision            string            `json:"revision,omitempty"`
	VerifiedRevision    string            `json:"verifiedRevision,omitempty"`
	InstalledRevisions  map[string]string `json:"installedRevisions,omitempty"`
	LastAttempt         time.Time         `json:"lastAttempt,omitempty"`
	LastSuccessfulCheck time.Time         `json:"lastSuccessfulCheck,omitempty"`
	Skip                string            `json:"skip,omitempty"`
	SkippedSkills       []string          `json:"skippedSkills,omitempty"`
	Failure             string            `json:"failure,omitempty"`
}

func skillsUpdateStatePath(home string) string {
	return filepath.Join(skillsUpdateDir(home), "state.json")
}

func ReadSkillsUpdateState(home string) (SkillsUpdateState, error) {
	var state SkillsUpdateState
	file, err := os.Open(skillsUpdateStatePath(home))
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, 64<<10+1))
	if err != nil {
		return state, err
	}
	if len(data) > 64<<10 {
		return state, errors.New("skills update state is too large")
	}
	err = json.Unmarshal(data, &state)
	return state, err
}

func WriteSkillsUpdateState(home string, state SkillsUpdateState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeConfigFile(skillsUpdateStatePath(home), append(data, '\n'))
}

// Nonmanifest installs use the same lock but cannot claim a bundle revision.
func invalidateSkillsBundleRevision(home string, changed []string) error {
	if len(changed) == 0 {
		return nil
	}
	state, err := ReadSkillsUpdateState(home)
	if err != nil {
		return err
	}
	if state.VerifiedRevision == "" && len(state.InstalledRevisions) == 0 {
		return nil
	}
	for _, name := range changed {
		delete(state.InstalledRevisions, name)
	}
	state.VerifiedRevision, state.Skip = "", ""
	return WriteSkillsUpdateState(home, state)
}

func SkillsUpdateDisabled(state SkillsUpdateState, env func(string) string) string {
	if SkillsInstallDisabled(env) {
		return "disabled by BL_INSTALL_SKILLS or CI"
	}
	if state.AutoUpdate != nil && !*state.AutoUpdate {
		return "disabled by saved autoupdate preference"
	}
	return ""
}

func newSkillsUpdateClient() *http.Client {
	return &http.Client{
		Timeout: skillsUpdateTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			// GitHub redirects release downloads to its asset CDN. No login,
			// cookies, API transport, or credentials participate in this fetch.
			if len(via) >= 3 || request.URL.Scheme != "https" || request.URL.User != nil ||
				request.URL.Host != "release-assets.githubusercontent.com" || via[0].URL.Host != "github.com" {
				return errors.New("untrusted skills download redirect")
			}
			return nil
		},
	}
}

func (manifest skillsUpdateManifest) validate() error {
	checksum, err := hex.DecodeString(manifest.SHA256)
	if !skillsRevisionPattern.MatchString(manifest.Revision) || err != nil || len(checksum) != sha256.Size {
		return errors.New("invalid skills revision or checksum")
	}
	address := "https://github.com/" + skillsRepo + "/releases/download/skills-" + manifest.Revision + "/skills.tar.gz"
	if manifest.BundleURL != address {
		return errors.New("skills bundle must use its immutable GitHub release URL")
	}
	if _, err := semver.StrictNewVersion(manifest.MinimumCLI); err != nil {
		return errors.New("invalid minimum CLI version in skills manifest")
	}
	return nil
}

type SkillsUpdater struct {
	home, version string
	env           func(string) string
	client        *http.Client
	now           func() time.Time
}

func DefaultSkillsUpdater(version string) (SkillsUpdater, error) {
	home, err := os.UserHomeDir()
	return SkillsUpdater{home: home, version: version, env: os.Getenv, client: newSkillsUpdateClient(), now: time.Now}, err
}

func (updater SkillsUpdater) manifest(ctx context.Context) (skillsUpdateManifest, error) {
	var manifest skillsUpdateManifest
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, skillsManifestURL, nil)
	if err != nil {
		return manifest, err
	}
	request.Header.Set("User-Agent", "blaxel-cli/"+updater.version)
	response, err := updater.client.Do(request)
	if err != nil {
		// Return a bounded message without reflected response bodies/URLs.
		return manifest, fmt.Errorf("skills manifest unavailable: %w", skillsDownloadError(err))
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusNotFound {
		return manifest, os.ErrNotExist
	}
	if response.StatusCode != http.StatusOK {
		return manifest, fmt.Errorf("skills manifest returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, skillsManifestMaxBytes+1))
	if err != nil {
		return manifest, err
	}
	if len(data) > skillsManifestMaxBytes {
		return manifest, errors.New("skills manifest is too large")
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, errors.New("invalid skills manifest JSON")
	}
	return manifest, manifest.validate()
}

func skillsDownloadError(err error) error {
	var requestError *url.Error
	if errors.As(err, &requestError) {
		return requestError.Err
	}
	return err
}

// Every automatic attempt, including failure, consumes one six-hour slot.
// Explicit refresh bypasses scheduling and opt-outs, never content protection.
func (updater SkillsUpdater) check(ctx context.Context, manual bool) error {
	if !manual && SkillsInstallDisabled(updater.env) {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, skillsUpdateTimeout)
	defer cancel()
	return withSkillsUpdateLock(ctx, updater.home, manual, func() error {
		state, err := ReadSkillsUpdateState(updater.home)
		if err != nil {
			return err
		}
		now := updater.now().UTC()
		if !manual && (SkillsUpdateDisabled(state, updater.env) != "" || !state.LastAttempt.IsZero() && now.Sub(state.LastAttempt) < SkillsUpdateInterval) {
			return nil
		}
		state.LastAttempt, state.Skip, state.Failure = now, "", ""
		if err := WriteSkillsUpdateState(updater.home, state); err != nil {
			return err
		}
		err = updater.apply(ctx, &state, manual)
		if err != nil {
			state.Failure = err.Error()
		} else {
			state.LastSuccessfulCheck = updater.now().UTC()
		}
		return errors.Join(err, WriteSkillsUpdateState(updater.home, state))
	})
}

func (updater SkillsUpdater) apply(ctx context.Context, state *SkillsUpdateState, manual bool) error {
	manifest, err := updater.manifest(ctx)
	if errors.Is(err, os.ErrNotExist) {
		state.Skip = "manifest not published yet"
		return nil
	}
	if err != nil {
		return err
	}
	state.Revision = manifest.Revision
	version, err := semver.StrictNewVersion(updater.version)
	minimum, _ := semver.StrictNewVersion(manifest.MinimumCLI)
	if err != nil || version.LessThan(minimum) {
		state.Skip = "requires CLI " + manifest.MinimumCLI + " or newer (development/unknown versions are skipped)"
		return nil
	}
	if !manual && state.VerifiedRevision == manifest.Revision {
		state.Skip = "revision already checked; restart your coding agent to load changed skills"
		return nil
	}
	if !manual && len(InstalledBlaxelSkills(updater.home, updater.env)) == 0 {
		state.Skip = "no CLI-managed skills installed; use bl skills install or bl skills update"
		return nil
	}
	archive, err := downloadSkillsArchiveWithClient(ctx, manifest.BundleURL, updater.client)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(archive)
	if hex.EncodeToString(hash[:]) != manifest.SHA256 {
		return errors.New("skills bundle checksum mismatch; installed skills were left unchanged")
	}
	result, err := installSkillsArchiveUnlocked(archive, updater.home, updater.env, detectedSkillsAgents(updater.home, updater.env), updater.now(), true)
	if err != nil {
		return err
	}
	state.VerifiedRevision = manifest.Revision
	state.SkippedSkills = append(result.Preserved, result.skipped...)
	if state.InstalledRevisions == nil {
		state.InstalledRevisions = map[string]string{}
	}
	for _, name := range result.Skills {
		state.InstalledRevisions[name] = manifest.Revision
	}
	state.Skip = "restart your coding agent to load changed skills"
	if len(state.SkippedSkills) > 0 {
		state.Skip = "modified, unowned or externally managed skills preserved; restart your coding agent to load any updated skills"
	}
	return nil
}

// WriteSkillsUpdateStatus writes the skills revisions, update checks, skips
// and failures as JSON.
func WriteSkillsUpdateStatus(out io.Writer) error {
	updater, err := DefaultSkillsUpdater(core.GetVersion())
	if err != nil {
		return err
	}
	state, err := ReadSkillsUpdateState(updater.home)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(struct {
		SkillsUpdateState
		StatePath        string `json:"statePath"`
		AutomaticUpdates bool   `json:"automaticUpdates"`
		DisabledReason   string `json:"disabledReason,omitempty"`
	}{state, skillsUpdateStatePath(updater.home), SkillsUpdateDisabled(state, updater.env) == "", SkillsUpdateDisabled(state, updater.env)})
}

// UpdateSkills refreshes the skills from the verified release bundle now,
// ignoring automatic update opt-outs, and writes the outcome.
func UpdateSkills(ctx context.Context, out io.Writer) error {
	updater, err := DefaultSkillsUpdater(core.GetVersion())
	if err != nil {
		return err
	}
	if err := updater.check(ctx, true); err != nil {
		return err
	}
	state, err := ReadSkillsUpdateState(updater.home)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, state.Skip)
	return err
}

// SetSkillsAutoUpdate saves this machine's automatic skills update preference.
func SetSkillsAutoUpdate(ctx context.Context, enabled bool) error {
	updater, err := DefaultSkillsUpdater(core.GetVersion())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, skillsUpdateTimeout)
	defer cancel()
	return withSkillsUpdateLock(ctx, updater.home, true, func() error {
		state, err := ReadSkillsUpdateState(updater.home)
		if err != nil {
			return err
		}
		state.AutoUpdate = &enabled
		return WriteSkillsUpdateState(updater.home, state)
	})
}
