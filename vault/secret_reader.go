package vault

import (
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
	"gitlab.com/gitlab-org/gitlab-runner/vault/secret"
)

type SecretReaderFactory func() SecretReader

type SecretReader interface {
	Read(cli client.Client, builder secret.Builder, path string, secretSpec *config.VaultSecret) error
}

var secretReaderFactories = map[config.VaultSecretType]SecretReaderFactory{
	config.VaultSecretTypeKV1: func() SecretReader { return new(secret.KV1) },
	config.VaultSecretTypeKV2: func() SecretReader { return new(secret.KV2) },
}

func getSecretReader(secret *config.VaultSecret) (SecretReader, error) {
	readerFactory, ok := secretReaderFactories[secret.Type]
	if !ok {
		return nil, errors.Errorf("SecretReader factory for type %q is not defined", secret.Type)
	}

	return readerFactory(), nil
}
