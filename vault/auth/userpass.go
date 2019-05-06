package auth

import (
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

type Userpass struct{}

func NewUserpass() *Userpass {
	return new(Userpass)
}

func (u *Userpass) Authenticate(cli client.Client, authConfig config.VaultAuth) (client.TokenInfo, error) {
	conf := authConfig.Userpass

	tokenInfo, err := cli.UserpassLogin(conf.GetPath(), conf.Username, conf.Password)
	if err != nil {
		return client.TokenInfo{}, errors.Wrap(err, "couldn't authenticate with userpass method")
	}

	return tokenInfo, nil
}
