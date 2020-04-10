package kv2

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines"
)

const engineName = "kv2"

type engine struct {
	client vault.Client
	path   string
}

func NewEngine(client vault.Client, path string) vault.SecretEngine {
	return &engine{
		client: client,
		path:   vault.TrimSlashes(path),
	}
}

func (e *engine) EngineName() string {
	return engineName
}

func (e *engine) Get(path string) (map[string]interface{}, error) {
	secret, err := e.client.Read(e.dataPath(path))
	if err != nil {
		return nil, fmt.Errorf("reading from Vault: %w", err)
	}

	if secret == nil {
		return nil, nil
	}

	data := secret.Data()
	if data == nil {
		return nil, nil
	}

	_, ok := data["data"]
	if !ok {
		return nil, nil
	}

	return data["data"].(map[string]interface{}), nil
}

func (e *engine) dataPath(path string) string {
	return fmt.Sprintf("%s/data/%s", e.path, vault.TrimSlashes(path))
}

func (e *engine) Put(path string, data map[string]interface{}) error {
	dataWrapper := map[string]interface{}{
		"data": data,
	}

	_, err := e.client.Write(e.dataPath(path), dataWrapper)
	if err != nil {
		return fmt.Errorf("writing to Vault: %w", err)
	}

	return nil
}

func (e *engine) Delete(path string) error {
	err := e.client.Delete(e.metadataPath(path))
	if err != nil {
		return fmt.Errorf("deleting from Vault: %w", err)
	}

	return nil
}

func (e *engine) metadataPath(path string) string {
	return fmt.Sprintf("%s/metadata/%s", e.path, vault.TrimSlashes(path))
}

func init() {
	secret_engines.MustRegisterFactory(engineName, NewEngine)
}
