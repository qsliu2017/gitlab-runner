package vault

import (
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
	"gitlab.com/gitlab-org/gitlab-runner/vault/secret"
)

type Vault interface {
	Connect(server config.VaultServer) error
	Authenticate(auth config.VaultAuth) error
	ReadSecrets(secrets config.VaultSecrets) error
}

func New(builder secret.Builder) Vault {
	return &vault{builder: builder}
}

type vault struct {
	client  client.Client
	builder secret.Builder

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

func (v *vault) Authenticate(auth config.VaultAuth) error {
	authenticator, err := getAuthenticator(auth)
	if err != nil {
		return errors.Wrap(err, "couldn't create authenticator")
	}

	tokenInfo, err := authenticator.Authenticate(v.client, auth)
	if err != nil {
		return errors.Wrap(err, "couldn't authenticate against Vault server")
	}

	v.client.SetToken(tokenInfo.Token)

	return nil
}

func (v *vault) ReadSecrets(secrets config.VaultSecrets) error {
	for _, sec := range secrets {
		reader, err := getSecretReader(sec)
		if err != nil {
			return errors.Wrapf(err, "couldn't create reader for secret %v", sec)
		}

		err = reader.Read(v.client, v.builder, sec.Path, sec)
		if err != nil {
			return errors.Wrapf(err, "couldn't read secret %v", sec)
		}
	}

	return nil
}

func PrepareVaultSecrets(builder secret.Builder, conf *config.Vault) error {
	return PrepareVaultSecretsWithService(New(builder), builder, conf)
}

func PrepareVaultSecretsWithService(vaultService Vault, builder secret.Builder, conf *config.Vault) error {
	if builder == nil {
		return errors.New("builder can't be nil")
	}

	err := vaultService.Connect(conf.Server)
	if err != nil {
		return errors.Wrap(err, "couldn't connect to vault")
	}

	err = vaultService.Authenticate(conf.Auth)
	if err != nil {
		return errors.Wrap(err, "couldn't authenticate in vault")
	}

	err = vaultService.ReadSecrets(conf.Secrets)
	if err != nil {
		return errors.Wrap(err, "couldn't read secrets from vault")
	}

	return nil
}
