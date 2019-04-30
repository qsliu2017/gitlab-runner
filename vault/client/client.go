package client

import (
	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

type VaultServerReadyResp struct {
	State bool
	Err   error
}

type Client interface {
	IsServerReady() VaultServerReadyResp
	SetToken(token string)
}

func New(server config.VaultServer) (Client, error) {
	vaultCliConfig := api.DefaultConfig()
	vaultCliConfig.Address = server.URL

	tlsConfig := &api.TLSConfig{
		CACert: server.TLSCAFile,
	}

	err := vaultCliConfig.ConfigureTLS(tlsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't prepare TLS configuration for the new Vault client")
	}

	cli, err := api.NewClient(vaultCliConfig)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create new Vault client")
	}

	return &client{c: cli}, nil
}

type client struct {
	c *api.Client
}

func (c *client) IsServerReady() VaultServerReadyResp {
	resp, err := c.c.Sys().Health()
	if err != nil {
		return VaultServerReadyResp{State: false, Err: err}
	}

	return VaultServerReadyResp{State: resp.Initialized && !resp.Sealed, Err: nil}
}

func (c *client) SetToken(token string) {
	c.c.SetToken(token)
}
