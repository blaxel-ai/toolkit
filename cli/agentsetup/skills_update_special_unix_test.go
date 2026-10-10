//go:build !windows

package agentsetup

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestSkillsUpdateRejectsFIFOManifestAndReleasesLock(t *testing.T) {
	home := t.TempDir()
	manifestPath := filepath.Join(home, ".agents", "skills", "blaxel-cli", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0755))
	require.NoError(t, unix.Mkfifo(manifestPath, 0600))
	archive := buildSkillsArchive(t, testSkillsEntries())
	manifest := updateManifest(archive, strings.Repeat("f", 40))
	updater := fixtureUpdater(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest" {
			_ = json.NewEncoder(w).Encode(manifest)
			return
		}
		_, _ = w.Write(archive)
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- updater.check(ctx, true) }()
	select {
	case err := <-finished:
		require.ErrorContains(t, err, "regular file")
	case <-time.After(time.Second):
		t.Fatal("skills update blocked on a FIFO manifest")
	}
	require.NoError(t, withSkillsUpdateLock(context.Background(), home, false, func() error { return nil }))
	info, err := os.Lstat(manifestPath)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeNamedPipe, "local special file remains unchanged")
}
