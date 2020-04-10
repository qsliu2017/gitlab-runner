package vault

import (
	"errors"
	"fmt"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
)

type Client interface {
	Authenticate(auth AuthMethod) error
	Write(path string, data map[string]interface{}) (Result, error)
	Read(path string) (Result, error)
	Delete(path string) error
}

type defaultClient struct {
	internal *api.Client
	logger   logrus.FieldLogger
}

var ErrVaultServerNotReady = errors.New("not initialized or sealed Vault server")

func NewClient(URL string, logger logrus.FieldLogger) (Client, error) {
	config := &api.Config{
		Address: URL,
	}

	logger.Debug("Creating new Vault API client")

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("creating new Vault client: %w", unwrapAPIResponseError(err))
	}

	logger.Debug("Checking Vault server status")

	healthResp, err := client.Sys().Health()
	if err != nil {
		return nil, fmt.Errorf("checking Vault server health: %w", unwrapAPIResponseError(err))
	}

	if !healthResp.Initialized || healthResp.Sealed {
		return nil, ErrVaultServerNotReady
	}

	logger.Debug("Vault server connected and waits for requests")

	c := &defaultClient{
		internal: client,
		logger:   logger,
	}

	return c, nil
}

func (c *defaultClient) Authenticate(auth AuthMethod) error {
	logger := c.logger.WithField("auth", auth.Name())
	logger.Debug("Authenticating in Vault")

	err := auth.Authenticate(c)
	if err != nil {
		return fmt.Errorf("authenticating Vault client: %w", err)
	}

	c.internal.SetToken(auth.Token())

	logger.Debug("Authentication finished")

	return nil
}

func (c *defaultClient) Write(path string, data map[string]interface{}) (Result, error) {
	logger := c.logger.WithField("path", path)
	logger.Debug("Writing to Vault's path")

	secret, err := c.internal.Logical().Write(TrimSlashes(path), data)

	logger.WithError(err).Debug("Writing finished")

	return newResult(secret), unwrapAPIResponseError(err)
}

func (c *defaultClient) Read(path string) (Result, error) {
	logger := c.logger.WithField("path", path)
	logger.Debug("Reading from Vault's path")

	secret, err := c.internal.Logical().Read(path)

	logger.WithError(err).Debug("Read finished")

	return newResult(secret), unwrapAPIResponseError(err)
}

func (c *defaultClient) Delete(path string) error {
	logger := c.logger.WithField("path", path)
	logger.Debug("Deleting Vault path")

	_, err := c.internal.Logical().Delete(path)

	logger.WithError(err).Debug("Path deleted")

	return unwrapAPIResponseError(err)
}
