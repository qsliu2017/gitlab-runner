package config

import (
	"fmt"
	"strings"
)

const (
	VaultSecretTypeKV1 VaultSecretType = "kv1"
	VaultSecretTypeKV2 VaultSecretType = "kv2"
)

const (
	DefaultUserpassAuthPath = "userpass"
	DefaultTLSAuthPath      = "cert"
)

type Vault struct {
	Server  VaultServer  `toml:"server" json:"server" group:"vault server configuration" namespace:"server"`
	Auth    VaultAuth    `toml:"auth" json:"auth" group:"vault auth configuration" namespace:"auth"`
	Secrets VaultSecrets `toml:"secrets" json:"secrets" group:"vault secrets configuration" namespace:"secrets"`
}

type VaultServer struct {
	URL       string `toml:"url" json:"url" long:"url" env:"RUNNER_VAULT_URL" required:"true" description:"URL of the Vault server to use"`
	TLSCAFile string `toml:"tls_ca_file,omitempty" json:"tls_ca_file" long:"tls-ca-file" env:"RUNNER_VAULT_TLS_CA_FILE" description:"CA file to use when communicating with Vault server"`
}

type VaultAuth struct {
	Token    *VaultTokenAuth    `toml:"token,omitempty" json:"token" group:"vault token auth configuration" namespace:"token"`
	Userpass *VaultUserpassAuth `toml:"userpass,omitempty" json:"userpass" group:"vault userpass auth configuration" namespace:"userpass"`
	TLS      *VaultTLSAuth      `toml:"tls,omitempty" json:"tls" group:"vault tls auth configuration" namespace:"tls"`
}

type VaultTokenAuth struct {
	Token string `toml:"token" json:"token" long:"token" env:"RUNNER_VAULT_AUTH_TOKEN_TOKEN" required:"true" description:"Token to authenticate against Vault server"`
}

type VaultUserpassAuth struct {
	Path     string `toml:"path,omitempty" json:"path" long:"path" env:"RUNNER_VAULT_AUTH_USERPASS_PATH" required:"false" description:"Path on which userpass login method is enabled"`
	Username string `toml:"username" json:"username" long:"username" env:"RUNNER_VAULT_AUTH_USERPASS_USERNAME" required:"true" description:"Username to authenticate against Vault server"`
	Password string `toml:"password" json:"password" long:"password" env:"RUNNER_VAULT_AUTH_USERPASS_PASSWORD" required:"true" description:"Password to authenticate against Vault server"`
}

func (a *VaultUserpassAuth) GetPath() string {
	if a.Path != "" {
		return a.Path
	}

	return DefaultUserpassAuthPath
}

type VaultTLSAuth struct {
	Path        string `toml:"path,omitempty" json:"path" long:"path" env:"RUNNER_VAULT_AUTH_TLS_PATH" required:"false" description:"Path on which TLS login method is enabled"`
	Name        string `toml:"name" json:"name" long:"name" env:"RUNNER_VAULT_AUTH_TLS_NAME" required:"false" description:"Name of the role that should be used for authentication. If not given any role matching the provided certificate will be used"`
	TLSCertFile string `toml:"tls_cert_file" json:"tls_cert_file" long:"tls-cert-file" env:"RUNNER_VAULT_AUTH_TLS_TLS_CERT_FILE" required:"true" description:"TLS client certificate to authenticate against Vault server"`
	TLSKeyFile  string `toml:"tls_key_file" json:"tls_key_file" long:"tls-key-file" env:"RUNNER_VAULT_AUTH_TLS_TLS_KEY_FILE" required:"true" description:"TLS client private key to authenticate against Vault server"`
}

func (a *VaultTLSAuth) GetPath() string {
	if a.Path != "" {
		return a.Path
	}

	return DefaultTLSAuthPath
}

type VaultSecrets []*VaultSecret

type VaultSecret struct {
	Type VaultSecretType `toml:"type" json:"type" long:"type" required:"true" description:"Type of the secret"`
	Path string          `toml:"path" json:"path" long:"path" required:"true" description:"Path of the secret to request from Vault Server"`
	Keys VaultSecretKeys `toml:"keys,omitempty" json:"keys" group:"vault secret keys configuration" namespace:"keys"`
}

type VaultSecretType string

type VaultSecretKeys []*VaultSecretKey

func (k VaultSecretKeys) String() string {
	var out []string

	for _, s := range k {
		out = append(out, fmt.Sprintf("%s=%s", s.Key, s.EnvName))
	}

	return fmt.Sprintf("[%s]", strings.Join(out, " "))
}

type VaultSecretKey struct {
	Key     string `toml:"key" json:"key" long:"key" required:"true" description:"Secret's key"`
	EnvName string `toml:"env_name" json:"env_name" long:"env_name" description:"Environment variable on which the secret value should be set"`
}
