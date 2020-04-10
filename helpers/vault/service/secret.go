package service

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/auth_methods"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/vault/auth_methods/jwt" // register auth method
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines/kv1" // register secret engine
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines/kv2" // register secret engine
)

type Secret struct {
	URL string `long:"url" description:"HashiCorp Vault server URL"`

	AuthName string   `long:"auth-name" description:"Name of the auth method to use"`
	AuthPath string   `long:"auth-path" description:"Path for the registered auth method to use"`
	AuthData []string `long:"auth-data" description:"Array of auth method configuration details (in form of 'key=value')"`

	SecretEngineName string `long:"secret-engine-name" description:"Vault's secret engine to use"`
	SecretEnginePath string `long:"secret-engine-path" description:"Path for the registered secret engine"`

	SecretPath string `long:"secret-path" description:"Path for the used secret"`
	SecretName string `long:"secret-name" description:"Name describing the secret (the name of variable representing the storage directory)"`

	logger logrus.FieldLogger
	client vault.Client
	engine vault.SecretEngine
}

func (v *Secret) Logger() logrus.FieldLogger {
	return v.logger
}

func (v *Secret) Initialize(logger logrus.FieldLogger) error {
	v.logger = logger.WithField("secret", v.SecretName)

	err := v.prepareAuthenticatedClient()
	if err != nil {
		return fmt.Errorf("preparing authenticated client: %w", err)
	}

	err = v.prepareSecretEngine()
	if err != nil {
		return fmt.Errorf("preparing secrets engine: %w", err)
	}

	return nil
}

func (v *Secret) prepareAuthenticatedClient() error {
	client, err := vault.NewClient(v.URL, v.logger)
	if err != nil {
		return err
	}

	auth, err := v.prepareAuthMethodAdapter()
	if err != nil {
		return err
	}

	err = client.Authenticate(auth)
	if err != nil {
		return err
	}

	v.client = client

	return nil
}

func (v *Secret) prepareAuthMethodAdapter() (vault.AuthMethod, error) {
	authFactory, err := auth_methods.GetFactory(v.AuthName)
	if err != nil {
		return nil, fmt.Errorf("initializing auth method factory: %w", err)
	}

	data := make(map[string]interface{})
	for _, element := range v.AuthData {
		parts := strings.Split(element, "=")
		data[parts[0]] = parts[1]
	}

	auth, err := authFactory(v.AuthPath, data)
	if err != nil {
		return nil, fmt.Errorf("initializing auth method adapter: %w", err)
	}

	return auth, nil
}

func (v *Secret) prepareSecretEngine() error {
	engineFactory, err := secret_engines.GetFactory(v.SecretEngineName)
	if err != nil {
		return fmt.Errorf("requesting SecretEngine factory: %w", err)
	}

	v.engine = engineFactory(v.client, v.SecretEnginePath)

	return nil
}

func (v *Secret) Get() (map[string]interface{}, error) {
	secret, err := v.engine.Get(v.SecretPath)
	if err != nil {
		return nil, fmt.Errorf("reading secret: %w", err)
	}

	return secret, nil
}

func (v *Secret) Put(data map[string]interface{}) error {
	err := v.engine.Put(v.SecretPath, data)
	if err != nil {
		return fmt.Errorf("writing secret: %w", err)
	}

	return nil
}

func (v *Secret) Delete() error {
	err := v.engine.Delete(v.SecretPath)
	if err != nil {
		return fmt.Errorf("deleting secret: %w", err)
	}

	return nil
}
