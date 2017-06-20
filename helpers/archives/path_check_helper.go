package archives

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
)

func isPathAGitDirectory(path string) bool {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	if len(parts) > 0 && parts[0] == ".git" {
		return true
	}
	return false

}

func errorIfGitDirectory(path string) *os.PathError {
	if !isPathAGitDirectory(path) {
		return nil
	}

	return &os.PathError{
		Op:   ".git inside of archive",
		Path: path,
		Err:  errors.New("Trying to archive or extract .git path"),
	}
}

func doesPathsListContainGitDirectory(paths []string) bool {
	for _, path := range paths {
		if isPathAGitDirectory(path) {
			return true
		}
	}

	return false
}

func printGitArchiveWarning(operation string) {
	logrus.Warn(fmt.Sprintf("Part of .git directory is on the list of files to %s", operation))
	logrus.Warn("This may introduce unexpected problems")
}

func warnOnGitDirectory(operation string, paths []string) {
	if !doesPathsListContainGitDirectory(paths) {
		return
	}

	printGitArchiveWarning(operation)
}
