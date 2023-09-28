package variables

import "gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"

var variables = []string{}

func CIRegistryPassword() string {
	return mageutils.EnvOrDefault("CI_REGISTRY_PASSWORD", "")
}

func CIRegistryUser() string {
	return mageutils.EnvOrDefault("CI_REGISTRY_USER", "")
}
