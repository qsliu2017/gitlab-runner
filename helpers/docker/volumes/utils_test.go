package volumes

import (
	"runtime"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/volumes/parser"
)

type isHostMountedVolumeTestCases map[string]isHostMountedVolumeTestCase

type isHostMountedVolumeTestCase struct {
	dir            string
	volumes        []string
	expectedResult bool
	expectedError  error
}

func skipOnOsType(t *testing.T, osType string) {
	if runtime.GOOS != osType {
		return
	}

	t.Skipf("skipping the test because running on %q OS type", osType)
}

func testIsHostMountedVolume(t *testing.T, osType string, testCases isHostMountedVolumeTestCases) {
	t.Run(osType, func(t *testing.T) {
		for testName, testCase := range testCases {
			t.Run(testName, func(t *testing.T) {
				volumesParser, err := parser.New(types.Info{OSType: osType})
				require.NoError(t, err)

				result, err := IsHostMountedVolume(volumesParser, testCase.dir, testCase.volumes...)
				assert.Equal(t, testCase.expectedResult, result)
				if testCase.expectedError == nil {
					assert.NoError(t, err)
				} else {
					assert.EqualError(t, err, testCase.expectedError.Error())
				}
			})
		}
	})
}

func TestIsHostMountedVolume_Linux(t *testing.T) {
	skipOnOsType(t, common.OSTypeWindows)

	testCases := isHostMountedVolumeTestCases{
		"empty volumes": {
			dir:            "/test/to/checked/dir",
			volumes:        []string{},
			expectedResult: false,
		},
		"no host volumes": {
			dir:            "/test/to/checked/dir",
			volumes:        []string{"/tests/to"},
			expectedResult: false,
		},
		"dir not within volumes": {
			dir:            "/test/to/checked/dir",
			volumes:        []string{"/host:/root"},
			expectedResult: false,
		},
		"dir within volumes": {
			dir:            "/test/to/checked/dir",
			volumes:        []string{"/host:/test/to"},
			expectedResult: true,
		},
		"error on parsing": {
			dir:           "/test/to/checked/dir",
			volumes:       []string{""},
			expectedError: parser.NewInvalidVolumeSpecErr(""),
		},
	}

	testIsHostMountedVolume(t, common.OSTypeLinux, testCases)
}

func TestIsHostMountedVolume_Windows(t *testing.T) {
	skipOnOsType(t, common.OSTypeLinux)

	testCases := isHostMountedVolumeTestCases{
		"empty volumes": {
			dir:            `c:\test\to\checked\dir`,
			volumes:        []string{},
			expectedResult: false,
		},
		"no host volumes": {
			dir:            `c:\test\to\checked\dir`,
			volumes:        []string{`c:\test\to`},
			expectedResult: false,
		},
		"dir not within volumes": {
			dir:            `c:\test\to\checked\dir`,
			volumes:        []string{`c:\host:c:\destination`},
			expectedResult: false,
		},
		"dir within volumes": {
			dir:            `c:\test\to\checked\dir`,
			volumes:        []string{`c:\host:c:\test\to`},
			expectedResult: true,
		},
		"error on parsing": {
			dir:           `c:\test\to\checked\dir`,
			volumes:       []string{""},
			expectedError: parser.NewInvalidVolumeSpecErr(""),
		},
	}

	testIsHostMountedVolume(t, common.OSTypeWindows, testCases)
}
