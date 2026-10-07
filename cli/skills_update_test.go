package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blaxel-ai/toolkit/cli/mcpbridge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixture entrypoint exercises Execute/MCPCmd on macOS without changing
// system trust. Only the test binary injects its local CA into the HTTP client.
// The finite worker inherits SSL_CERT_FILE and uses the same entrypoint.
func TestMain(tests *testing.M) {
	if os.Getenv("BL_TEST_SKILLS_ENTRYPOINT") == "1" || os.Getenv(skillsUpdateWorkerEnv) == "1" {
		certificate, err := os.ReadFile(os.Getenv("SSL_CERT_FILE"))
		if err != nil {
			panic(err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(certificate) {
			panic("invalid fixture certificate")
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
		http.DefaultTransport = transport
		if err := Execute("0.1.121", "fixture", "fixture"); err != nil {
			_, _ = io.WriteString(os.Stderr, err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(tests.Run())
}

type skillsRoundTripper func(*http.Request) (*http.Response, error)

func (transport skillsRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func updateManifest(archive []byte, revision string) skillsUpdateManifest {
	hash := sha256.Sum256(archive)
	return skillsUpdateManifest{revision, "https://github.com/" + skillsRepo + "/releases/download/skills-" + revision + "/skills.tar.gz", hex.EncodeToString(hash[:]), "0.1.121"}
}

func fixtureUpdater(t *testing.T, home string, handler http.Handler) skillsUpdater {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	updater := skillsUpdater{home: home, version: "0.1.121", env: noEnv, now: time.Now, client: newSkillsUpdateClient()}
	updater.client.Transport = fixtureSkillsTransport(server.URL)
	return updater
}

// Tests keep the production trust contract: only the injected transport maps
// the fixed GitHub addresses to a local fixture; there is no runtime URL knob.
func fixtureSkillsTransport(address string) http.RoundTripper {
	return skillsRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" || request.URL.User != nil {
			panic("credentials on skills update request")
		}
		copy := request.Clone(request.Context())
		copy.URL, _ = url.Parse(address)
		if request.URL.String() == skillsManifestURL {
			copy.URL.Path = "/manifest"
		} else {
			copy.URL.Path = "/bundle"
		}
		copy.Host = ""
		return http.DefaultTransport.RoundTrip(copy)
	})
}

func TestSkillsUpdaterInstallsVerifiedRevisionAndThrottles(t *testing.T) {
	home := resolvedTempDir(t)
	archive := buildSkillsArchive(t, testSkillsEntries())
	manifest := updateManifest(archive, strings.Repeat("a", 40))
	var manifests, bundles atomic.Int32
	updater := fixtureUpdater(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest" {
			manifests.Add(1)
			_ = json.NewEncoder(w).Encode(manifest)
		} else {
			bundles.Add(1)
			_, _ = w.Write(archive)
		}
	}))
	require.NoError(t, updater.check(context.Background(), true))
	state, err := readSkillsUpdateState(home)
	require.NoError(t, err)
	assert.Equal(t, manifest.Revision, state.InstalledRevisions["blaxel-cli"])
	assert.Equal(t, manifest.Revision, state.VerifiedRevision)
	assert.False(t, state.LastSuccessfulCheck.IsZero())
	assert.FileExists(t, filepath.Join(home, ".agents", "skills", "blaxel-sdk", "SKILL.md"))
	require.NoError(t, updater.check(context.Background(), false))
	assert.EqualValues(t, 1, manifests.Load())
	state.LastAttempt = time.Now().Add(-skillsUpdateInterval)
	require.NoError(t, writeSkillsUpdateState(home, state))
	require.NoError(t, updater.check(context.Background(), false))
	assert.EqualValues(t, 2, manifests.Load())
	assert.EqualValues(t, 1, bundles.Load(), "the same revision is not downloaded again")
}

func TestSkillsExplicitInstallInvalidatesChangedBundleRevision(t *testing.T) {
	home := resolvedTempDir(t)
	archive := buildSkillsArchive(t, testSkillsEntries())
	manifest := updateManifest(archive, strings.Repeat("a", 40))
	var bundles atomic.Int32
	updater := fixtureUpdater(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest" {
			_ = json.NewEncoder(w).Encode(manifest)
		} else {
			bundles.Add(1)
			_, _ = w.Write(archive)
		}
	}))
	require.NoError(t, updater.check(context.Background(), true))
	explicit := buildSkillsArchive(t, []archiveEntry{
		{name: "skills/blaxel-cli/SKILL.md", body: skillManifest("blaxel-cli") + "Explicitly refreshed content.\n"},
	})
	_, err := installSkillsArchive(explicit, home, noEnv, nil, time.Now())
	require.NoError(t, err)
	assert.Contains(t, readTestFile(t, filepath.Join(home, ".agents", "skills", "blaxel-cli", "SKILL.md")), "Explicitly refreshed")
	state, err := readSkillsUpdateState(home)
	require.NoError(t, err)
	assert.Empty(t, state.InstalledRevisions["blaxel-cli"])
	assert.Equal(t, manifest.Revision, state.InstalledRevisions["blaxel-sdk"], "untouched skill retains its revision")
	assert.Empty(t, state.VerifiedRevision)
	updater.now = func() time.Time { return state.LastAttempt.Add(skillsUpdateInterval) }
	require.NoError(t, updater.check(context.Background(), false))
	assert.EqualValues(t, 2, bundles.Load(), "a due check reconciles the same channel revision")
	state, err = readSkillsUpdateState(home)
	require.NoError(t, err)
	assert.Equal(t, manifest.Revision, state.InstalledRevisions["blaxel-cli"])
	assert.NotContains(t, readTestFile(t, filepath.Join(home, ".agents", "skills", "blaxel-cli", "SKILL.md")), "Explicitly refreshed")
	// A check that preserved every local edit can verify a revision without
	// recording any installed revision. Explicit installs must clear that memo too.
	state.InstalledRevisions = nil
	require.NoError(t, writeSkillsUpdateState(home, state))
	_, err = installSkillsArchive(explicit, home, noEnv, nil, time.Now())
	require.NoError(t, err)
	state, err = readSkillsUpdateState(home)
	require.NoError(t, err)
	assert.Empty(t, state.VerifiedRevision)
}

