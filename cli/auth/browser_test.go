package auth

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserCommand(t *testing.T) {
	const url = "https://app.blaxel.ai/device?code=ABC"
	found := func(names ...string) func(string) (string, error) {
		return func(name string) (string, error) {
			for _, n := range names {
				if n == name {
					return "/usr/bin/" + name, nil
				}
			}
			return "", errors.New("not found")
		}
	}
	env := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}

	cmd, err := browserCommand("darwin", url, found(), env(nil))
	require.NoError(t, err)
	assert.Equal(t, []string{"open", url}, cmd.Args)

	cmd, err = browserCommand("windows", url, found(), env(nil))
	require.NoError(t, err)
	assert.Equal(t, []string{"rundll32", "url.dll,FileProtocolHandler", url}, cmd.Args)

	cmd, err = browserCommand("linux", url, found("xdg-open"), env(map[string]string{"DISPLAY": ":0"}))
	require.NoError(t, err)
	assert.Equal(t, []string{"/usr/bin/xdg-open", url}, cmd.Args)

	cmd, err = browserCommand("linux", url, found("xdg-open"), env(map[string]string{"WAYLAND_DISPLAY": "wayland-0"}))
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/xdg-open", cmd.Path)

	cmd, err = browserCommand("linux", url, found("wslview", "xdg-open"), env(nil))
	require.NoError(t, err)
	assert.Equal(t, []string{"/usr/bin/wslview", url}, cmd.Args, "WSL needs no Linux display")

	_, err = browserCommand("linux", url, found("xdg-open"), env(nil))
	assert.Error(t, err, "headless sessions print the URL instead")

	_, err = browserCommand("freebsd", url, found(), env(map[string]string{"DISPLAY": ":0"}))
	assert.Error(t, err)
}
