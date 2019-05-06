package auth

import (
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

type Token struct{}

func NewToken() *Token {
	return new(Token)
}

func (t *Token) Authenticate(cli client.Client, authConfig config.VaultAuth) (client.TokenInfo, error) {
	cli.SetToken(authConfig.Token.Token)

	tokenInfo, err := cli.TokenLookupSelf()
	if err != nil {
		return client.TokenInfo{}, errors.Wrap(err, "couldn't self-lookup the token")
	}

	return tokenInfo, nil
}
