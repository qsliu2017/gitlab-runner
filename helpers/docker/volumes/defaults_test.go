package volumes

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/errors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/volumes/parser"
)

func TestGetDefaultCacheVolume(t *testing.T) {
	testCases := map[string]struct {
		expectedVolume parser.Volume
		expectedError  interface{}
	}{
		common.OSTypeLinux: {
			expectedVolume: parser.Volume{Destination: "/cache"},
			expectedError:  nil,
		},
		common.OSTypeWindows: {
			expectedVolume: parser.Volume{Destination: `c:\cache`},
			expectedError:  nil,
		},
		"unsupported": {
			expectedVolume: parser.Volume{},
			expectedError:  errors.NewUnsupportedOSTypeError("unsupported"),
		},
	}

	for osType, testCase := range testCases {
		t.Run(osType, func(t *testing.T) {
			v, err := GetDefaultCacheVolume(types.Info{OSType: osType})

			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, testCase.expectedVolume, v)
		})
	}
}
