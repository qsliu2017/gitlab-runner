package vault

import (
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

type Vault interface {
	Connect(server config.VaultServer) error
}

func New() Vault {
	return new(vault)
}

type vault struct {
	client client.Client

	serverConfig config.VaultServer
}

func (v *vault) Connect(server config.VaultServer) error {
	v.serverConfig = server

	cli, err := v.getClient()
	if err != nil {
		return errors.Wrap(err, "couldn't connect Vault client to the Vault server")
	}

	isReady := cli.IsServerReady()
	if !isReady.State {
		errorMessage := "Vault server is not ready to receive connections"
		if isReady.Err != nil {
			return errors.Wrap(isReady.Err, errorMessage)
		}

		return errors.New(errorMessage)
	}

	return nil
}

var newClient = client.New

func (v *vault) getClient() (client.Client, error) {
	if v.client != nil {
		return v.client, nil
	}

	cli, err := newClient(v.serverConfig)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't initialize Vault client")
	}

	v.client = cli

	return v.client, nil
}
