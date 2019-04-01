package volumes

import (
	"github.com/docker/docker/api/types"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/errors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/volumes/parser"
)

var cacheVolumeDestinations = map[string]string{
	common.OSTypeLinux:   "/cache",
	common.OSTypeWindows: `c:\cache`,
}

func GetDefaultCacheVolume(info types.Info) (parser.Volume, error) {
	destination, ok := cacheVolumeDestinations[info.OSType]
	if !ok {
		return parser.Volume{}, errors.NewUnsupportedOSTypeError(info.OSType)
	}

	return parser.Volume{Destination: destination}, nil
}
