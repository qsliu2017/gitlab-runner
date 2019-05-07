package vault

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
	"gitlab.com/gitlab-org/gitlab-runner/vault/secret"
)

func TestGetSecret_UnknownSecretsReaderFactory(t *testing.T) {
	secretReader, err := getSecretReader(&config.VaultSecret{
		Type: config.VaultSecretType("unknown"),
	})

	assert.Nil(t, secretReader)
	assert.EqualError(t, err, `SecretReader factory for type "unknown" is not defined`)
}

func TestGetSecret_ValidConfiguration(t *testing.T) {
	secretReader, err := getSecretReader(&config.VaultSecret{
		Type: config.VaultSecretTypeKV1,
	})

	assert.IsType(t, &secret.KV1{}, secretReader)
	assert.NoError(t, err)
}
