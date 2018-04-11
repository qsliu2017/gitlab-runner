package commands

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker/machine" // Register docker+machine as executor
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"     // Register kubernetes as executor
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
)

func setupDockerRegisterCommand(dockerConfig *common.DockerConfig) *RegisterCommand {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	ctx := cli.NewContext(cli.NewApp(), fs, nil)
	fs.String("docker-image", "ruby:2.1", "")

	s := &RegisterCommand{
		context:        ctx,
		NonInteractive: true,
	}
	s.Docker = dockerConfig

	return s
}

func TestRegisterDefaultDockerCacheVolume(t *testing.T) {
	s := setupDockerRegisterCommand(&common.DockerConfig{
		Volumes: []string{},
	})

	s.askDocker()

	assert.Equal(t, 1, len(s.Docker.Volumes))
	assert.Equal(t, "/cache", s.Docker.Volumes[0])
}

func TestRegisterCustomDockerCacheVolume(t *testing.T) {
	s := setupDockerRegisterCommand(&common.DockerConfig{
		Volumes: []string{"/cache"},
	})

	s.askDocker()

	assert.Equal(t, 1, len(s.Docker.Volumes))
	assert.Equal(t, "/cache", s.Docker.Volumes[0])
}

func TestRegisterCustomMappedDockerCacheVolume(t *testing.T) {
	s := setupDockerRegisterCommand(&common.DockerConfig{
		Volumes: []string{"/my/cache:/cache"},
	})

	s.askDocker()

	assert.Equal(t, 1, len(s.Docker.Volumes))
	assert.Equal(t, "/my/cache:/cache", s.Docker.Volumes[0])
}

