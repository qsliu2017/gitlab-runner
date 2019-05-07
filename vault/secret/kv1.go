package secret

import (
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

type KV1 struct{}

func (s *KV1) Read(cli client.Client, builder Builder, path string, secretSpec *config.VaultSecret) error {
	data, err := cli.Read(path)
	if err != nil {
		return errors.Wrapf(err, "couldn't read KV1 secret for %q", path)
	}

	for _, key := range secretSpec.Keys {
		keyData, ok := data[key.Key]
		if !ok {
			return errors.Errorf("no data for key %q for KV1 secret %q", key.Key, path)
		}

		err = builder.BuildSecret(key, keyData)
		if err != nil {
			return errors.Wrapf(err, "couldn't build secret for %q::%q", path, key.Key)
		}
	}

	return nil
}
