package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// homebrewSkillsLocation recognizes the resolved keg path without invoking brew
// on every command. It supports custom prefixes and both binary aliases.
func homebrewSkillsLocation(executable string) (prefix, version string) {
	executable = filepath.Clean(executable)
	if !filepath.IsAbs(executable) || (filepath.Base(executable) != "blaxel" && filepath.Base(executable) != "bl") {
		return "", ""
	}
	bin := filepath.Dir(executable)
	keg := filepath.Dir(bin)
	rack := filepath.Dir(keg)
	cellar := filepath.Dir(rack)
	if filepath.Base(bin) != "bin" || filepath.Base(rack) != "blaxel" || filepath.Base(cellar) != "Cellar" {
		return "", ""
	}
	return filepath.Dir(cellar), filepath.Base(keg)
}

func installHomebrewSkills() {
	if skillsInstallDisabled(os.Getenv) {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	setupHomebrewSkills(executable, installSkills)
}

// setupHomebrewSkills records an attempt before installing, so concurrent
// launches and failures cannot repeatedly delay commands. A failed attempt can
// be retried explicitly with bl upgrade or the printed npx command.
func setupHomebrewSkills(executable string, install func()) {
	if skillsInstallDisabled(os.Getenv) {
		return
	}
	realPath, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return
	}
	_, version := homebrewSkillsLocation(realPath)
	if version == "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	marker := filepath.Join(home, ".blaxel", "skills", "homebrew", version)
	claimed, err := claimSkillsInstall(marker)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not save Blaxel skills setup state:", err)
		fmt.Fprintln(os.Stderr, "You can install the skills with:", skillsInstallCommand())
		return
	}
	if claimed {
		install()
	}
}

func claimSkillsInstall(marker string) (bool, error) {
	file, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(marker), 0700); err != nil {
			return false, err
		}
		file, err = os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	}
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, file.Close()
}

// The running binary still belongs to the old keg after brew upgrades it.
// Follow Homebrew's stable opt link to mark the newly installed version too.
func markUpgradedHomebrewSkills() {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	markHomebrewSkillsForExecutable(executable)
}

func markHomebrewSkillsForExecutable(executable string) {
	if realPath, err := filepath.EvalSymlinks(executable); err == nil {
		executable = realPath
	}
	prefix, _ := homebrewSkillsLocation(executable)
	if prefix != "" {
		setupHomebrewSkills(filepath.Join(prefix, "opt", "blaxel", "bin", "blaxel"), func() {})
	}
}
