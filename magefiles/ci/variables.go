package ci

import (
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
)

var (
	RegistryImage    = env.NewDefault("CI_REGISTRY_IMAGE", build.AppName)
	Registry         = env.NewDefault("CI_REGISTRY", "registry.gitlab.com")
	RegistryUser     = env.New("CI_REGISTRY_USER")
	RegistryPassword = env.New("CI_REGISTRY_PASSWORD")

	RegistryAuthBundle = env.Variables{
		Registry,
		RegistryUser,
		RegistryPassword,
	}
)
