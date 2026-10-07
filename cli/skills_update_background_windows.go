package cli

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func detachSkillsUpdateWorker(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS}
}