func TestSkillsUpdaterFailuresLeaveContentsAndBackOff(t *testing.T) {
	archive := buildSkillsArchive(t, testSkillsEntries())
	for _, scenario := range []string{"checksum", "bad archive", "invalid manifest", "oversized manifest", "timeout", "missing manifest", "incompatible", "unknown version", "prerelease", "offline"} {
		t.Run(scenario, func(t *testing.T) {
			home := resolvedTempDir(t)
			_, err := installSkillsArchive(archive, home, noEnv, nil, time.Now())
			require.NoError(t, err)
			before := readTestFile(t, filepath.Join(home, ".agents", "skills", "blaxel-cli", "SKILL.md"))
			lock := readTestFile(t, skillsLockPath(home, noEnv))
			bundle := archive
			if scenario == "bad archive" {
				bundle = []byte("not tar")
			}
			manifest := updateManifest(bundle, strings.Repeat("b", 40))
			if scenario == "checksum" {
				manifest.SHA256 = strings.Repeat("0", 64)
			}
			if scenario == "incompatible" {
				manifest.MinimumCLI = "999.0.0"
			}
			var requests atomic.Int32
			updater := fixtureUpdater(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path != "/manifest" {
					_, _ = w.Write(bundle)
					return
				}
				switch scenario {
				case "missing manifest":
					http.NotFound(w, r)
				case "timeout":
					<-r.Context().Done()
				case "offline":
					w.WriteHeader(http.StatusServiceUnavailable)
				case "invalid manifest":
					_, _ = io.WriteString(w, "broken")
				case "oversized manifest":
					_, _ = io.WriteString(w, strings.Repeat(" ", skillsManifestMaxBytes+1))
				default:
					_ = json.NewEncoder(w).Encode(manifest)
				}
			}))
			if scenario == "unknown version" {
				updater.version = "dev"
			}
			if scenario == "prerelease" {
				updater.version = "0.1.121-rc1"
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err = updater.check(ctx, false)
			if scenario == "missing manifest" || scenario == "incompatible" || scenario == "unknown version" || scenario == "prerelease" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			state, err := readSkillsUpdateState(home)
			require.NoError(t, err)
			assert.False(t, state.LastAttempt.IsZero())
			assert.Empty(t, state.VerifiedRevision)
			assert.NotEmpty(t, state.Skip+state.Failure)
			require.NoError(t, updater.check(context.Background(), false))
			assert.LessOrEqual(t, requests.Load(), int32(2), "one attempt, without retrying a failed manifest")
			assert.Equal(t, before, readTestFile(t, filepath.Join(home, ".agents", "skills", "blaxel-cli", "SKILL.md")))
			assert.Equal(t, lock, readTestFile(t, skillsLockPath(home, noEnv)))
		})
	}
}

