package volumes

import (
	"path/filepath"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/volumes/parser"
)

func IsHostMountedVolume(volumeParser parser.Parser, dir string, volumes ...string) (bool, error) {
	for _, volume := range volumes {
		parsedVolume, err := volumeParser.ParseVolume(volume)
		if err != nil {
			return false, err
		}

		if parsedVolume.Len() < 2 {
			continue
		}

		if isParentOf(filepath.Clean(parsedVolume.Destination), filepath.Clean(dir)) {
			return true, nil
		}
	}
	return false, nil
}

func isParentOf(parent string, dir string) bool {
	for dir != "/" && dir != "." {
		if dir == parent {
			return true
		}
		dir = filepath.Dir(dir)
	}
	return false
}
