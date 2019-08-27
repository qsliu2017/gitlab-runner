// +build darwin dragonfly freebsd linux netbsd openbsd

package helpers

import (
	"syscall"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

func SetProcessGroup(cmd process.Commander) {
	// Create process group
	process.SetProcessGroup(cmd)
}

func KillProcessGroup(cmd process.Commander) {
	if cmd == nil {
		return
	}

	process := cmd.Process()
	if process != nil {
		if process.Pid > 0 {
			syscall.Kill(-process.Pid, syscall.SIGKILL)
		} else {
			// doing normal kill
			process.Kill()
		}
	}
}