func TestSkillsUpdaterPreservesEditedOrdinaryAndManagedSkills(t *testing.T) {
	home := resolvedTempDir(t)
	root := filepath.Join(home, ".agents", "skills")
	archive := buildSkillsArchive(t, testSkillsEntries())
	_, err := installSkillsArchive(archive, home, noEnv, nil, time.Now())
	require.NoError(t, err)
	writeTestFile(t, filepath.Join(root, "blaxel-cli", "local.txt"), "local changes")
	require.NoError(t, os.RemoveAll(filepath.Join(root, "blaxel-sdk")))
	external := filepath.Join(home, "managed", "sdk")
	writeTestFile(t, filepath.Join(external, "SKILL.md"), skillManifest("blaxel-sdk")+"external fork")
	managedSkillLink(t, external, filepath.Join(root, "blaxel-sdk"))
	plugin := filepath.Join(home, ".claude", "plugins", "cache", "blaxel", "skills", "blaxel-cli", "SKILL.md")
	writeTestFile(t, plugin, "plugin-owned")
	ordinary := filepath.Join(home, ".claude", "skills", "blaxel-cli", "SKILL.md")
	writeTestFile(t, ordinary, skillManifest("blaxel-cli")+"agent-specific edits")
	beforeLock := readTestFile(t, skillsLockPath(home, noEnv))
	manifest := updateManifest(archive, strings.Repeat("c", 40))
	updater := fixtureUpdater(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest" {
			_ = json.NewEncoder(w).Encode(manifest)
		} else {
			_, _ = w.Write(archive)
		}
	}))
	require.NoError(t, updater.check(context.Background(), true))
	state, err := readSkillsUpdateState(home)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"blaxel-cli", "blaxel-sdk"}, state.SkippedSkills)
	assert.Empty(t, state.InstalledRevisions)
	assert.Equal(t, "local changes", readTestFile(t, filepath.Join(root, "blaxel-cli", "local.txt")))
	assert.Equal(t, beforeLock, readTestFile(t, skillsLockPath(home, noEnv)))
	assert.Equal(t, skillManifest("blaxel-sdk")+"external fork", readTestFile(t, filepath.Join(external, "SKILL.md")))
	assert.Equal(t, "plugin-owned", readTestFile(t, plugin))
	assert.Equal(t, skillManifest("blaxel-cli")+"agent-specific edits", readTestFile(t, ordinary))
}

func TestSkillsUpdaterOptOutAndExplicitRefresh(t *testing.T) {
	archive := buildSkillsArchive(t, testSkillsEntries())
	for _, scenario := range []string{"saved", "CI", "BL_INSTALL_SKILLS"} {
		t.Run(scenario, func(t *testing.T) {
			home := resolvedTempDir(t)
			var requests atomic.Int32
			updater := fixtureUpdater(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path == "/manifest" {
					_ = json.NewEncoder(w).Encode(updateManifest(archive, strings.Repeat("d", 40)))
				} else {
					_, _ = w.Write(archive)
				}
			}))
			enabled := false
			if scenario == "saved" {
				require.NoError(t, writeSkillsUpdateState(home, skillsUpdateState{AutoUpdate: &enabled}))
			} else {
				updater.env = func(key string) string {
					if key == scenario {
						if key == "CI" {
							return "true"
						}
						return "false"
					}
					return ""
				}
			}
			require.NoError(t, updater.check(context.Background(), false))
			assert.Zero(t, requests.Load())
			require.NoError(t, updater.check(context.Background(), true))
			assert.EqualValues(t, 2, requests.Load())
			state, err := readSkillsUpdateState(home)
			require.NoError(t, err)
			if scenario == "saved" {
				require.NotNil(t, state.AutoUpdate)
				assert.False(t, *state.AutoUpdate)
			}
		})
	}
}

