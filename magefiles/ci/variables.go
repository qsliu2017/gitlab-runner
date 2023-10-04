package ci

import (
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
)

const (
	EnvRegistryImage    = "CI_REGISTRY_IMAGE"
	EnvRegistry         = "CI_REGISTRY"
	EnvRegistryUser     = "CI_REGISTRY_USER"
	EnvRegistryPassword = "CI_REGISTRY_PASSWORD"
)

var (
	RegistryImage    = env.NewDefault(EnvRegistryImage, build.AppName)
	Registry         = env.NewDefault(EnvRegistry, "registry.gitlab.com")
	RegistryUser     = env.New(EnvRegistryUser)
	RegistryPassword = env.New(EnvRegistryPassword)
)
