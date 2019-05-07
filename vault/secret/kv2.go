package secret

import (
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

type KV2 struct{}

func (s *KV2) Read(cli client.Client, builder Builder, path string, secretSpec *config.VaultSecret) error {
	data, err := cli.Read(path)
	if err != nil {
		return errors.Wrapf(err, "couldn't read KV2 secret for %q", path)
	}

	innerData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.Errorf("no a valid KV2 secret format for %q", path)
	}

	for _, key := range secretSpec.Keys {
		keyData, ok := innerData[key.Key]
		if !ok {
			return errors.Errorf("no data for key %q for KV2 secret %q", key.Key, path)
		}

		err = builder.BuildSecret(key, keyData)
		if err != nil {
			return errors.Wrapf(err, "couldn't build secret for %q::%q", path, key.Key)
		}
	}

	return nil
}
