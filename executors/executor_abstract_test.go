package executors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestStartBuild(t *testing.T) {
	tests := []struct {
		name            string
		settings        common.RunnerSettings
		execOptions     ExecutorOptions
		concurrent      int
		ciProjectDir    string
		expectedRootDir string
		expectedErr     bool
	}{
		{
			name:     "default build dir",
			settings: common.RunnerSettings{},
			execOptions: ExecutorOptions{
				DefaultBuildsDir: "/builds",
			},
			concurrent:      1,
			ciProjectDir:    "",
			expectedRootDir: "/builds",
			expectedErr:     false,
		},
		{
			name: "config build dir",
			settings: common.RunnerSettings{
				BuildsDir: "/builds",
				CustomBuildDir: &common.CustomBuildDir{
					Enable: true,
				},
			},
			execOptions: ExecutorOptions{
				DefaultBuildsDir: "/builds/default",
			},
			concurrent:      1,
			ciProjectDir:    "",
			expectedRootDir: "/builds",
			expectedErr:     false,
		},
		{
			name: "CI_PROJECT_DIR takes precedence over configured dir",
			settings: common.RunnerSettings{
				BuildsDir: "/builds",
				CustomBuildDir: &common.CustomBuildDir{
					Enable: true,
				},
			},
			execOptions: ExecutorOptions{
				DefaultBuildsDir: "/builds/default",
			},
			concurrent:      1,
			ciProjectDir:    "/builds/job-specific-location",
			expectedRootDir: "/builds/job-specific-location",
			expectedErr:     false,
		},
		{
			name: "CI_PROJECT_DIR takes precedence over configured dir with concurrent over 1",
			settings: common.RunnerSettings{
				BuildsDir: "/builds",
				CustomBuildDir: &common.CustomBuildDir{
					Enable: true,
				},
			},
			execOptions: ExecutorOptions{
				DefaultBuildsDir: "/builds/default",
			},
			concurrent:      4,
			ciProjectDir:    "/builds/job-specific-location",
			expectedRootDir: "/builds/job-specific-location",
			expectedErr:     false,
		},
		{
			name: "CI_PROJECT_DIR specified custom build dir set to false in config",
			settings: common.RunnerSettings{
				BuildsDir: "/builds",
				CustomBuildDir: &common.CustomBuildDir{
					Enable: false,
				},
			},
			concurrent: 1,
			execOptions: ExecutorOptions{
				DefaultCustomBuildsDirEnabled: true,
				DefaultBuildsDir:              "/builds/default",
			},
			ciProjectDir:    "/builds/job-specific-location",
			expectedRootDir: "",
			expectedErr:     true,
		},
		{
			name: "CI_PROJECT_DIR specified custom build dir not defined in config when default is false",
			settings: common.RunnerSettings{
				BuildsDir:      "/builds",
				CustomBuildDir: nil,
			},
			concurrent: 1,
			execOptions: ExecutorOptions{
				DefaultCustomBuildsDirEnabled: false,
				DefaultBuildsDir:              "/builds/default",
			},
			ciProjectDir:    "/builds/job-specific-location",
			expectedRootDir: "",
			expectedErr:     true,
		},
		{
			name: "CI_PROJECT_DIR specified custom build dir not defined in config when default is true",
			settings: common.RunnerSettings{
				BuildsDir:      "/builds",
				CustomBuildDir: nil,
			},
			concurrent: 1,
			execOptions: ExecutorOptions{
				DefaultCustomBuildsDirEnabled: true,
				DefaultBuildsDir:              "/builds/default",
			},
			ciProjectDir:    "/builds/job-specific-location",
			expectedRootDir: "/builds/job-specific-location",
			expectedErr:     false,
		},
		{
			name: "CI_PROJECT_DIR specified when shared dir is true with 1 concurrent runner",
			settings: common.RunnerSettings{
				BuildsDir: "/builds",
				CustomBuildDir: &common.CustomBuildDir{
					Enable: true,
				},
			},
			concurrent: 1,
			execOptions: ExecutorOptions{
				DefaultBuildsDir: "/builds/default",
				SharedBuildsDir:  true,
			},
			ciProjectDir:    "/builds/job-specific/location",
			expectedRootDir: "/builds/job-specific/location",
			expectedErr:     false,
		},
		{
			name: "CI_PROJECT_DIR specified when share dir is true and more then 1 concurrent runner",
			settings: common.RunnerSettings{
				BuildsDir: "/builds",
				CustomBuildDir: &common.CustomBuildDir{
					Enable: true,
				},
			},
			concurrent: 4,
			execOptions: ExecutorOptions{
				DefaultBuildsDir: "/builds/default",
				SharedBuildsDir:  true,
			},
			ciProjectDir:    "/builds/job-specific/location",
			expectedRootDir: "/builds/job-specific/location",
			expectedErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := AbstractExecutor{
				ExecutorOptions: tt.execOptions,
				Config: common.RunnerConfig{
					RunnerSettings: tt.settings,
				},
				Build: &common.Build{
					Runner: &common.RunnerConfig{
						RunnerSettings: tt.settings,
					},
					Concurrent: tt.concurrent,
				},
			}

			if tt.ciProjectDir != "" {
				e.Build.Variables = common.JobVariables{
					{Key: "CI_PROJECT_DIR", Value: tt.ciProjectDir, Public: true, Internal: true, File: false},
				}
			}

			err := e.startBuild()
			if tt.expectedErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedRootDir, e.Build.RootDir)
		})
	}
}
