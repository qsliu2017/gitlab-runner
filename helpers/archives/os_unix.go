//go:build unix

package archives

import (
	"os"

	"golang.org/x/sys/unix"
)

func lchmod(name string, mode os.FileMode) error {
	err := unix.Fchmodat(unix.AT_FDCWD, name, uint32(mode), unix.AT_SYMLINK_NOFOLLOW)
	if err != nil {
		return &os.PathError{Op: "lchmod", Path: name, Err: err}
	}
	return nil
}
