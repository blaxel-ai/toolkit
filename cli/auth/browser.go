package auth

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
)

// openBrowser opens a URL in the default browser without waiting for it.
// Tests replace it.
var openBrowser = func(url string) error {
	cmd, err := browserCommand(runtime.GOOS, url, exec.LookPath, os.Getenv)
	if err != nil {
		return err
	}
	return cmd.Start()
}

// browserCommand returns the platform's URL opener. On Linux it needs a
// graphical session; over SSH or in a container the caller prints the URL.
func browserCommand(goos, url string, lookPath func(string) (string, error), getenv func(string) string) (*exec.Cmd, error) {
	switch goos {
	case "darwin":
		return exec.Command("open", url), nil
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url), nil
	}
	// WSL opens the Windows browser and needs no Linux display.
	if path, err := lookPath("wslview"); err == nil {
		return exec.Command(path, url), nil
	}
	if getenv("DISPLAY") == "" && getenv("WAYLAND_DISPLAY") == "" {
		return nil, errors.New("no graphical session to open a browser in")
	}
	path, err := lookPath("xdg-open")
	if err != nil {
		return nil, err
	}
	return exec.Command(path, url), nil
}
