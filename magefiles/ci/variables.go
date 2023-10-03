package ci

import "gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"

var (
	RegistryImage = mageutils.EnvOrDefault("CI_REGISTRY_IMAGE", "registry.gitlab.com/gitlab-runner")
)
