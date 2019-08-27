package helpers

import (
	"os/exec"
	"strconv"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

func SetProcessGroup(cmd process.Commander) {
	// Not supported on Windows
}

func KillProcessGroup(cmd process.Commander) {
	if cmd == nil {
		return
	}

	process := cmd.Process()
	if process == nil {
		return
	}

	exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(process.Pid)).Run()
	process.Kill()
}