func TestSkillsSavedOptOutKeepsAutomaticRefreshMCPIndependent(t *testing.T) {
	home := resolvedTempDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".cursor"), 0755))
	enabled := false
	require.NoError(t, writeSkillsUpdateState(home, skillsUpdateState{AutoUpdate: &enabled}))
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{skillsInstallEnv: "true"}, recorder)
	runSetupRefresh(context.Background(), options)
	assert.Empty(t, recorder.skillsAgents, "saved preference wins over automatic installer refresh")
	assert.NotNil(t, mcpTargets["cursor"].entry(options.mcp, "blaxel"), "MCP refresh remains independent")
	assert.Empty(t, recorder.logins)
	assert.Empty(t, recorder.tracking)
}

func TestSkillsManifestTrustAndRedirects(t *testing.T) {
	manifest := updateManifest([]byte("x"), strings.Repeat("e", 40))
	require.NoError(t, manifest.validate())
	for _, address := range []string{"http://github.com/" + skillsRepo + "/bundle", "https://github.com/other/repo/releases/download/x/skills.tar.gz", manifest.BundleURL + "?token=secret", "https://user:secret@github.com/" + skillsRepo, "https://example.org/skills.tar.gz"} {
		copy := manifest
		copy.BundleURL = address
		assert.Error(t, copy.validate())
	}
	client := newSkillsUpdateClient()
	from, _ := http.NewRequest(http.MethodGet, manifest.BundleURL, nil)
	for _, address := range []string{"http://release-assets.githubusercontent.com/file", "https://example.org/file", "https://release-assets.githubusercontent.com:443/file", "https://user:secret@release-assets.githubusercontent.com/file"} {
		to, _ := http.NewRequest(http.MethodGet, address, nil)
		assert.Error(t, client.CheckRedirect(to, []*http.Request{from}))
	}
	to, _ := http.NewRequest(http.MethodGet, "https://release-assets.githubusercontent.com/file?signed=fixture", nil)
	assert.NoError(t, client.CheckRedirect(to, []*http.Request{from}))
	from, _ = http.NewRequest(http.MethodGet, skillsManifestURL, nil)
	assert.Error(t, client.CheckRedirect(to, []*http.Request{from}))
}

// Run the same updater in separate OS processes against one home to exercise
// real kernel locking, not just goroutine synchronization.
func TestSkillsUpdateProcess(t *testing.T) {
	address := os.Getenv("BL_TEST_SKILLS_SERVER")
	if address == "" {
		t.Skip("subprocess helper")
	}
	updater := skillsUpdater{home: os.Getenv("BL_TEST_SKILLS_HOME"), version: "0.1.121", env: noEnv, now: time.Now, client: newSkillsUpdateClient()}
	updater.client.Transport = fixtureSkillsTransport(address)
	err := updater.check(context.Background(), false)
	if err != nil && err != errSkillsUpdateBusy {
		t.Fatal(err)
	}
}

func TestSkillsUpdaterCoordinatesProcesses(t *testing.T) {
	home := resolvedTempDir(t)
	archive := buildSkillsArchive(t, testSkillsEntries())
	_, err := installSkillsArchive(archive, home, noEnv, nil, time.Now())
	require.NoError(t, err)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/manifest" {
			time.Sleep(100 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(updateManifest(archive, strings.Repeat("f", 40)))
		} else {
			_, _ = w.Write(archive)
		}
	}))
	defer server.Close()
	var processes []*exec.Cmd
	for i := 0; i < 6; i++ {
		process := exec.Command(os.Args[0], "-test.run=^TestSkillsUpdateProcess$", "-test.count=1")
		process.Env = append(os.Environ(), "BL_TEST_SKILLS_SERVER="+server.URL, "BL_TEST_SKILLS_HOME="+home)
		require.NoError(t, process.Start())
		processes = append(processes, process)
	}
	for _, process := range processes {
		require.NoError(t, process.Wait())
	}
	assert.EqualValues(t, 2, requests.Load(), "one manifest and one bundle across all processes")
	state, err := readSkillsUpdateState(home)
	require.NoError(t, err)
	assert.NotEmpty(t, state.VerifiedRevision)
}

