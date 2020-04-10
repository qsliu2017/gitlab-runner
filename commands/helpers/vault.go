package helpers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/service"
)

type VaultSecretCommand struct {
	service.Secret

	Action     string `long:"action" description:"Action to perform on the secret (get, put, delete)"`
	StorageDir string `long:"storage-dir" description:"Directory where the secret files are stored"`
	ServerName string `long:"server-name" description:"Name of the server used for performing Vault operation"`
}

func (c *VaultSecretCommand) Execute(_ *cli.Context) {
	logger := logrus.StandardLogger().
		WithField("serverName", c.ServerName)

	actions := map[string]func() error{
		"get":    c.getSecret,
		"put":    c.putSecret,
		"delete": c.deleteSecret,
	}

	action, ok := actions[c.Action]
	if !ok {
		logger.Fatalf("Undefined Vault operation %q, expected one of: get, put, delete", c.Action)
	}

	err := c.Initialize(logger)
	if err != nil {
		logger.WithError(err).
			Fatal("Couldn't initialize vault integration")
	}

	err = action()
	if err != nil {
		logger.WithError(err).
			Fatal("Error while execution Vault operation")
	}
}

func (c *VaultSecretCommand) getSecret() error {
	secret, err := c.Secret.Get()
	if err != nil {
		return err
	}

	if secret == nil {
		c.Logger().Warning("Getting secret... nothing")
		return nil
	}

	for key, value := range secret {
		filePath := filepath.Join(c.StorageDir, key)
		err := createSecretFile(filePath, value)
		if err != nil {
			return fmt.Errorf("creating secret file for key %q: %w", key, err)
		}
	}

	c.Logger().Print("Getting secret from Vault... ok")

	return nil
}

func createSecretFile(filePath string, value interface{}) error {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating file %q: %w", filePath, err)
	}
	defer file.Close()

	_, err = fmt.Fprint(file, value)
	if err != nil {
		return fmt.Errorf("writing to file %q: %w", filePath, err)
	}

	return nil
}

func (c *VaultSecretCommand) putSecret() error {
	secret, err := readSecretFiles(c.StorageDir)
	if err != nil {
		return err
	}

	err = c.Secret.Put(secret)
	if err != nil {
		return err
	}

	c.Logger().Print("Saving secret to Vault... ok")

	return nil
}

func readSecretFiles(path string) (map[string]interface{}, error) {
	secrets := make(map[string]interface{})

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading secret file %q: %w", info.Name(), err)
			}

			secrets[info.Name()] = string(content)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("scanning secrets directory: %w", err)
	}

	return secrets, nil
}

func (c *VaultSecretCommand) deleteSecret() error {
	err := c.Secret.Delete()
	if err != nil {
		return fmt.Errorf("deleting secret: %w", err)
	}

	c.Logger().Print("Deleting secret from Vault... ok")

	return nil
}

func init() {
	common.RegisterCommand2("vault-secret", "manage HashiCorp Vault secrets", new(VaultSecretCommand))
}
