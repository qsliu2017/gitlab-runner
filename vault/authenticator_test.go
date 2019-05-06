package vault

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/vault/auth"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

func TestGetAuthenticator_NoAuthenticationConfigured(t *testing.T) {
	authenticator, err := getAuthenticator(config.VaultAuth{})
	assert.Nil(t, authenticator)
	assert.EqualError(t, err, "couldn't detect configured authentication method: missing authentication method configuration")
}

func TestGetAuthenticator_MissingFactoryForAuthenticationMethod(t *testing.T) {
	oldAuthenticatorFactories := authenticatorFactories
	defer func() {
		authenticatorFactories = oldAuthenticatorFactories
	}()
	authenticatorFactories = map[reflect.Type]AuthenticatorFactory{}

	authenticator, err := getAuthenticator(config.VaultAuth{
		Token: &config.VaultTokenAuth{},
	})

	assert.Nil(t, authenticator)
	assert.EqualError(t, err, `authenticator factory for "*config.VaultTokenAuth" authentication method is unknown`)
}

func TestGetAuthenticator_ValidConfiguration(t *testing.T) {
	authenticator, err := getAuthenticator(config.VaultAuth{
		Token: &config.VaultTokenAuth{},
	})

	assert.IsType(t, &auth.Token{}, authenticator)
	assert.NoError(t, err)
}
