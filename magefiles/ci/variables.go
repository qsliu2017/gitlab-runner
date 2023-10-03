package ci

import (
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"
)

func RegistryImage() string {
	return mageutils.EnvOrDefault("CI_REGISTRY_IMAGE", "registry.gitlab.com/gitlab-runner")
}

func Registry() string {
	return mageutils.EnvOrDefault("CI_REGISTRY", "registry.gitlab.com")
}

func RegistryUser() string {
	return mageutils.Env("CI_REGISTRY_USER")
}

func RegistryPassword() string {
	return mageutils.Env("CI_REGISTRY_PASSWORD")
}