func TestDefaultExecutorConfiguration(t *testing.T) {
	tests := []struct {
		executor       string
		expectedConfig common.RunnerConfig
	}{
		{
			executor: "kubernetes",
			expectedConfig: common.RunnerConfig{
				Name: "ci-test",
				RunnerCredentials: common.RunnerCredentials{
					URL:   "https://gitlab.com/",
					Token: "test-token",
				},
				RunnerSettings: common.RunnerSettings{
					Executor: "kubernetes",
					CustomBuildDir: &common.CustomBuildDir{
						Enable: true,
					},
				},
			},
		},
		{
			executor: "docker+machine",
			expectedConfig: common.RunnerConfig{
				Name: "ci-test",
				RunnerCredentials: common.RunnerCredentials{
					URL:   "https://gitlab.com/",
					Token: "test-token",
				},
				RunnerSettings: common.RunnerSettings{
					Executor: "docker+machine",
					CustomBuildDir: &common.CustomBuildDir{
						Enable: true,
					},
					Docker: &common.DockerConfig{
						Image:   "ruby:2.1",
						Volumes: []string{"/cache"},
					},
				},
			},
		},
		{
			executor: "docker-ssh+machine",
			expectedConfig: common.RunnerConfig{
				Name: "ci-test",
				RunnerCredentials: common.RunnerCredentials{
					URL:   "https://gitlab.com/",
					Token: "test-token",
				},
				RunnerSettings: common.RunnerSettings{
					Executor: "docker-ssh+machine",
					CustomBuildDir: &common.CustomBuildDir{
						Enable: true,
					},
					Docker: &common.DockerConfig{
						Image:   "ruby:2.1",
						Volumes: []string{"/cache"},
					},
					SSH: &ssh.Config{
						User:         "user",
						Password:     "password",
						IdentityFile: "/home/user/.ssh/id_rsa",
					},
				},
			},
		},
		{
			executor: "docker",
			expectedConfig: common.RunnerConfig{
				Name: "ci-test",
				RunnerCredentials: common.RunnerCredentials{
					URL:   "https://gitlab.com/",
					Token: "test-token",
				},
				RunnerSettings: common.RunnerSettings{
					Executor: "docker",
					CustomBuildDir: &common.CustomBuildDir{
						Enable: true,
					},
					Docker: &common.DockerConfig{
						Image:   "ruby:2.1",
						Volumes: []string{"/cache"},
					},
				},
			},
		},
		{
			executor: "docker-ssh",
			expectedConfig: common.RunnerConfig{
				Name: "ci-test",
				RunnerCredentials: common.RunnerCredentials{
					URL:   "https://gitlab.com/",
					Token: "test-token",
				},
				RunnerSettings: common.RunnerSettings{
					Executor: "docker-ssh",
					CustomBuildDir: &common.CustomBuildDir{
						Enable: true,
					},
					Docker: &common.DockerConfig{
						Image:   "ruby:2.1",
						Volumes: []string{"/cache"},
					},
					SSH: &ssh.Config{
						User:         "user",
						Password:     "password",
						IdentityFile: "/home/user/.ssh/id_rsa",
					},
				},
			},
		},
		{
			executor: "ssh",
			expectedConfig: common.RunnerConfig{
				Name: "ci-test",
				RunnerCredentials: common.RunnerCredentials{
					URL:   "https://gitlab.com/",
					Token: "test-token",
				},
				RunnerSettings: common.RunnerSettings{
					Executor: "ssh",
					SSH: &ssh.Config{
						User:         "user",
						Password:     "password",
						Host:         "my.server.com",
						Port:         "22",
						IdentityFile: "/home/user/.ssh/id_rsa",
					},
				},
			},
		},
		{
			executor: "parallels",
			expectedConfig: common.RunnerConfig{
				Name: "ci-test",
				RunnerCredentials: common.RunnerCredentials{
					URL:   "https://gitlab.com/",
					Token: "test-token",
				},
				RunnerSettings: common.RunnerSettings{
					Executor: "parallels",
					SSH: &ssh.Config{
						Host: "my.server.com",
						Port: "22",
					},
					Parallels: &common.ParallelsConfig{
						BaseName: "my-parallels-vm",
					},
				},
			},
		},
		{
			executor: "virtualbox",
			expectedConfig: common.RunnerConfig{
				Name: "ci-test",
				RunnerCredentials: common.RunnerCredentials{
					URL:   "https://gitlab.com/",
					Token: "test-token",
				},
				RunnerSettings: common.RunnerSettings{
					Executor: "virtualbox",
					SSH: &ssh.Config{
						User:         "user",
						Password:     "password",
						IdentityFile: "/home/user/.ssh/id_rsa",
					},
					VirtualBox: &common.VirtualBoxConfig{
						BaseName: "my-virtualbox-vm",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.executor, func(t *testing.T) {
			fs := flag.NewFlagSet("", flag.ExitOnError)
			ctx := cli.NewContext(cli.NewApp(), fs, nil)
			fs.String("url", "https://gitlab.com/", "")
			fs.String("registration-token", "test-registration-token", "")
			fs.String("name", "ci-test", "")
			fs.String("tag-list", "ci,test", "")
			fs.String("executor", test.executor, "")
			fs.String("docker-image", "ruby:2.1", "")
			fs.String("ssh-user", "user", "")
			fs.String("ssh-password", "password", "")
			fs.String("ssh-identity-file", "/home/user/.ssh/id_rsa", "")
			fs.String("ssh-host", "my.server.com", "")
			fs.String("ssh-port", "22", "")
			fs.String("parallels-base-name", "my-parallels-vm", "")
			fs.String("virtualbox-base-name", "my-virtualbox-vm", "")

			registerRunnerRep := common.RegisterRunnerResponse{
				Token: "test-token",
			}
			mockNetwork := &common.MockNetwork{}
			mockNetwork.On("RegisterRunner", mock.Anything, mock.Anything).Return(&registerRunnerRep, true).Once()

			s := &RegisterCommand{
				context:        ctx,
				NonInteractive: true,
			}

			s.SSH = &ssh.Config{}
			s.Parallels = &common.ParallelsConfig{}
			s.VirtualBox = &common.VirtualBoxConfig{}
			s.network = mockNetwork
			s.Execute(ctx)

			assert.Equal(t, test.expectedConfig, s.RunnerConfig)
		})
	}
}