func TestSkillsLockHolderProcess(t *testing.T) {
	home := os.Getenv("BL_TEST_SKILLS_LOCK_HOME")
	if home == "" {
		t.Skip("subprocess helper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := withSkillsUpdateLock(ctx, home, true, func() error {
		if err := os.WriteFile(filepath.Join(home, "lock-ready"), nil, 0600); err != nil {
			return err
		}
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSkillsLockReleasesAfterCrashAndBoundsWait(t *testing.T) {
	home := resolvedTempDir(t)
	process := exec.Command(os.Args[0], "-test.run=^TestSkillsLockHolderProcess$")
	process.Env = append(os.Environ(), "BL_TEST_SKILLS_LOCK_HOME="+home)
	require.NoError(t, process.Start())
	t.Cleanup(func() { _ = process.Process.Kill() })
	require.Eventually(t, func() bool { _, err := os.Stat(filepath.Join(home, "lock-ready")); return err == nil }, 3*time.Second, 10*time.Millisecond)
	assert.ErrorIs(t, withSkillsUpdateLock(context.Background(), home, false, func() error { t.Fatal("entered held lock"); return nil }), errSkillsUpdateBusy)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, withSkillsUpdateLock(ctx, home, true, func() error { t.Fatal("entered held lock"); return nil }), context.DeadlineExceeded)
	require.NoError(t, process.Process.Kill())
	_ = process.Wait()
	require.NoError(t, withSkillsUpdateLock(context.Background(), home, false, func() error { return nil }))
}

func TestSkillsUpdateSwapRechecksLocalContents(t *testing.T) {
	home := resolvedTempDir(t)
	destination := filepath.Join(home, ".agents", "skills", "blaxel-cli")
	writeTestFile(t, filepath.Join(destination, "SKILL.md"), skillManifest("blaxel-cli"))
	baseline, err := localSkillHash(destination)
	require.NoError(t, err)
	writeTestFile(t, filepath.Join(destination, "custom.txt"), "edited after planning")
	err = replaceSkillFolderChecked(destination, []skillFile{{path: "SKILL.md", data: []byte("replacement")}}, true, baseline)
	require.ErrorContains(t, err, "changed during update")
	assert.Equal(t, "edited after planning", readTestFile(t, filepath.Join(destination, "custom.txt")))
	assert.Equal(t, skillManifest("blaxel-cli"), readTestFile(t, filepath.Join(destination, "SKILL.md")))
	assert.Equal(t, []string{"skills"}, dirNames(t, filepath.Dir(filepath.Dir(destination))))
}

func TestSkillsUpdaterRunsPeriodicallyWithoutBlockingMCP(t *testing.T) {
	home := resolvedTempDir(t)
	archive := buildSkillsArchive(t, testSkillsEntries())
	_, err := installSkillsArchive(archive, home, noEnv, nil, time.Now())
	require.NoError(t, err)
	var clock atomic.Int64
	clock.Store(time.Now().Unix())
	var manifests atomic.Int32
	started, unblock := make(chan struct{}), make(chan struct{})
	var once sync.Once
	updater := fixtureUpdater(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest" {
			manifests.Add(1)
			once.Do(func() { close(started) })
			select {
			case <-unblock:
			case <-r.Context().Done():
				return
			}
			_ = json.NewEncoder(w).Encode(updateManifest(archive, strings.Repeat("a", 40)))
		} else {
			_, _ = w.Write(archive)
		}
	}))
	updater.now = func() time.Time { return time.Unix(clock.Load(), 0) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan struct{})
	go func() { defer close(finished); updater.run(ctx, 20*time.Millisecond) }()
	<-started
	// Exercise an actual stdio bridge while the skills HTTP fetch is blocked.
	var output bytes.Buffer
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	require.NoError(t, mcpbridge.Serve(ctx, "", input, &output))
	assert.JSONEq(t, `{"jsonrpc":"2.0","id":1,"result":{}}`, strings.TrimSpace(output.String()))
	close(unblock)
	require.Eventually(t, func() bool {
		state, err := readSkillsUpdateState(home)
		return err == nil && state.VerifiedRevision != ""
	}, 3*time.Second, 10*time.Millisecond)
	clock.Add(int64(skillsUpdateInterval.Seconds()))
	require.Eventually(t, func() bool { return manifests.Load() == 2 }, 3*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("updater did not stop")
	}
}
