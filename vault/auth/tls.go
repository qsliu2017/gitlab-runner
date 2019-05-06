package auth

import (
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

type TLS struct{}

func NewTLS() *TLS {
	return new(TLS)
}

func (t *TLS) Authenticate(cli client.Client, authConfig config.VaultAuth) (client.TokenInfo, error) {
	conf := authConfig.TLS

	tokenInfo, err := cli.TLSLogin(conf.GetPath(), conf.Name, conf.TLSCertFile, conf.TLSKeyFile)
	if err != nil {
		return client.TokenInfo{}, errors.Wrap(err, "couldn't authenticate with TLS method")
	}

	return tokenInfo, nil
}
