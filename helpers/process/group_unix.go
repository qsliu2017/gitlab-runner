// +build darwin dragonfly freebsd linux netbsd openbsd

package process

import (
	"syscall"
)

func SetProcessGroup(cmd Commander) {
	// Create process group
	attr := cmd.SysProcAttr()
	if attr == nil {
		attr = new(syscall.SysProcAttr)
	}

	attr.Setpgid = true
	cmd.SetSysProcAttr(attr)
}
