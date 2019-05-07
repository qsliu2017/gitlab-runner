package client

import (
	"fmt"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

type VaultServerReadyResp struct {
	State bool
	Err   error
}

type TokenInfo struct {
	Token string
	TTL   time.Duration
}

type Client interface {
	IsServerReady() VaultServerReadyResp
	SetToken(token string)
	TokenLookupSelf() (TokenInfo, error)
	UserpassLogin(path string, username string, password string) (TokenInfo, error)
	TLSLogin(path string, name string, tlsCertFile string, tlsKeyFile string) (TokenInfo, error)
	Read(path string) (map[string]interface{}, error)
}

func New(server config.VaultServer) (Client, error) {
	vaultCliConfig := prepareVaultCliConfig(server)

	tlsConfig := &api.TLSConfig{
		CACert: server.TLSCAFile,
	}

	cli, err := newClient(vaultCliConfig, tlsConfig)
	if err != nil {
		return nil, err
	}

	return &client{c: cli, vaultServer: server}, nil
}

func prepareVaultCliConfig(server config.VaultServer) *api.Config {
	vaultCliConfig := api.DefaultConfig()
	vaultCliConfig.Address = server.URL

	return vaultCliConfig
}

func newClient(vaultCliConfig *api.Config, tlsConfig *api.TLSConfig) (*api.Client, error) {
	err := vaultCliConfig.ConfigureTLS(tlsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't prepare TLS configuration for the new Vault client")
	}

	cli, err := api.NewClient(vaultCliConfig)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create new Vault client")
	}

	return cli, nil
}

type client struct {
	c *api.Client

	vaultServer config.VaultServer
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

func (c *client) TokenLookupSelf() (TokenInfo, error) {
	secret, err := c.c.Auth().Token().LookupSelf()
	if err != nil {
		return TokenInfo{}, errors.Wrap(err, "error while executing self-lookup API")
	}

	ttl, err := secret.TokenTTL()
	if err != nil {
		return TokenInfo{}, errors.Wrap(err, "couldn't retrieve token's TTL")
	}

	info := TokenInfo{
		Token: c.c.Token(),
		TTL:   ttl,
	}

	return info, nil
}

func (c *client) UserpassLogin(path string, username string, password string) (TokenInfo, error) {
	apiPath := fmt.Sprintf("auth/%s/login/%s", path, username)
	data := map[string]interface{}{
		"password": password,
	}

	secret, err := c.c.Logical().Write(apiPath, data)
	if err != nil {
		return TokenInfo{}, errors.Wrap(err, "error while executing userpass login")
	}

	token, err := secret.TokenID()
	if err != nil {
		return TokenInfo{}, errors.Wrap(err, "couldn't retrieve token from userpass login response")
	}

	ttl, err := secret.TokenTTL()
	if err != nil {
		return TokenInfo{}, errors.Wrap(err, "couldn't retrieve token's TTL")
	}

	info := TokenInfo{
		Token: token,
		TTL:   ttl,
	}

	return info, nil
}

func (c *client) TLSLogin(path string, name string, tlsCertFile string, tlsKeyFile string) (TokenInfo, error) {
	vaultCliConfig := prepareVaultCliConfig(c.vaultServer)

	tlsConfig := &api.TLSConfig{
		CACert:     c.vaultServer.TLSCAFile,
		ClientCert: tlsCertFile,
		ClientKey:  tlsKeyFile,
	}

	client, err := newClient(vaultCliConfig, tlsConfig)
	if err != nil {
		return TokenInfo{}, errors.Wrap(err, "couldn't re-create the Vault client with TLS Client Authentication config")
	}

	apiPath := fmt.Sprintf("auth/%s/login", path)
	data := map[string]interface{}{}

	if name != "" {
		data["name"] = name
	}

	secret, err := client.Logical().Write(apiPath, data)
	if err != nil {
		return TokenInfo{}, errors.Wrap(err, "error while executing TLS login")
	}

	token, err := secret.TokenID()
	if err != nil {
		return TokenInfo{}, errors.Wrap(err, "couldn't retrieve token from TLS login response")
	}

	ttl, err := secret.TokenTTL()
	if err != nil {
		return TokenInfo{}, errors.Wrap(err, "couldn't retrieve token's TTL")
	}

	info := TokenInfo{
		Token: token,
		TTL:   ttl,
	}

	return info, nil
}

func (c *client) Read(path string) (map[string]interface{}, error) {
	secret, err := c.c.Logical().Read(path)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't read data for %q", path)
	}

	return secret.Data, nil
}
