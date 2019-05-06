package vault

import (
	"reflect"

	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/gitlab-runner/vault/auth"
	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

type AuthenticatorFactory func() Authenticator

type Authenticator interface {
	Authenticate(cli client.Client, authConfig config.VaultAuth) (client.TokenInfo, error)
}

var authenticatorFactories = map[reflect.Type]AuthenticatorFactory{
	reflect.TypeOf(&config.VaultTokenAuth{}):    func() Authenticator { return auth.NewToken() },
	reflect.TypeOf(&config.VaultUserpassAuth{}): func() Authenticator { return auth.NewUserpass() },
	reflect.TypeOf(&config.VaultTLSAuth{}):      func() Authenticator { return auth.NewTLS() },
}

func getAuthenticator(auth config.VaultAuth) (Authenticator, error) {
	authMethod, err := firstDefined(auth)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't detect configured authentication method")
	}

	authenticatorFactory, ok := authenticatorFactories[authMethod]
	if !ok {
		return nil, errors.Errorf("authenticator factory for %q authentication method is unknown", authMethod)
	}

	return authenticatorFactory(), nil
}

func firstDefined(auth config.VaultAuth) (reflect.Type, error) {
	authReflectVal := reflect.ValueOf(auth)
	fieldsNum := authReflectVal.Type().NumField()

	var authVal reflect.Value

	for i := 0; i < fieldsNum; i++ {
		authVal = authReflectVal.Field(i)
		if !authVal.IsNil() {
			return authVal.Type(), nil
		}
	}

	return nil, errors.New("missing authentication method configuration")
}
