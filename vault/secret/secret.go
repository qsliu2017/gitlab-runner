package secret

import (
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

type Builder interface {
	BuildSecret(secretSpec *config.VaultSecretKey, data interface{}) error
}
