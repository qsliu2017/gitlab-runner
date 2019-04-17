package auth

import (
	"bytes"

	"github.com/docker/docker/api/types"
)

type Credential struct {
	URL      string
	Username string
	Password string
}

func GetBuildAuthConfigurations(credentials []Credential) map[string]types.AuthConfig {
	authConfigs := make(map[string]types.AuthConfig)

	for _, credential := range credentials {
		authConfigs[credential.URL] = types.AuthConfig{
			Username:      credential.Username,
			Password:      credential.Password,
			ServerAddress: credential.URL,
		}
	}

	return authConfigs
}

func GetBuildAuthConfiguration(indexName string, credentials []Credential) *types.AuthConfig {
	return ResolveDockerAuthConfig(indexName, GetBuildAuthConfigurations(credentials))
}

func GetUserAuthConfigurations(authConfig string) map[string]types.AuthConfig {
	buf := bytes.NewBufferString(authConfig)
	authConfigs, _ := ReadAuthConfigsFromReader(buf)
	return authConfigs
}

func GetUserAuthConfiguration(indexName string) *types.AuthConfig {
	return ResolveDockerAuthConfig(indexName, GetUserAuthConfigurations())
}
